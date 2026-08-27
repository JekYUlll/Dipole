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
	uploader := &MinIOUploader{client: client, bucket: bucket}
	store, err := NewAgentArtifactBlobStore(uploader)
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
