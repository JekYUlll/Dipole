package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"mime/multipart"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/model"
	platformStorage "github.com/JekYUlll/Dipole/internal/platform/storage"
)

var (
	ErrFileMissing            = errors.New("file is missing")
	ErrFileTooLarge           = errors.New("file is too large")
	ErrFileStorageUnavailable = errors.New("file storage is unavailable")
	ErrFileNotFound           = errors.New("file not found")
	ErrFilePermissionDenied   = errors.New("file permission denied")
	ErrFileExpired            = errors.New("file is expired")
)

type fileRepository interface {
	Create(file *model.UploadedFile) error
	GetByUUID(uuid string) (*model.UploadedFile, error)
}

type fileMessageRepository interface {
	FindLatestAccessibleFileMessage(fileUUID, userUUID string) (*model.Message, error)
}

type fileStorage interface {
	platformStorage.Uploader
	platformStorage.Downloader
}

type FileDownloadResult struct {
	FileID      string     `json:"file_id"`
	FileName    string     `json:"file_name"`
	ContentType string     `json:"content_type"`
	FileSize    int64      `json:"file_size"`
	DownloadURL string     `json:"download_url"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

type FileService struct {
	repo             fileRepository
	messageRepo      fileMessageRepository
	storage          fileStorage
	maxFileSizeBytes int64
	downloadURLTTL   time.Duration
}

func NewFileService(repo fileRepository, messageRepo fileMessageRepository, storage fileStorage) *FileService {
	storageCfg := config.StorageConfig()
	return newFileService(
		repo,
		messageRepo,
		storage,
		storageCfg.FileMaxSizeMB*1024*1024,
		time.Duration(storageCfg.DownloadURLTTLMinutes)*time.Minute,
	)
}

func newFileService(repo fileRepository, messageRepo fileMessageRepository, storage fileStorage, maxFileSizeBytes int64, downloadURLTTL time.Duration) *FileService {
	return &FileService{
		repo:             repo,
		messageRepo:      messageRepo,
		storage:          storage,
		maxFileSizeBytes: maxFileSizeBytes,
		downloadURLTTL:   downloadURLTTL,
	}
}

func (s *FileService) UploadMessageFile(uploaderUUID string, header *multipart.FileHeader) (*model.UploadedFile, error) {
	if header == nil {
		return nil, ErrFileMissing
	}
	if s.storage == nil {
		return nil, ErrFileStorageUnavailable
	}

	if s.maxFileSizeBytes > 0 && header.Size > s.maxFileSizeBytes {
		return nil, ErrFileTooLarge
	}

	file, err := header.Open()
	if err != nil {
		return nil, fmt.Errorf("open uploaded file: %w", err)
	}
	defer file.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	uploaded, err := s.storage.UploadMessageFile(ctx, file, header)
	if err != nil {
		return nil, fmt.Errorf("upload message file: %w", err)
	}

	record := &model.UploadedFile{
		UUID:         generateFileUUID(),
		UploaderUUID: strings.TrimSpace(uploaderUUID),
		Bucket:       uploaded.Bucket,
		ObjectKey:    uploaded.ObjectKey,
		FileName:     uploaded.FileName,
		FileSize:     uploaded.FileSize,
		ContentType:  uploaded.ContentType,
		URL:          uploaded.URL,
	}
	if err := s.repo.Create(record); err != nil {
		return nil, fmt.Errorf("persist uploaded file: %w", err)
	}

	return record, nil
}

func (s *FileService) GetOwnedFile(uploaderUUID, fileUUID string) (*model.UploadedFile, error) {
	file, err := s.repo.GetByUUID(strings.TrimSpace(fileUUID))
	if err != nil {
		return nil, fmt.Errorf("get uploaded file: %w", err)
	}
	if file == nil {
		return nil, ErrFileNotFound
	}
	if file.UploaderUUID != strings.TrimSpace(uploaderUUID) {
		return nil, ErrFilePermissionDenied
	}

	return file, nil
}

func (s *FileService) CreateDownloadLink(currentUserUUID, fileUUID string) (*FileDownloadResult, error) {
	if s.storage == nil {
		return nil, ErrFileStorageUnavailable
	}

	file, err := s.repo.GetByUUID(strings.TrimSpace(fileUUID))
	if err != nil {
		return nil, fmt.Errorf("get uploaded file: %w", err)
	}
	if file == nil {
		return nil, ErrFileNotFound
	}

	now := time.Now().UTC()
	if file.UploaderUUID != strings.TrimSpace(currentUserUUID) {
		if s.messageRepo == nil {
			return nil, ErrFilePermissionDenied
		}
		message, err := s.messageRepo.FindLatestAccessibleFileMessage(file.UUID, strings.TrimSpace(currentUserUUID))
		if err != nil {
			return nil, fmt.Errorf("find accessible file message: %w", err)
		}
		if message == nil {
			return nil, ErrFilePermissionDenied
		}
		if message.FileExpiresAt != nil && !message.FileExpiresAt.After(now) {
			return nil, ErrFileExpired
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	downloadURL, err := s.storage.PresignDownloadURL(ctx, file.Bucket, file.ObjectKey, s.downloadURLTTL)
	if err != nil {
		return nil, fmt.Errorf("presign download url: %w", err)
	}

	return &FileDownloadResult{
		FileID:      file.UUID,
		FileName:    file.FileName,
		ContentType: file.ContentType,
		FileSize:    file.FileSize,
		DownloadURL: downloadURL,
	}, nil
}

func generateFileUUID() string {
	buf := make([]byte, 10)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Errorf("generate file uuid: %w", err))
	}

	return "F" + strings.ToUpper(hex.EncodeToString(buf))
}
