package storageops

import (
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

	matched := RunMultipartReconciliation(ctx, reconciliationClient, redisClient, bucket, "message-files/", 100)
	if !matched.Complete || matched.RedisKeysScanned != 1 || matched.MinIOUploadsSeen != 1 || matched.MissingRedis != 0 || matched.MissingMinIO != 0 {
		t.Fatalf("matching stores reported drift: %+v", matched)
	}

	if err := redisClient.Del(ctx, redisKey).Err(); err != nil {
		t.Fatalf("delete matching session: %v", err)
	}
	missingRedis := RunMultipartReconciliation(ctx, reconciliationClient, redisClient, bucket, "message-files/", 100)
	if !missingRedis.Complete || missingRedis.MissingRedis != 1 || missingRedis.MissingMinIO != 0 {
		t.Fatalf("missing Redis metadata was not detected: %+v", missingRedis)
	}

	orphanKey := "file:multipart:orphan:meta"
	orphanPayload := []byte(fmt.Sprintf(`{"object_key":"%s","upload_id":"missing-upload"}`, objectKey))
	if err := redisClient.Set(ctx, orphanKey, orphanPayload, 10*time.Minute).Err(); err != nil {
		t.Fatalf("write orphan session: %v", err)
	}
	orphan := RunMultipartReconciliation(ctx, reconciliationClient, redisClient, bucket, "message-files/", 100)
	if !orphan.Complete || orphan.MissingRedis != 1 || orphan.MissingMinIO != 1 {
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
