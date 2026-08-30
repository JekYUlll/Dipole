package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func TestMinIOMultipartUploadLifecycle(t *testing.T) {
	endpoint := strings.TrimSpace(os.Getenv("DIPOLE_TEST_MINIO_ENDPOINT"))
	if endpoint == "" {
		t.Skip("DIPOLE_TEST_MINIO_ENDPOINT is required")
	}
	accessKey := os.Getenv("DIPOLE_TEST_MINIO_ACCESS_KEY")
	secretKey := os.Getenv("DIPOLE_TEST_MINIO_SECRET_KEY")
	if accessKey == "" || secretKey == "" {
		t.Skip("DIPOLE_TEST_MINIO_ACCESS_KEY and DIPOLE_TEST_MINIO_SECRET_KEY are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(accessKey, secretKey, "")})
	if err != nil {
		t.Fatalf("create MinIO client: %v", err)
	}
	randomID := make([]byte, 8)
	if _, err := rand.Read(randomID); err != nil {
		t.Fatalf("generate test bucket suffix: %v", err)
	}
	bucket := "dipole-multipart-" + hex.EncodeToString(randomID)
	if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
		t.Fatalf("create test bucket: %v", err)
	}
	t.Cleanup(func() {
		_ = client.RemoveObject(context.Background(), bucket, "message-files/multipart-integration.bin", minio.RemoveObjectOptions{})
		_ = client.RemoveObject(context.Background(), bucket, "message-files/multipart-abort-integration.bin", minio.RemoveObjectOptions{})
		_ = client.RemoveObject(context.Background(), bucket, "message-files/multipart-interrupted-integration.bin", minio.RemoveObjectOptions{})
		_ = client.RemoveBucket(context.Background(), bucket)
	})

	uploader := &MinIOUploader{client: client, core: minio.Core{Client: client}, bucket: bucket}
	first := bytes.Repeat([]byte("r"), 5*1024*1024)
	second := []byte("second part")
	upload, err := uploader.InitiateMessageMultipartUpload(ctx, "multipart-integration.bin", "application/octet-stream")
	if err != nil {
		t.Fatalf("initiate multipart upload: %v", err)
	}

	// Upload part two first, then replace part one before ordered completion.
	partTwo, err := uploader.UploadMultipartPart(ctx, upload.ObjectKey, upload.UploadID, 2, bytes.NewReader(second), int64(len(second)))
	if err != nil {
		t.Fatalf("upload part two: %v", err)
	}
	stale := bytes.Repeat([]byte("s"), 5*1024*1024)
	if _, err := uploader.UploadMultipartPart(ctx, upload.ObjectKey, upload.UploadID, 1, bytes.NewReader(stale), int64(len(stale))); err != nil {
		t.Fatalf("upload stale part one: %v", err)
	}
	partOne, err := uploader.UploadMultipartPart(ctx, upload.ObjectKey, upload.UploadID, 1, bytes.NewReader(first), int64(len(first)))
	if err != nil {
		t.Fatalf("replace part one: %v", err)
	}
	parts := []MultipartCompletePart{
		{PartNumber: 1, ETag: partOne.ETag, Size: partOne.Size},
		{PartNumber: 2, ETag: partTwo.ETag, Size: partTwo.Size},
	}
	// Complete through the storage adapter's explicit Core client. The public
	// service method also builds a configured application URL, which is outside
	// this isolated S3 lifecycle contract.
	if _, err := uploader.core.CompleteMultipartUpload(ctx, bucket, upload.ObjectKey, upload.UploadID, []minio.CompletePart{
		{PartNumber: parts[0].PartNumber, ETag: parts[0].ETag},
		{PartNumber: parts[1].PartNumber, ETag: parts[1].ETag},
	}, minio.PutObjectOptions{}); err != nil {
		t.Fatalf("complete multipart upload: %v", err)
	}

	reader, err := client.GetObject(ctx, bucket, upload.ObjectKey, minio.GetObjectOptions{})
	if err != nil {
		t.Fatalf("open completed object: %v", err)
	}
	defer reader.Close()
	content, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read completed object: %v", err)
	}
	want := append(append([]byte{}, first...), second...)
	if !bytes.Equal(content, want) {
		t.Fatalf("completed content=%q, want %q", content, want)
	}

	abortUpload, err := uploader.InitiateMessageMultipartUpload(ctx, "multipart-abort-integration.bin", "application/octet-stream")
	if err != nil {
		t.Fatalf("initiate abort upload: %v", err)
	}
	if err := uploader.AbortMultipartUpload(ctx, abortUpload.ObjectKey, abortUpload.UploadID); err != nil {
		t.Fatalf("abort multipart upload: %v", err)
	}
	if err := uploader.AbortMultipartUpload(ctx, abortUpload.ObjectKey, abortUpload.UploadID); err != nil {
		t.Fatalf("repeat abort multipart upload: %v", err)
	}

	// A client connection can fail while MinIO is reading a part. The same
	// part number must remain retryable so the client can resume the session.
	interruptedUpload, err := uploader.InitiateMessageMultipartUpload(ctx, "multipart-interrupted-integration.bin", "application/octet-stream")
	if err != nil {
		t.Fatalf("initiate interrupted upload: %v", err)
	}
	interruptedPart := bytes.Repeat([]byte("i"), 5*1024*1024)
	_, err = uploader.UploadMultipartPart(ctx, interruptedUpload.ObjectKey, interruptedUpload.UploadID, 1,
		&interruptingReader{reader: bytes.NewReader(interruptedPart), failAfter: int64(len(interruptedPart) / 2)}, int64(len(interruptedPart)))
	if !errors.Is(err, errMultipartClientInterrupted) {
		t.Fatalf("interrupted part error=%v, want %v", err, errMultipartClientInterrupted)
	}
	if _, err := uploader.UploadMultipartPart(ctx, interruptedUpload.ObjectKey, interruptedUpload.UploadID, 1,
		bytes.NewReader(interruptedPart), int64(len(interruptedPart))); err != nil {
		t.Fatalf("retry interrupted part: %v", err)
	}
	if err := uploader.AbortMultipartUpload(ctx, interruptedUpload.ObjectKey, interruptedUpload.UploadID); err != nil {
		t.Fatalf("abort resumed interrupted upload: %v", err)
	}
}

var errMultipartClientInterrupted = errors.New("multipart client interrupted")

type interruptingReader struct {
	reader    io.Reader
	failAfter int64
	read      int64
}

func (r *interruptingReader) Read(p []byte) (int, error) {
	if r.read >= r.failAfter {
		return 0, errMultipartClientInterrupted
	}
	remaining := r.failAfter - r.read
	if int64(len(p)) > remaining {
		p = p[:remaining]
	}
	n, err := r.reader.Read(p)
	r.read += int64(n)
	if err != nil {
		return n, err
	}
	return n, nil
}
