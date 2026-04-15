package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"mime/multipart"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/JekYUlll/Dipole/internal/config"
)

type UploadedObject struct {
	Bucket      string
	ObjectKey   string
	FileName    string
	FileSize    int64
	ContentType string
	URL         string
}

type Uploader interface {
	UploadMessageFile(ctx context.Context, file multipart.File, header *multipart.FileHeader) (*UploadedObject, error)
}

type Downloader interface {
	PresignDownloadURL(ctx context.Context, bucket, objectKey string, expiry time.Duration) (string, error)
}

type ObjectStorage interface {
	Uploader
	Downloader
}

type MinIOUploader struct {
	client        *minio.Client
	presignClient *minio.Client // separate client using presign_endpoint; may be nil (falls back to client)
	bucket        string
	publicBaseURL string
}

var Client ObjectStorage

func Init() error {
	cfg := config.StorageConfig()
	if !cfg.Enabled {
		Client = nil
		return nil
	}
	if strings.TrimSpace(strings.ToLower(cfg.Provider)) != "minio" {
		return fmt.Errorf("unsupported storage provider: %s", cfg.Provider)
	}

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return fmt.Errorf("create minio client: %w", err)
	}

	uploader := &MinIOUploader{
		client:        client,
		bucket:        strings.TrimSpace(cfg.Bucket),
		publicBaseURL: strings.TrimRight(strings.TrimSpace(cfg.PublicBaseURL), "/"),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	exists, err := client.BucketExists(ctx, uploader.bucket)
	if err != nil {
		return fmt.Errorf("check minio bucket exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("minio bucket %s does not exist", uploader.bucket)
	}

	// If a separate presign endpoint is configured (e.g. a LAN/public address that
	// differs from the internal upload endpoint), create a dedicated client for it.
	// MinIO embeds the signing host in the HMAC signature, so the client used for
	// presigning must use the same host that browsers will ultimately reach.
	// We fetch the bucket region via the internal client first so the presign client
	// never needs to make a network call (it can't reach the internal endpoint).
	if presignEndpoint := strings.TrimSpace(cfg.PresignEndpoint); presignEndpoint != "" {
		region, _ := client.GetBucketLocation(ctx, uploader.bucket)
		presignClient, err := minio.New(presignEndpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
			Secure: cfg.UseSSL,
			Region: region,
		})
		if err != nil {
			return fmt.Errorf("create minio presign client: %w", err)
		}
		uploader.presignClient = presignClient
	}

	Client = uploader
	return nil
}

func (u *MinIOUploader) UploadMessageFile(ctx context.Context, file multipart.File, header *multipart.FileHeader) (*UploadedObject, error) {
	if u == nil || u.client == nil {
		return nil, fmt.Errorf("storage uploader is not initialized")
	}

	fileName := strings.TrimSpace(header.Filename)
	objectKey := buildObjectKey(fileName)
	contentType := detectContentType(header)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	_, err := u.client.PutObject(ctx, u.bucket, objectKey, file, header.Size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return nil, fmt.Errorf("upload object to minio: %w", err)
	}

	return &UploadedObject{
		Bucket:      u.bucket,
		ObjectKey:   objectKey,
		FileName:    fileName,
		FileSize:    header.Size,
		ContentType: contentType,
		URL:         u.objectURL(objectKey),
	}, nil
}

func (u *MinIOUploader) objectURL(objectKey string) string {
	if u.publicBaseURL != "" {
		return u.publicBaseURL + "/" + objectKey
	}

	scheme := "http"
	if config.StorageConfig().UseSSL {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/%s/%s", scheme, config.StorageConfig().Endpoint, u.bucket, objectKey)
}

func (u *MinIOUploader) PresignDownloadURL(ctx context.Context, bucket, objectKey string, expiry time.Duration) (string, error) {
	if u == nil || u.client == nil {
		return "", fmt.Errorf("storage uploader is not initialized")
	}
	if strings.TrimSpace(bucket) == "" || strings.TrimSpace(objectKey) == "" {
		return "", fmt.Errorf("bucket and object key are required")
	}
	if expiry <= 0 {
		expiry = 10 * time.Minute
	}

	// Use the presign client when available — it was initialized with the public/LAN
	// endpoint and a pre-cached region, so it signs URLs with the host browsers reach
	// without making any network calls.
	c := u.client
	if u.presignClient != nil {
		c = u.presignClient
	}

	presignedURL, err := c.PresignedGetObject(ctx, bucket, objectKey, expiry, url.Values{})
	if err != nil {
		return "", fmt.Errorf("presign minio object url: %w", err)
	}

	return presignedURL.String(), nil
}

func buildObjectKey(fileName string) string {
	ext := strings.ToLower(filepath.Ext(fileName))
	datePath := time.Now().UTC().Format("2006/01/02")
	return "message-files/" + datePath + "/" + generateObjectID() + ext
}

func generateObjectID() string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Errorf("generate object id: %w", err))
	}

	return strings.ToUpper(hex.EncodeToString(buf))
}

func detectContentType(header *multipart.FileHeader) string {
	if header == nil {
		return ""
	}
	return strings.TrimSpace(header.Header.Get("Content-Type"))
}
