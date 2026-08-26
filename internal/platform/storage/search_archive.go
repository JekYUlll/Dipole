package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	searchbackfill "github.com/JekYUlll/Dipole/internal/backfill/search"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type SearchArchiveConfig struct {
	Endpoint         string
	AccessKey        string
	SecretKey        string
	UseSSL           bool
	Bucket           string
	MinimumRetention time.Duration
}

type SearchArchiveStore struct {
	client           searchArchiveObjectClient
	bucket           string
	minimumRetention time.Duration
}

type searchArchiveObjectClient interface {
	ValidateBucket(context.Context, string) error
	PutFile(context.Context, string, string, string, time.Time) (searchbackfill.ArchiveObjectVersion, error)
	GetVersion(context.Context, string, searchbackfill.ArchiveObjectVersion) (io.ReadCloser, error)
}

func NewSearchArchiveStore(cfg SearchArchiveConfig) (*SearchArchiveStore, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" || strings.TrimSpace(cfg.AccessKey) == "" || strings.TrimSpace(cfg.SecretKey) == "" || strings.TrimSpace(cfg.Bucket) == "" || cfg.MinimumRetention <= 0 {
		return nil, errors.New("Search archive MinIO configuration is incomplete")
	}
	client, err := minio.New(cfg.Endpoint, &minio.Options{Creds: credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""), Secure: cfg.UseSSL})
	if err != nil {
		return nil, err
	}
	return newSearchArchiveStore(&minioSearchArchiveClient{client: client}, cfg.Bucket, cfg.MinimumRetention), nil
}

func newSearchArchiveStore(client searchArchiveObjectClient, bucket string, minimumRetention time.Duration) *SearchArchiveStore {
	return &SearchArchiveStore{client: client, bucket: strings.TrimSpace(bucket), minimumRetention: minimumRetention}
}

func (s *SearchArchiveStore) ValidateRetention(ctx context.Context, retention time.Duration) error {
	if retention < s.minimumRetention {
		return fmt.Errorf("Search archive retention %s is below minimum %s", retention, s.minimumRetention)
	}
	return s.client.ValidateBucket(ctx, s.bucket)
}

func (s *SearchArchiveStore) PutFile(ctx context.Context, objectKey, localPath string, retainUntil time.Time) (searchbackfill.ArchiveObjectVersion, error) {
	return s.client.PutFile(ctx, s.bucket, objectKey, localPath, retainUntil)
}

func (s *SearchArchiveStore) GetVersion(ctx context.Context, version searchbackfill.ArchiveObjectVersion, writer io.Writer) error {
	if version.Bucket != "" && version.Bucket != s.bucket {
		return errors.New("Search archive receipt bucket does not match configured bucket")
	}
	reader, err := s.client.GetVersion(ctx, s.bucket, version)
	if err != nil {
		return err
	}
	defer reader.Close()
	_, err = io.Copy(writer, reader)
	return err
}

type minioSearchArchiveClient struct{ client *minio.Client }

func (c *minioSearchArchiveClient) ValidateBucket(ctx context.Context, bucket string) error {
	versioning, err := c.client.GetBucketVersioning(ctx, bucket)
	if err != nil {
		return err
	}
	if !versioning.Enabled() {
		return errors.New("Search archive bucket versioning is not enabled")
	}
	objectLock, _, _, _, err := c.client.GetObjectLockConfig(ctx, bucket)
	if err != nil {
		return err
	}
	if objectLock != "Enabled" {
		return errors.New("Search archive bucket object lock is not enabled")
	}
	return nil
}

func (c *minioSearchArchiveClient) PutFile(ctx context.Context, bucket, objectKey, localPath string, retainUntil time.Time) (searchbackfill.ArchiveObjectVersion, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return searchbackfill.ArchiveObjectVersion{}, err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return searchbackfill.ArchiveObjectVersion{}, err
	}
	info, err := c.client.PutObject(ctx, bucket, objectKey, file, stat.Size(), minio.PutObjectOptions{
		ContentType: "application/octet-stream", Mode: minio.Governance, RetainUntilDate: retainUntil,
	})
	if err != nil {
		return searchbackfill.ArchiveObjectVersion{}, err
	}
	return searchbackfill.ArchiveObjectVersion{Bucket: bucket, ObjectKey: objectKey, VersionID: info.VersionID, ETag: info.ETag}, nil
}

func (c *minioSearchArchiveClient) GetVersion(ctx context.Context, bucket string, version searchbackfill.ArchiveObjectVersion) (io.ReadCloser, error) {
	return c.client.GetObject(ctx, bucket, version.ObjectKey, minio.GetObjectOptions{VersionID: version.VersionID})
}
