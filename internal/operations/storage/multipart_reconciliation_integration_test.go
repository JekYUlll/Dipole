package storageops

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"
)

func TestMultipartReconciliationWithRealMinIOAndRedis(t *testing.T) {
	endpoint := strings.TrimSpace(os.Getenv("DIPOLE_TEST_MULTIPART_RECONCILIATION_MINIO_ENDPOINT"))
	redisAddress := strings.TrimSpace(os.Getenv("DIPOLE_TEST_MULTIPART_RECONCILIATION_REDIS_ADDR"))
	accessKey := os.Getenv("DIPOLE_TEST_MULTIPART_RECONCILIATION_MINIO_ACCESS_KEY")
	secretKey := os.Getenv("DIPOLE_TEST_MULTIPART_RECONCILIATION_MINIO_SECRET_KEY")
	if endpoint == "" || redisAddress == "" || accessKey == "" || secretKey == "" {
		t.Skip("MinIO endpoint/credentials and Redis address are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	minioClient, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(accessKey, secretKey, ""), Secure: false})
	if err != nil {
		t.Fatalf("create MinIO client: %v", err)
	}
	redisClient := redis.NewClient(&redis.Options{Addr: redisAddress})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		redisClient.Close()
		t.Fatalf("ping Redis: %v", err)
	}
	t.Cleanup(func() { _ = redisClient.Close() })

	suffix := make([]byte, 6)
	if _, err := rand.Read(suffix); err != nil {
		t.Fatalf("generate bucket suffix: %v", err)
	}
	bucket := "dipole-reconcile-" + hex.EncodeToString(suffix)
	objectKey := "message-files/reconcile.bin"
	if err := minioClient.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
		t.Fatalf("create bucket: %v", err)
	}
	core := minio.Core{Client: minioClient}
	uploadID, err := core.NewMultipartUpload(ctx, bucket, objectKey, minio.PutObjectOptions{ContentType: "application/octet-stream"})
	if err != nil {
		_ = minioClient.RemoveBucket(ctx, bucket)
		t.Fatalf("create multipart upload: %v", err)
	}
	if _, err := core.PutObjectPart(ctx, bucket, objectKey, uploadID, 1, bytes.NewReader([]byte("reconcile-part")), int64(len("reconcile-part")), minio.PutObjectPartOptions{}); err != nil {
		t.Fatalf("write multipart part: %v", err)
	}
	sessionID := "reconcile-" + hex.EncodeToString(suffix)
	redisKey := "file:multipart:" + sessionID + ":meta"
	t.Cleanup(func() {
		_ = core.AbortMultipartUpload(context.Background(), bucket, objectKey, uploadID)
		_ = minioClient.RemoveBucket(context.Background(), bucket)
		_ = redisClient.Del(context.Background(), redisKey, "file:multipart:orphan:meta").Err()
	})

	payload, err := json.Marshal(map[string]string{"object_key": objectKey, "upload_id": uploadID})
	if err != nil {
		t.Fatalf("marshal session: %v", err)
	}
	if err := redisClient.Set(ctx, redisKey, payload, 10*time.Minute).Err(); err != nil {
		t.Fatalf("write matching session: %v", err)
	}
	reconciliationClient := realMultipartClient{client: minioClient, core: core}
	if err := waitForMultipartListing(ctx, minioClient, bucket, objectKey, uploadID); err != nil {
		t.Fatal(err)
	}

	matched := RunMultipartReconciliation(ctx, reconciliationClient, redisClient, bucket, objectKey, 100)
	if !matched.Complete || matched.RedisKeysScanned != 1 || matched.MinIOUploadsSeen != 1 || matched.MissingRedis != 0 || matched.MissingMinIO != 0 {
		t.Fatalf("matching stores reported drift: %+v", matched)
	}
	if err := waitForRedisRestartWindow(ctx); err != nil {
		t.Fatal(err)
	}

	if err := redisClient.Expire(ctx, redisKey, time.Second).Err(); err != nil {
		t.Fatalf("expire matching session: %v", err)
	}
	if err := waitForRedisKeyAbsence(ctx, redisClient, redisKey); err != nil {
		t.Fatal(err)
	}
	expired := RunMultipartReconciliation(ctx, reconciliationClient, redisClient, bucket, objectKey, 100)
	if !expired.Complete || expired.MissingRedis != 1 || expired.MissingMinIO != 0 {
		t.Fatalf("expired Redis metadata was not detected: %+v", expired)
	}

	cleanupReport := RunMultipartCleanup(ctx, reconciliationClient, bucket, objectKey, time.Now().UTC().Add(time.Hour), true)
	if !cleanupReport.Complete || cleanupReport.Selected != 1 || cleanupReport.Aborted != 1 || cleanupReport.Failed != 0 {
		t.Fatalf("expired session cleanup failed: %+v", cleanupReport)
	}
	if err := waitForMultipartListingAbsence(ctx, minioClient, bucket, objectKey, uploadID); err != nil {
		t.Fatal(err)
	}

	orphanKey := "file:multipart:orphan:meta"
	orphanPayload := []byte(fmt.Sprintf(`{"object_key":"%s","upload_id":"missing-upload"}`, objectKey))
	if err := redisClient.Set(ctx, orphanKey, orphanPayload, 10*time.Minute).Err(); err != nil {
		t.Fatalf("write orphan session: %v", err)
	}
	orphan := RunMultipartReconciliation(ctx, reconciliationClient, redisClient, bucket, objectKey, 100)
	if !orphan.Complete || orphan.MissingRedis != 0 || orphan.MissingMinIO != 1 {
		t.Fatalf("cross-store drift was not fully detected: %+v", orphan)
	}
}

type realMultipartClient struct {
	client *minio.Client
	core   minio.Core
}

func (c realMultipartClient) ListIncompleteUploads(ctx context.Context, bucket, prefix string, recursive bool) <-chan minio.ObjectMultipartInfo {
	return c.client.ListIncompleteUploads(ctx, bucket, prefix, recursive)
}

func (c realMultipartClient) AbortMultipartUpload(ctx context.Context, bucket, objectKey, uploadID string) error {
	return c.core.AbortMultipartUpload(ctx, bucket, objectKey, uploadID)
}

func waitForMultipartListing(ctx context.Context, client *minio.Client, bucket, prefix, uploadID string) error {
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		for upload := range client.ListIncompleteUploads(ctx, bucket, prefix, true) {
			if upload.Err != nil {
				return fmt.Errorf("list multipart uploads while waiting for %s: %w", uploadID, upload.Err)
			}
			if upload.UploadID == uploadID {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for multipart upload %s: %w", uploadID, ctx.Err())
		case <-deadline.C:
			return fmt.Errorf("multipart upload %s did not appear in listing", uploadID)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func waitForMultipartListingAbsence(ctx context.Context, client *minio.Client, bucket, prefix, uploadID string) error {
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		found := false
		for upload := range client.ListIncompleteUploads(ctx, bucket, prefix, true) {
			if upload.Err != nil {
				return fmt.Errorf("list multipart uploads while waiting for cleanup of %s: %w", uploadID, upload.Err)
			}
			if upload.UploadID == uploadID {
				found = true
			}
		}
		if !found {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for multipart upload cleanup %s: %w", uploadID, ctx.Err())
		case <-deadline.C:
			return fmt.Errorf("multipart upload %s remained in listing after cleanup", uploadID)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func waitForRedisKeyAbsence(ctx context.Context, client *redis.Client, key string) error {
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		present, err := client.Exists(ctx, key).Result()
		if err != nil {
			return fmt.Errorf("check Redis key %s: %w", key, err)
		}
		if present == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for Redis key %s expiry: %w", key, ctx.Err())
		case <-deadline.C:
			return fmt.Errorf("Redis key %s remained after TTL", key)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func waitForRedisRestartWindow(ctx context.Context) error {
	readyFile := strings.TrimSpace(os.Getenv("DIPOLE_TEST_MULTIPART_RECONCILIATION_REDIS_RESTART_READY_FILE"))
	resumeFile := strings.TrimSpace(os.Getenv("DIPOLE_TEST_MULTIPART_RECONCILIATION_REDIS_RESTART_RESUME_FILE"))
	if readyFile == "" && resumeFile == "" {
		return nil
	}
	if readyFile == "" || resumeFile == "" {
		return fmt.Errorf("both Redis restart marker files are required")
	}
	if err := os.WriteFile(readyFile, []byte("ready\n"), 0o600); err != nil {
		return fmt.Errorf("write Redis restart ready marker: %w", err)
	}
	deadline := time.NewTimer(15 * time.Second)
	defer deadline.Stop()
	for {
		if _, err := os.Stat(resumeFile); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("check Redis restart resume marker: %w", err)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for Redis restart resume: %w", ctx.Err())
		case <-deadline.C:
			return fmt.Errorf("Redis restart resume marker did not appear")
		case <-time.After(100 * time.Millisecond):
		}
	}
}
