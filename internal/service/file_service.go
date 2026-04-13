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
)

type fileRepository interface {
	Create(file *model.UploadedFile) error
	GetByUUID(uuid string) (*model.UploadedFile, error)
}

type FileService struct {
	repo             fileRepository
	uploader         platformStorage.Uploader
	maxFileSizeBytes int64
}

func NewFileService(repo fileRepository, uploader platformStorage.Uploader) *FileService {
	return newFileService(repo, uploader, config.StorageConfig().FileMaxSizeMB*1024*1024)
}

func newFileService(repo fileRepository, uploader platformStorage.Uploader, maxFileSizeBytes int64) *FileService {
	return &FileService{
		repo:             repo,
		uploader:         uploader,
		maxFileSizeBytes: maxFileSizeBytes,
	}
}

func (s *FileService) UploadMessageFile(uploaderUUID string, header *multipart.FileHeader) (*model.UploadedFile, error) {
	if header == nil {
		return nil, ErrFileMissing
	}
	if s.uploader == nil {
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

	uploaded, err := s.uploader.UploadMessageFile(ctx, file, header)
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

func generateFileUUID() string {
	buf := make([]byte, 10)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Errorf("generate file uuid: %w", err))
	}

	return "F" + strings.ToUpper(hex.EncodeToString(buf))
}
