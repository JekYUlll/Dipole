package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func TestAgentArtifactMinIOIdentityIsBucketAndOperationBound(t *testing.T) {
	endpoint := os.Getenv("DIPOLE_TEST_MINIO_ENDPOINT")
	if endpoint == "" || os.Getenv("DIPOLE_TEST_ARTIFACT_MINIO_ACCESS_KEY") == "" {
		t.Skip("isolated MinIO identity environment is required")
	}
	artifactAccessKey := os.Getenv("DIPOLE_TEST_ARTIFACT_MINIO_ACCESS_KEY")
	artifactSecretKey := os.Getenv("DIPOLE_TEST_ARTIFACT_MINIO_SECRET_KEY")
	platformAccessKey := os.Getenv("DIPOLE_TEST_PLATFORM_MINIO_ACCESS_KEY")
	platformSecretKey := os.Getenv("DIPOLE_TEST_PLATFORM_MINIO_SECRET_KEY")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	store, err := NewAgentArtifactBlobStoreFromConfig(ctx, AgentArtifactStorageConfigV1{
		Enabled: true, Endpoint: endpoint, AccessKey: artifactAccessKey, SecretKey: artifactSecretKey,
		Bucket: "dipole-agent-artifacts", GeneralAccessKey: platformAccessKey, GeneralBucket: "dipole-files",
	})
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("least privilege evidence")
	artifact, err := application.NewAgentArtifactV1(application.AgentArtifactCreateV1{
		TenantID: "dipole", TaskUUID: "TASK-AUTH", RunUUID: "RUN-AUTH", ArtifactType: "report",
		Version: 1, Title: "Auth", MediaType: "text/plain", Content: body,
	})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := store.PutImmutable(ctx, artifact.ObjectKey, artifact.MediaType, body, artifact.ContentSHA256)
	if err != nil {
		t.Fatal(err)
	}
	artifactClient := mustMinIOTestClient(t, endpoint, artifactAccessKey, artifactSecretKey)
	platformClient := mustMinIOTestClient(t, endpoint, platformAccessKey, platformSecretKey)
	platformKey := "auth-evidence/platform-write"
	if _, err := platformClient.PutObject(ctx, "dipole-files", platformKey, bytes.NewReader(body), int64(len(body)), minio.PutObjectOptions{}); err != nil {
		t.Fatalf("platform identity cannot write its own file bucket: %v", err)
	}
	if err := platformClient.RemoveObject(ctx, "dipole-files", platformKey, minio.RemoveObjectOptions{}); err != nil {
		t.Fatalf("platform identity cannot clean up its own file object: %v", err)
	}
	if _, err := artifactClient.PutObject(ctx, "dipole-files", "forbidden", bytes.NewReader(body), int64(len(body)), minio.PutObjectOptions{}); err == nil {
		t.Fatal("Artifact identity wrote to the platform file bucket")
	}
	if _, err := artifactClient.PutObject(ctx, receipt.Bucket, "outside-prefix", bytes.NewReader(body), int64(len(body)), minio.PutObjectOptions{}); err == nil {
		t.Fatal("Artifact identity wrote outside its object prefix")
	}
	if err := artifactClient.RemoveObject(ctx, receipt.Bucket, receipt.ObjectKey, minio.RemoveObjectOptions{}); err == nil {
		t.Fatal("Artifact runtime identity deleted an immutable object")
	}
	if _, err := platformClient.PutObject(ctx, receipt.Bucket, receipt.ObjectKey, bytes.NewReader(body), int64(len(body)), minio.PutObjectOptions{}); err == nil {
		t.Fatal("platform file identity wrote to the Agent Artifact bucket")
	}
	if reader, err := store.Open(ctx, receipt); err != nil {
		t.Fatalf("immutable object disappeared after denied operations: %v", err)
	} else {
		_ = reader.Close()
	}
}

func mustMinIOTestClient(t *testing.T, endpoint, accessKey, secretKey string) *minio.Client {
	t.Helper()
	client, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(accessKey, secretKey, "")})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestAgentArtifactBlobStoreMinIOContract(t *testing.T) {
	endpoint := os.Getenv("DIPOLE_TEST_MINIO_ENDPOINT")
	if endpoint == "" {
		t.Skip("DIPOLE_TEST_MINIO_ENDPOINT is required")
	}
	accessKey, secretKey := os.Getenv("DIPOLE_TEST_MINIO_ACCESS_KEY"), os.Getenv("DIPOLE_TEST_MINIO_SECRET_KEY")
	client, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(accessKey, secretKey, "")})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	bucket := "dipole-agent-artifact-contract"
	if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.RemoveBucket(context.Background(), bucket) })
	store, err := NewAgentArtifactBlobStoreFromConfig(ctx, AgentArtifactStorageConfigV1{
		Enabled: true, Endpoint: endpoint, AccessKey: accessKey, SecretKey: secretKey, Bucket: bucket,
	})
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("durable report")
	artifact, err := application.NewAgentArtifactV1(application.AgentArtifactCreateV1{TenantID: "dipole", TaskUUID: "TASK-1", RunUUID: "RUN-1", ArtifactType: "report", Version: 1, Title: "Report", MediaType: "text/plain", Content: body})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := store.PutImmutable(ctx, artifact.ObjectKey, artifact.MediaType, body, artifact.ContentSHA256)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = client.RemoveObject(context.Background(), bucket, receipt.ObjectKey, minio.RemoveObjectOptions{})
	})
	if _, err := store.PutImmutable(ctx, artifact.ObjectKey, artifact.MediaType, body, artifact.ContentSHA256); err != nil {
		t.Fatalf("exact MinIO replay: %v", err)
	}
	reader, err := store.Open(ctx, receipt)
	if err != nil {
		t.Fatal(err)
	}
	loaded, readErr := io.ReadAll(reader)
	_ = reader.Close()
	if readErr != nil || !bytes.Equal(loaded, body) {
		t.Fatalf("read body=%q err=%v", loaded, readErr)
	}
	if _, err := client.PutObject(ctx, bucket, receipt.ObjectKey, bytes.NewReader([]byte("drift")), 5, minio.PutObjectOptions{ContentType: "text/plain"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.PutImmutable(ctx, artifact.ObjectKey, artifact.MediaType, body, artifact.ContentSHA256); err == nil {
		t.Fatal("expected MinIO object drift to fail closed")
	}
}
