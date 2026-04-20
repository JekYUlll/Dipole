package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
	platformStorage "github.com/JekYUlll/Dipole/internal/platform/storage"
)

type stubFileRepository struct {
	created *model.UploadedFile
	files   map[string]*model.UploadedFile
}

func (r *stubFileRepository) Create(file *model.UploadedFile) error {
	r.created = file
	if r.files == nil {
		r.files = map[string]*model.UploadedFile{}
	}
	r.files[file.UUID] = file
	return nil
}

func (r *stubFileRepository) GetByUUID(uuid string) (*model.UploadedFile, error) {
	return r.files[uuid], nil
}

type stubUploader struct {
	uploadFn            func(ctx context.Context, file multipart.File, header *multipart.FileHeader) (*platformStorage.UploadedObject, error)
	presignFn           func(ctx context.Context, bucket, objectKey string, expiry time.Duration) (string, error)
	initiateMultipartFn func(ctx context.Context, fileName, contentType string) (*platformStorage.MultipartUpload, error)
	uploadPartFn        func(ctx context.Context, objectKey, uploadID string, partNumber int, reader io.Reader, size int64) (*platformStorage.UploadedPart, error)
	completeMultipartFn func(ctx context.Context, uploadID, objectKey, fileName, contentType string, fileSize int64, parts []platformStorage.MultipartCompletePart) (*platformStorage.UploadedObject, error)
	abortMultipartFn    func(ctx context.Context, objectKey, uploadID string) error
}

func (u *stubUploader) UploadMessageFile(ctx context.Context, file multipart.File, header *multipart.FileHeader) (*platformStorage.UploadedObject, error) {
	if u.uploadFn == nil {
		return nil, errors.New("unexpected upload call")
	}
	return u.uploadFn(ctx, file, header)
}

func (u *stubUploader) UploadAvatar(ctx context.Context, file multipart.File, header *multipart.FileHeader, userUUID string) (*platformStorage.UploadedObject, error) {
	_ = userUUID
	if u.uploadFn == nil {
		return nil, errors.New("unexpected upload call")
	}
	return u.uploadFn(ctx, file, header)
}

func (u *stubUploader) UploadGroupAvatar(ctx context.Context, file multipart.File, header *multipart.FileHeader, groupUUID string) (*platformStorage.UploadedObject, error) {
	_ = groupUUID
	if u.uploadFn == nil {
		return nil, errors.New("unexpected upload call")
	}
	return u.uploadFn(ctx, file, header)
}

func (u *stubUploader) PresignDownloadURL(ctx context.Context, bucket, objectKey string, expiry time.Duration) (string, error) {
	if u.presignFn == nil {
		return "", errors.New("unexpected presign call")
	}
	return u.presignFn(ctx, bucket, objectKey, expiry)
}

func (u *stubUploader) OpenObject(ctx context.Context, bucket, objectKey string) (io.ReadCloser, error) {
	_ = ctx
	_ = bucket
	_ = objectKey
	return io.NopCloser(bytes.NewReader(nil)), nil
}

func (u *stubUploader) InitiateMessageMultipartUpload(ctx context.Context, fileName, contentType string) (*platformStorage.MultipartUpload, error) {
	if u.initiateMultipartFn == nil {
		return nil, errors.New("unexpected initiate multipart call")
	}
	return u.initiateMultipartFn(ctx, fileName, contentType)
}

func (u *stubUploader) UploadMultipartPart(ctx context.Context, objectKey, uploadID string, partNumber int, reader io.Reader, size int64) (*platformStorage.UploadedPart, error) {
	if u.uploadPartFn == nil {
		return nil, errors.New("unexpected upload multipart part call")
	}
	return u.uploadPartFn(ctx, objectKey, uploadID, partNumber, reader, size)
}

func (u *stubUploader) CompleteMessageMultipartUpload(ctx context.Context, uploadID, objectKey, fileName, contentType string, fileSize int64, parts []platformStorage.MultipartCompletePart) (*platformStorage.UploadedObject, error) {
	if u.completeMultipartFn == nil {
		return nil, errors.New("unexpected complete multipart call")
	}
	return u.completeMultipartFn(ctx, uploadID, objectKey, fileName, contentType, fileSize, parts)
}

func (u *stubUploader) AbortMultipartUpload(ctx context.Context, objectKey, uploadID string) error {
	if u.abortMultipartFn == nil {
		return errors.New("unexpected abort multipart call")
	}
	return u.abortMultipartFn(ctx, objectKey, uploadID)
}

type stubFileMessageRepository struct {
	message *model.Message
}

func (r *stubFileMessageRepository) FindLatestAccessibleFileMessage(fileUUID, userUUID string) (*model.Message, error) {
	return r.message, nil
}

type stubMultipartSessionStore struct {
	sessions map[string]*multipartUploadSession
	parts    map[string][]platformStorage.MultipartCompletePart
}

func (s *stubMultipartSessionStore) Create(ctx context.Context, session *multipartUploadSession, ttl time.Duration) error {
	_ = ctx
	_ = ttl
	if s.sessions == nil {
		s.sessions = map[string]*multipartUploadSession{}
	}
	s.sessions[session.SessionID] = session
	return nil
}

func (s *stubMultipartSessionStore) Get(ctx context.Context, sessionID string) (*multipartUploadSession, error) {
	_ = ctx
	return s.sessions[sessionID], nil
}

func (s *stubMultipartSessionStore) SavePart(ctx context.Context, sessionID string, part *platformStorage.UploadedPart, ttl time.Duration) error {
	_ = ctx
	_ = ttl
	if s.parts == nil {
		s.parts = map[string][]platformStorage.MultipartCompletePart{}
	}
	s.parts[sessionID] = append(s.parts[sessionID], platformStorage.MultipartCompletePart{
		PartNumber: part.PartNumber,
		ETag:       part.ETag,
	})
	return nil
}

func (s *stubMultipartSessionStore) ListParts(ctx context.Context, sessionID string) ([]platformStorage.MultipartCompletePart, error) {
	_ = ctx
	return s.parts[sessionID], nil
}

func (s *stubMultipartSessionStore) Delete(ctx context.Context, sessionID string) error {
	_ = ctx
	delete(s.sessions, sessionID)
	delete(s.parts, sessionID)
	return nil
}

func TestFileServiceUploadMessageFileSuccess(t *testing.T) {
	t.Parallel()

	repo := &stubFileRepository{}
	service := newFileService(repo, nil, &stubUploader{
		uploadFn: func(ctx context.Context, file multipart.File, header *multipart.FileHeader) (*platformStorage.UploadedObject, error) {
			return &platformStorage.UploadedObject{
				Bucket:      "dipole-files",
				ObjectKey:   "message-files/2026/04/13/X.txt",
				FileName:    header.Filename,
				FileSize:    header.Size,
				ContentType: header.Header.Get("Content-Type"),
				URL:         "http://127.0.0.1:9000/dipole-files/message-files/2026/04/13/X.txt",
			}, nil
		},
	}, 50*1024*1024, 5*1024*1024, time.Hour, 10*time.Minute)

	header := newTestFileHeader(t, "hello.txt", "text/plain", []byte("hello"))
	file, err := service.UploadMessageFile("U100", header)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if file == nil || file.UUID == "" || file.UploaderUUID != "U100" {
		t.Fatalf("unexpected uploaded file: %+v", file)
	}
	if repo.created == nil || repo.created.FileName != "hello.txt" {
		t.Fatalf("expected uploaded file to be persisted, got %+v", repo.created)
	}
}

func TestFileServiceUploadMessageFileRejectsTooLargeFile(t *testing.T) {
	t.Parallel()

	service := newFileService(&stubFileRepository{}, nil, &stubUploader{}, 4, 5*1024*1024, time.Hour, 10*time.Minute)
	header := newTestFileHeader(t, "hello.txt", "text/plain", []byte("hello"))

	_, err := service.UploadMessageFile("U100", header)
	if !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("expected ErrFileTooLarge, got %v", err)
	}
}

func TestFileServiceGetOwnedFileRejectsOtherUploader(t *testing.T) {
	t.Parallel()

	service := newFileService(&stubFileRepository{
		files: map[string]*model.UploadedFile{
			"F100": {UUID: "F100", UploaderUUID: "U200"},
		},
	}, nil, nil, 0, 5*1024*1024, time.Hour, 10*time.Minute)

	_, err := service.GetOwnedFile("U100", "F100")
	if !errors.Is(err, ErrFilePermissionDenied) {
		t.Fatalf("expected ErrFilePermissionDenied, got %v", err)
	}
}

func newTestFileHeader(t *testing.T, fileName, contentType string, content []byte) *multipart.FileHeader {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": {`form-data; name="file"; filename="` + fileName + `"`},
		"Content-Type":        {contentType},
	})
	if err != nil {
		t.Fatalf("create multipart part: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write multipart content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	reader := multipart.NewReader(&body, writer.Boundary())
	form, err := reader.ReadForm(int64(len(content)) + 1024)
	if err != nil {
		t.Fatalf("read multipart form: %v", err)
	}
	files := form.File["file"]
	if len(files) != 1 {
		t.Fatalf("expected one file header, got %d", len(files))
	}

	return files[0]
}

func TestFileServiceCreateDownloadLinkSuccess(t *testing.T) {
	t.Parallel()

	expiresAt := time.Now().UTC().Add(24 * time.Hour)
	service := newFileService(
		&stubFileRepository{
			files: map[string]*model.UploadedFile{
				"F100": {
					UUID:        "F100",
					Bucket:      "dipole-files",
					ObjectKey:   "message-files/2026/04/14/F100.txt",
					FileName:    "hello.txt",
					FileSize:    5,
					ContentType: "text/plain",
				},
			},
		},
		&stubFileMessageRepository{
			message: &model.Message{
				FileID:        "F100",
				FileExpiresAt: &expiresAt,
			},
		},
		&stubUploader{
			presignFn: func(ctx context.Context, bucket, objectKey string, expiry time.Duration) (string, error) {
				return "https://signed.example/download", nil
			},
		},
		50*1024*1024,
		5*1024*1024,
		time.Hour,
		10*time.Minute,
	)

	result, err := service.CreateDownloadLink("U200", "F100")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil || result.DownloadURL == "" {
		t.Fatalf("expected download url, got %+v", result)
	}
}

func TestFileServiceCreateDownloadLinkRejectsExpired(t *testing.T) {
	t.Parallel()

	expiresAt := time.Now().UTC().Add(-time.Hour)
	service := newFileService(
		&stubFileRepository{
			files: map[string]*model.UploadedFile{
				"F100": {UUID: "F100", Bucket: "dipole-files", ObjectKey: "message-files/F100.txt"},
			},
		},
		&stubFileMessageRepository{
			message: &model.Message{
				FileID:        "F100",
				FileExpiresAt: &expiresAt,
			},
		},
		&stubUploader{
			presignFn: func(ctx context.Context, bucket, objectKey string, expiry time.Duration) (string, error) {
				return "https://signed.example/download", nil
			},
		},
		50*1024*1024,
		5*1024*1024,
		time.Hour,
		10*time.Minute,
	)

	_, err := service.CreateDownloadLink("U200", "F100")
	if !errors.Is(err, ErrFileExpired) {
		t.Fatalf("expected ErrFileExpired, got %v", err)
	}
}

func TestFileServiceMultipartUploadFlow(t *testing.T) {
	t.Parallel()

	repo := &stubFileRepository{}
	sessionStore := &stubMultipartSessionStore{}
	service := newFileService(
		repo,
		nil,
		&stubUploader{
			initiateMultipartFn: func(ctx context.Context, fileName, contentType string) (*platformStorage.MultipartUpload, error) {
				return &platformStorage.MultipartUpload{
					Bucket:      "dipole-files",
					ObjectKey:   "message-files/2026/04/20/F100.bin",
					UploadID:    "UPLOAD-1",
					FileName:    fileName,
					ContentType: contentType,
				}, nil
			},
			uploadPartFn: func(ctx context.Context, objectKey, uploadID string, partNumber int, reader io.Reader, size int64) (*platformStorage.UploadedPart, error) {
				return &platformStorage.UploadedPart{
					PartNumber: partNumber,
					ETag:       fmt.Sprintf("etag-%d", partNumber),
					Size:       size,
				}, nil
			},
			completeMultipartFn: func(ctx context.Context, uploadID, objectKey, fileName, contentType string, fileSize int64, parts []platformStorage.MultipartCompletePart) (*platformStorage.UploadedObject, error) {
				if len(parts) != 2 {
					t.Fatalf("expected 2 parts, got %d", len(parts))
				}
				return &platformStorage.UploadedObject{
					Bucket:      "dipole-files",
					ObjectKey:   objectKey,
					FileName:    fileName,
					FileSize:    fileSize,
					ContentType: contentType,
					URL:         "http://127.0.0.1:9000/dipole-files/" + objectKey,
				}, nil
			},
		},
		50*1024*1024,
		5,
		time.Hour,
		10*time.Minute,
	)
	service.sessionStore = sessionStore

	initResult, err := service.InitiateMultipartUpload("U100", InitiateMultipartUploadInput{
		FileName:    "video.bin",
		FileSize:    8,
		ContentType: "application/octet-stream",
	})
	if err != nil {
		t.Fatalf("initiate multipart upload: %v", err)
	}
	if initResult.TotalParts != 2 {
		t.Fatalf("expected 2 parts, got %d", initResult.TotalParts)
	}

	if err := service.UploadMultipartPart("U100", initResult.SessionID, 1, 5, bytes.NewReader([]byte("12345"))); err != nil {
		t.Fatalf("upload part 1: %v", err)
	}
	if err := service.UploadMultipartPart("U100", initResult.SessionID, 2, 3, bytes.NewReader([]byte("678"))); err != nil {
		t.Fatalf("upload part 2: %v", err)
	}

	file, err := service.CompleteMultipartUpload("U100", initResult.SessionID)
	if err != nil {
		t.Fatalf("complete multipart upload: %v", err)
	}
	if file == nil || file.UUID == "" {
		t.Fatalf("expected uploaded file record, got %+v", file)
	}
}
