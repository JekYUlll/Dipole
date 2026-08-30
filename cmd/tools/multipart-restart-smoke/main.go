package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "multipart restart smoke failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("MinIO multipart restart smoke passed: uploaded parts survived service restart and completed object content matched")
}

func run() error {
	endpoint := strings.TrimSpace(os.Getenv("DIPOLE_TEST_MINIO_ENDPOINT"))
	accessKey := strings.TrimSpace(os.Getenv("DIPOLE_TEST_MINIO_ACCESS_KEY"))
	secretKey := strings.TrimSpace(os.Getenv("DIPOLE_TEST_MINIO_SECRET_KEY"))
	readyFile := strings.TrimSpace(os.Getenv("DIPOLE_MULTIPART_RESTART_READY_FILE"))
	resumeFile := strings.TrimSpace(os.Getenv("DIPOLE_MULTIPART_RESTART_RESUME_FILE"))
	if endpoint == "" || accessKey == "" || secretKey == "" || readyFile == "" || resumeFile == "" {
		return fmt.Errorf("DIPOLE_TEST_MINIO_ENDPOINT, credentials, and restart marker files are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	client, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(accessKey, secretKey, "")})
	if err != nil {
		return fmt.Errorf("create MinIO client: %w", err)
	}
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return fmt.Errorf("generate bucket suffix: %w", err)
	}
	bucket := "dipole-restart-" + hex.EncodeToString(suffix)
	objectKey := "message-files/restart.bin"
	if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
		return fmt.Errorf("create test bucket: %w", err)
	}
	defer func() {
		_ = client.RemoveObject(context.Background(), bucket, objectKey, minio.RemoveObjectOptions{})
		_ = client.RemoveBucket(context.Background(), bucket)
	}()

	core := minio.Core{Client: client}
	uploadID, err := core.NewMultipartUpload(ctx, bucket, objectKey, minio.PutObjectOptions{ContentType: "application/octet-stream"})
	if err != nil {
		return fmt.Errorf("initiate multipart upload: %w", err)
	}
	partOne := bytes.Repeat([]byte("a"), 5*1024*1024)
	partTwo := []byte("restart-survived")
	first, err := core.PutObjectPart(ctx, bucket, objectKey, uploadID, 1, bytes.NewReader(partOne), int64(len(partOne)), minio.PutObjectPartOptions{})
	if err != nil {
		return fmt.Errorf("upload first part: %w", err)
	}
	if err := os.WriteFile(readyFile, []byte("first-part-uploaded\n"), 0600); err != nil {
		return fmt.Errorf("write ready marker: %w", err)
	}
	if err := waitForFile(ctx, resumeFile); err != nil {
		return err
	}
	second, err := core.PutObjectPart(ctx, bucket, objectKey, uploadID, 2, bytes.NewReader(partTwo), int64(len(partTwo)), minio.PutObjectPartOptions{})
	if err != nil {
		return fmt.Errorf("upload second part after restart: %w", err)
	}
	if _, err := core.CompleteMultipartUpload(ctx, bucket, objectKey, uploadID, []minio.CompletePart{
		{PartNumber: first.PartNumber, ETag: first.ETag},
		{PartNumber: second.PartNumber, ETag: second.ETag},
	}, minio.PutObjectOptions{}); err != nil {
		return fmt.Errorf("complete multipart upload after restart: %w", err)
	}

	reader, err := client.GetObject(ctx, bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("open completed object: %w", err)
	}
	defer reader.Close()
	content, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("read completed object: %w", err)
	}
	want := append(append([]byte{}, partOne...), partTwo...)
	if !bytes.Equal(content, want) {
		return fmt.Errorf("completed content length=%d, want=%d", len(content), len(want))
	}
	return nil
}

func waitForFile(ctx context.Context, path string) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(path); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect resume marker: %w", err)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("waiting for resume marker: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}
