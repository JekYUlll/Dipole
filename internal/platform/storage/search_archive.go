package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	platformarchive "github.com/JekYUlll/Dipole/internal/platform/archive"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type VersionedArchiveConfig struct {
	Endpoint         string
	AccessKey        string
	SecretKey        string
	UseSSL           bool
	Bucket           string
	MinimumRetention time.Duration
}

type SearchArchiveConfig = VersionedArchiveConfig

type VersionedArchiveStore struct {
	client           searchArchiveObjectClient
	bucket           string
	minimumRetention time.Duration
}

type searchArchiveObjectClient interface {
	ValidateBucket(context.Context, string) error
	PutFile(context.Context, string, string, string, time.Time) (platformarchive.ObjectVersion, error)
	GetVersion(context.Context, string, platformarchive.ObjectVersion) (io.ReadCloser, error)
}

type SearchArchiveStore = VersionedArchiveStore

func NewVersionedArchiveStore(cfg VersionedArchiveConfig) (*VersionedArchiveStore, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" || strings.TrimSpace(cfg.AccessKey) == "" || strings.TrimSpace(cfg.SecretKey) == "" || strings.TrimSpace(cfg.Bucket) == "" || cfg.MinimumRetention <= 0 {
		return nil, errors.New("versioned archive MinIO configuration is incomplete")
	}
	client, err := minio.New(cfg.Endpoint, &minio.Options{Creds: credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""), Secure: cfg.UseSSL})
	if err != nil {
		return nil, err
	}
	return newVersionedArchiveStore(&minioSearchArchiveClient{client: client}, cfg.Bucket, cfg.MinimumRetention), nil
}

func NewSearchArchiveStore(cfg SearchArchiveConfig) (*SearchArchiveStore, error) {
	return NewVersionedArchiveStore(cfg)
}

func newVersionedArchiveStore(client searchArchiveObjectClient, bucket string, minimumRetention time.Duration) *VersionedArchiveStore {
	return &VersionedArchiveStore{client: client, bucket: strings.TrimSpace(bucket), minimumRetention: minimumRetention}
}

func newSearchArchiveStore(client searchArchiveObjectClient, bucket string, minimumRetention time.Duration) *SearchArchiveStore {
	return newVersionedArchiveStore(client, bucket, minimumRetention)
}

func (s *VersionedArchiveStore) ValidateRetention(ctx context.Context, retention time.Duration) error {
	if retention < s.minimumRetention {
		return fmt.Errorf("archive retention %s is below minimum %s", retention, s.minimumRetention)
	}
	return s.client.ValidateBucket(ctx, s.bucket)
}

func (s *VersionedArchiveStore) PutFile(ctx context.Context, objectKey, localPath string, retainUntil time.Time) (platformarchive.ObjectVersion, error) {
	return s.client.PutFile(ctx, s.bucket, objectKey, localPath, retainUntil)
}

func (s *VersionedArchiveStore) GetVersion(ctx context.Context, version platformarchive.ObjectVersion, writer io.Writer) error {
	if version.Bucket != "" && version.Bucket != s.bucket {
		return errors.New("archive receipt bucket does not match configured bucket")
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
		return errors.New("archive bucket versioning is not enabled")
	}
	objectLock, _, _, _, err := c.client.GetObjectLockConfig(ctx, bucket)
	if err != nil {
		return err
	}
	if objectLock != "Enabled" {
		return errors.New("archive bucket object lock is not enabled")
	}
	return nil
}

func (c *minioSearchArchiveClient) PutFile(ctx context.Context, bucket, objectKey, localPath string, retainUntil time.Time) (platformarchive.ObjectVersion, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return platformarchive.ObjectVersion{}, err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return platformarchive.ObjectVersion{}, err
	}
	info, err := c.client.PutObject(ctx, bucket, objectKey, file, stat.Size(), minio.PutObjectOptions{
		ContentType: "application/octet-stream", Mode: minio.Governance, RetainUntilDate: retainUntil,
	})
	if err != nil {
		return platformarchive.ObjectVersion{}, err
	}
	return platformarchive.ObjectVersion{Bucket: bucket, ObjectKey: objectKey, VersionID: info.VersionID, ETag: info.ETag}, nil
}

func (c *minioSearchArchiveClient) GetVersion(ctx context.Context, bucket string, version platformarchive.ObjectVersion) (io.ReadCloser, error) {
	return c.client.GetObject(ctx, bucket, version.ObjectKey, minio.GetObjectOptions{VersionID: version.VersionID})
}
