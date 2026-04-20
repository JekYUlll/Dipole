package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
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
	UploadAvatar(ctx context.Context, file multipart.File, header *multipart.FileHeader, userUUID string) (*UploadedObject, error)
	UploadGroupAvatar(ctx context.Context, file multipart.File, header *multipart.FileHeader, groupUUID string) (*UploadedObject, error)
	InitiateMessageMultipartUpload(ctx context.Context, fileName, contentType string) (*MultipartUpload, error)
	UploadMultipartPart(ctx context.Context, objectKey, uploadID string, partNumber int, reader io.Reader, size int64) (*UploadedPart, error)
	CompleteMessageMultipartUpload(ctx context.Context, uploadID, objectKey, fileName, contentType string, fileSize int64, parts []MultipartCompletePart) (*UploadedObject, error)
	AbortMultipartUpload(ctx context.Context, objectKey, uploadID string) error
}

type Downloader interface {
	PresignDownloadURL(ctx context.Context, bucket, objectKey string, expiry time.Duration) (string, error)
	OpenObject(ctx context.Context, bucket, objectKey string) (io.ReadCloser, error)
}

type ObjectStorage interface {
	Uploader
	Downloader
}

type MultipartUpload struct {
	Bucket      string
	ObjectKey   string
	UploadID    string
	FileName    string
	ContentType string
}

type UploadedPart struct {
	PartNumber int
	ETag       string
	Size       int64
}

type MultipartCompletePart struct {
	PartNumber int
	ETag       string
}

type MinIOUploader struct {
	client        *minio.Client
	presignClient *minio.Client // separate client using presign_endpoint; may be nil (falls back to client)
	core          minio.Core
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
		core:          minio.Core{Client: client},
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
	return u.uploadObject(ctx, file, header, buildMessageFileObjectKey)
}

func (u *MinIOUploader) UploadAvatar(ctx context.Context, file multipart.File, header *multipart.FileHeader, userUUID string) (*UploadedObject, error) {
	return u.uploadObject(ctx, file, header, func(fileName string) string {
		return buildAvatarObjectKey(strings.TrimSpace(userUUID), fileName)
	})
}

func (u *MinIOUploader) UploadGroupAvatar(ctx context.Context, file multipart.File, header *multipart.FileHeader, groupUUID string) (*UploadedObject, error) {
	return u.uploadObject(ctx, file, header, func(fileName string) string {
		return buildGroupAvatarObjectKey(strings.TrimSpace(groupUUID), fileName)
	})
}

func (u *MinIOUploader) InitiateMessageMultipartUpload(ctx context.Context, fileName, contentType string) (*MultipartUpload, error) {
	if u == nil || u.client == nil {
		return nil, fmt.Errorf("storage uploader is not initialized")
	}

	objectKey := buildMessageFileObjectKey(strings.TrimSpace(fileName))
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}

	uploadID, err := u.core.NewMultipartUpload(ctx, u.bucket, objectKey, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return nil, fmt.Errorf("initiate multipart upload: %w", err)
	}

	return &MultipartUpload{
		Bucket:      u.bucket,
		ObjectKey:   objectKey,
		UploadID:    uploadID,
		FileName:    strings.TrimSpace(fileName),
		ContentType: strings.TrimSpace(contentType),
	}, nil
}

func (u *MinIOUploader) UploadMultipartPart(ctx context.Context, objectKey, uploadID string, partNumber int, reader io.Reader, size int64) (*UploadedPart, error) {
	if u == nil || u.client == nil {
		return nil, fmt.Errorf("storage uploader is not initialized")
	}
	if strings.TrimSpace(objectKey) == "" || strings.TrimSpace(uploadID) == "" {
		return nil, fmt.Errorf("object key and upload id are required")
	}
	if partNumber <= 0 {
		return nil, fmt.Errorf("part number must be greater than 0")
	}
	if size <= 0 {
		return nil, fmt.Errorf("part size must be greater than 0")
	}

	part, err := u.core.PutObjectPart(ctx, u.bucket, objectKey, uploadID, partNumber, reader, size, minio.PutObjectPartOptions{})
	if err != nil {
		return nil, fmt.Errorf("upload multipart part: %w", err)
	}

	return &UploadedPart{
		PartNumber: part.PartNumber,
		ETag:       strings.Trim(part.ETag, "\""),
		Size:       part.Size,
	}, nil
}

func (u *MinIOUploader) CompleteMessageMultipartUpload(ctx context.Context, uploadID, objectKey, fileName, contentType string, fileSize int64, parts []MultipartCompletePart) (*UploadedObject, error) {
	if u == nil || u.client == nil {
		return nil, fmt.Errorf("storage uploader is not initialized")
	}
	if strings.TrimSpace(objectKey) == "" || strings.TrimSpace(uploadID) == "" {
		return nil, fmt.Errorf("object key and upload id are required")
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("multipart parts are required")
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}

	minioParts := make([]minio.CompletePart, len(parts))
	for i, part := range parts {
		minioParts[i] = minio.CompletePart{
			PartNumber: part.PartNumber,
			ETag:       strings.TrimSpace(strings.Trim(part.ETag, "\"")),
		}
	}

	if _, err := u.core.CompleteMultipartUpload(ctx, u.bucket, objectKey, uploadID, minioParts, minio.PutObjectOptions{
		ContentType: contentType,
	}); err != nil {
		return nil, fmt.Errorf("complete multipart upload: %w", err)
	}

	return &UploadedObject{
		Bucket:      u.bucket,
		ObjectKey:   objectKey,
		FileName:    strings.TrimSpace(fileName),
		FileSize:    fileSize,
		ContentType: contentType,
		URL:         u.objectURL(objectKey),
	}, nil
}

func (u *MinIOUploader) AbortMultipartUpload(ctx context.Context, objectKey, uploadID string) error {
	if u == nil || u.client == nil {
		return fmt.Errorf("storage uploader is not initialized")
	}
	if strings.TrimSpace(objectKey) == "" || strings.TrimSpace(uploadID) == "" {
		return fmt.Errorf("object key and upload id are required")
	}

	if err := u.core.AbortMultipartUpload(ctx, u.bucket, objectKey, uploadID); err != nil {
		return fmt.Errorf("abort multipart upload: %w", err)
	}
	return nil
}

func (u *MinIOUploader) uploadObject(ctx context.Context, file multipart.File, header *multipart.FileHeader, keyBuilder func(fileName string) string) (*UploadedObject, error) {
	if u == nil || u.client == nil {
		return nil, fmt.Errorf("storage uploader is not initialized")
	}

	fileName := strings.TrimSpace(header.Filename)
	objectKey := keyBuilder(fileName)
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

func (u *MinIOUploader) OpenObject(ctx context.Context, bucket, objectKey string) (io.ReadCloser, error) {
	if u == nil || u.client == nil {
		return nil, fmt.Errorf("storage uploader is not initialized")
	}
	if strings.TrimSpace(bucket) == "" || strings.TrimSpace(objectKey) == "" {
		return nil, fmt.Errorf("bucket and object key are required")
	}

	object, err := u.client.GetObject(ctx, bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("open minio object: %w", err)
	}
	if _, err := object.Stat(); err != nil {
		_ = object.Close()
		return nil, fmt.Errorf("stat minio object: %w", err)
	}

	return object, nil
}

func buildMessageFileObjectKey(fileName string) string {
	ext := strings.ToLower(filepath.Ext(fileName))
	datePath := time.Now().UTC().Format("2006/01/02")
	return "message-files/" + datePath + "/" + generateObjectID() + ext
}

func buildAvatarObjectKey(userUUID, fileName string) string {
	ext := strings.ToLower(filepath.Ext(fileName))
	if userUUID == "" {
		userUUID = "unknown"
	}
	datePath := time.Now().UTC().Format("2006/01/02")
	return "avatars/" + datePath + "/" + userUUID + "-" + generateObjectID() + ext
}

func buildGroupAvatarObjectKey(groupUUID, fileName string) string {
	ext := strings.ToLower(filepath.Ext(fileName))
	if groupUUID == "" {
		groupUUID = "unknown-group"
	}
	datePath := time.Now().UTC().Format("2006/01/02")
	return "group-avatars/" + datePath + "/" + groupUUID + "-" + generateObjectID() + ext
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
