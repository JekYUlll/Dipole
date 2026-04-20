package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/model"
	platformStorage "github.com/JekYUlll/Dipole/internal/platform/storage"
)

var (
	ErrFileMissing              = errors.New("file is missing")
	ErrFileTooLarge             = errors.New("file is too large")
	ErrFileStorageUnavailable   = errors.New("file storage is unavailable")
	ErrFileNotFound             = errors.New("file not found")
	ErrFilePermissionDenied     = errors.New("file permission denied")
	ErrFileExpired              = errors.New("file is expired")
	ErrMultipartSessionNotFound = errors.New("multipart upload session not found")
	ErrMultipartSessionInvalid  = errors.New("multipart upload session is invalid")
	ErrMultipartPartInvalid     = errors.New("multipart upload part is invalid")
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

type FileContentResult struct {
	FileID      string
	FileName    string
	ContentType string
	FileSize    int64
	Content     io.ReadCloser
	Cleanup     func()
}

type FileService struct {
	repo                fileRepository
	messageRepo         fileMessageRepository
	storage             fileStorage
	sessionStore        multipartUploadSessionStore
	maxFileSizeBytes    int64
	multipartChunkSize  int64
	multipartSessionTTL time.Duration
	downloadURLTTL      time.Duration
}

func NewFileService(repo fileRepository, messageRepo fileMessageRepository, storage fileStorage) *FileService {
	storageCfg := config.StorageConfig()
	return newFileService(
		repo,
		messageRepo,
		storage,
		storageCfg.FileMaxSizeMB*1024*1024,
		maxInt64(storageCfg.MultipartChunkSizeMB, 5)*1024*1024,
		time.Duration(maxInt(storageCfg.MultipartSessionTTLMin, 60))*time.Minute,
		time.Duration(storageCfg.DownloadURLTTLMinutes)*time.Minute,
	)
}

func newFileService(repo fileRepository, messageRepo fileMessageRepository, storage fileStorage, maxFileSizeBytes int64, multipartChunkSize int64, multipartSessionTTL time.Duration, downloadURLTTL time.Duration) *FileService {
	return &FileService{
		repo:                repo,
		messageRepo:         messageRepo,
		storage:             storage,
		sessionStore:        newMultipartUploadSessionStore(),
		maxFileSizeBytes:    maxFileSizeBytes,
		multipartChunkSize:  multipartChunkSize,
		multipartSessionTTL: multipartSessionTTL,
		downloadURLTTL:      downloadURLTTL,
	}
}

type InitiateMultipartUploadInput struct {
	FileName    string `json:"file_name"`
	FileSize    int64  `json:"file_size"`
	ContentType string `json:"content_type"`
}

type InitiateMultipartUploadResult struct {
	SessionID  string `json:"session_id"`
	ChunkSize  int64  `json:"chunk_size"`
	TotalParts int    `json:"total_parts"`
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
		UUID:         generateUploadedFileUUID(),
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

func (s *FileService) InitiateMultipartUpload(uploaderUUID string, input InitiateMultipartUploadInput) (*InitiateMultipartUploadResult, error) {
	if s.storage == nil {
		return nil, ErrFileStorageUnavailable
	}
	if s.sessionStore == nil {
		return nil, ErrMultipartSessionInvalid
	}

	fileName := strings.TrimSpace(input.FileName)
	if fileName == "" {
		return nil, ErrFileMissing
	}
	if input.FileSize <= 0 {
		return nil, ErrMultipartSessionInvalid
	}
	if s.maxFileSizeBytes > 0 && input.FileSize > s.maxFileSizeBytes {
		return nil, ErrFileTooLarge
	}

	contentType := strings.TrimSpace(input.ContentType)
	if contentType == "" {
		contentType = detectFileContentType(fileName)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	upload, err := s.storage.InitiateMessageMultipartUpload(ctx, fileName, contentType)
	if err != nil {
		return nil, fmt.Errorf("initiate multipart upload: %w", err)
	}

	chunkSize := s.effectiveMultipartChunkSize()
	totalParts := int((input.FileSize + chunkSize - 1) / chunkSize)
	session := &multipartUploadSession{
		SessionID:    generateMultipartSessionID(),
		UploaderUUID: strings.TrimSpace(uploaderUUID),
		Bucket:       upload.Bucket,
		ObjectKey:    upload.ObjectKey,
		UploadID:     upload.UploadID,
		FileName:     fileName,
		FileSize:     input.FileSize,
		ContentType:  contentType,
		ChunkSize:    chunkSize,
		TotalParts:   totalParts,
		CreatedAt:    time.Now().UTC(),
	}
	if err := s.sessionStore.Create(ctx, session, s.effectiveMultipartSessionTTL()); err != nil {
		_ = s.storage.AbortMultipartUpload(ctx, upload.ObjectKey, upload.UploadID)
		return nil, fmt.Errorf("persist multipart session: %w", err)
	}

	return &InitiateMultipartUploadResult{
		SessionID:  session.SessionID,
		ChunkSize:  chunkSize,
		TotalParts: totalParts,
	}, nil
}

func (s *FileService) UploadMultipartPart(uploaderUUID, sessionID string, partNumber int, contentLength int64, body io.Reader) error {
	if s.storage == nil {
		return ErrFileStorageUnavailable
	}
	session, err := s.getOwnedMultipartSession(uploaderUUID, sessionID)
	if err != nil {
		return err
	}
	if partNumber <= 0 || partNumber > session.TotalParts {
		return ErrMultipartPartInvalid
	}
	if body == nil || contentLength <= 0 {
		return ErrMultipartPartInvalid
	}
	if contentLength > session.ChunkSize {
		return ErrMultipartPartInvalid
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	part, err := s.storage.UploadMultipartPart(ctx, session.ObjectKey, session.UploadID, partNumber, body, contentLength)
	if err != nil {
		return fmt.Errorf("upload multipart part: %w", err)
	}
	if err := s.sessionStore.SavePart(ctx, session.SessionID, part, s.effectiveMultipartSessionTTL()); err != nil {
		return fmt.Errorf("persist multipart part: %w", err)
	}
	return nil
}

func (s *FileService) CompleteMultipartUpload(uploaderUUID, sessionID string) (*model.UploadedFile, error) {
	if s.storage == nil {
		return nil, ErrFileStorageUnavailable
	}
	session, err := s.getOwnedMultipartSession(uploaderUUID, sessionID)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	parts, err := s.sessionStore.ListParts(ctx, session.SessionID)
	if err != nil {
		return nil, fmt.Errorf("list multipart parts: %w", err)
	}
	if err := validateMultipartParts(parts, session.TotalParts); err != nil {
		return nil, err
	}

	uploaded, err := s.storage.CompleteMessageMultipartUpload(
		ctx,
		session.UploadID,
		session.ObjectKey,
		session.FileName,
		session.ContentType,
		session.FileSize,
		parts,
	)
	if err != nil {
		return nil, fmt.Errorf("complete multipart upload: %w", err)
	}

	record := &model.UploadedFile{
		UUID:         generateUploadedFileUUID(),
		UploaderUUID: session.UploaderUUID,
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
	if err := s.sessionStore.Delete(ctx, session.SessionID); err != nil {
		return nil, fmt.Errorf("delete multipart session: %w", err)
	}

	return record, nil
}

func (s *FileService) AbortMultipartUpload(uploaderUUID, sessionID string) error {
	if s.storage == nil {
		return ErrFileStorageUnavailable
	}
	session, err := s.getOwnedMultipartSession(uploaderUUID, sessionID)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := s.storage.AbortMultipartUpload(ctx, session.ObjectKey, session.UploadID); err != nil {
		return err
	}
	if err := s.sessionStore.Delete(ctx, session.SessionID); err != nil {
		return fmt.Errorf("delete multipart session: %w", err)
	}
	return nil
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

func (s *FileService) OpenContent(currentUserUUID, fileUUID string) (*FileContentResult, error) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	content, err := s.storage.OpenObject(ctx, file.Bucket, file.ObjectKey)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("open file object: %w", err)
	}

	contentType := strings.TrimSpace(file.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	return &FileContentResult{
		FileID:      file.UUID,
		FileName:    file.FileName,
		ContentType: contentType,
		FileSize:    file.FileSize,
		Content:     content,
		Cleanup: func() {
			cancel()
		},
	}, nil
}

func (s *FileService) getOwnedMultipartSession(uploaderUUID, sessionID string) (*multipartUploadSession, error) {
	if s.sessionStore == nil {
		return nil, ErrMultipartSessionInvalid
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session, err := s.sessionStore.Get(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return nil, fmt.Errorf("get multipart session: %w", err)
	}
	if session == nil {
		return nil, ErrMultipartSessionNotFound
	}
	if session.UploaderUUID != strings.TrimSpace(uploaderUUID) {
		return nil, ErrFilePermissionDenied
	}
	return session, nil
}

func (s *FileService) effectiveMultipartChunkSize() int64 {
	if s.multipartChunkSize > 0 {
		return s.multipartChunkSize
	}
	return 5 * 1024 * 1024
}

func (s *FileService) effectiveMultipartSessionTTL() time.Duration {
	if s.multipartSessionTTL > 0 {
		return s.multipartSessionTTL
	}
	return time.Hour
}

func validateMultipartParts(parts []platformStorage.MultipartCompletePart, totalParts int) error {
	if totalParts <= 0 || len(parts) != totalParts {
		return ErrMultipartSessionInvalid
	}
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].PartNumber < parts[j].PartNumber
	})
	for idx, part := range parts {
		expected := idx + 1
		if part.PartNumber != expected || strings.TrimSpace(part.ETag) == "" {
			return ErrMultipartSessionInvalid
		}
	}
	return nil
}

func detectFileContentType(fileName string) string {
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(fileName)))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain"
	default:
		return "application/octet-stream"
	}
}

func maxInt64(v, fallback int64) int64 {
	if v > 0 {
		return v
	}
	return fallback
}

func maxInt(v, fallback int) int {
	if v > 0 {
		return v
	}
	return fallback
}
