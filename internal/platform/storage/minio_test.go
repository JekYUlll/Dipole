package storage

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func TestMinIOUploaderPresignMultipartPartURLBindsSessionAndPart(t *testing.T) {
	client, err := minio.New("127.0.0.1:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("access", "secret", ""),
		Region: "us-east-1",
	})
	if err != nil {
		t.Fatalf("create minio client: %v", err)
	}
	uploader := &MinIOUploader{client: client, bucket: "dipole-files"}

	value, err := uploader.PresignMultipartPartURL(context.Background(), "message-files/f.bin", "upload-123", 7, 10*time.Minute)
	if err != nil {
		t.Fatalf("presign part: %v", err)
	}
	parsed, err := url.Parse(value)
	if err != nil {
		t.Fatalf("parse presigned URL: %v", err)
	}
	if parsed.Query().Get("uploadId") != "upload-123" || parsed.Query().Get("partNumber") != "7" {
		t.Fatalf("presigned URL lost multipart binding: %s", parsed.RawQuery)
	}
}
