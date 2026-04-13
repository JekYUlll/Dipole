package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"mime/multipart"
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

type MinIOUploader struct {
	client        *minio.Client
	bucket        string
	publicBaseURL string
}

var Client Uploader

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
