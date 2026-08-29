package corefile

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	presignPartFn       func(ctx context.Context, objectKey, uploadID string, partNumber int, expiry time.Duration) (string, error)
	inspectPartFn       func(ctx context.Context, objectKey, uploadID string, partNumber int) (*platformStorage.UploadedPart, error)
	initiateMultipartFn func(ctx context.Context, fileName, contentType string) (*platformStorage.MultipartUpload, error)
	uploadPartFn        func(ctx context.Context, objectKey, uploadID string, partNumber int, reader io.Reader, size int64) (*platformStorage.UploadedPart, error)
	completeMultipartFn func(ctx context.Context, uploadID, objectKey, fileName, contentType string, fileSize int64, parts []platformStorage.MultipartCompletePart) (*platformStorage.UploadedObject, error)
	abortMultipartFn    func(ctx context.Context, objectKey, uploadID string) error
	openObjectFn        func(ctx context.Context, bucket, objectKey string) (io.ReadCloser, error)
	removeObjectFn      func(ctx context.Context, bucket, objectKey string) error
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

func (u *stubUploader) PresignMultipartPartURL(ctx context.Context, objectKey, uploadID string, partNumber int, expiry time.Duration) (string, error) {
	if u.presignPartFn == nil {
		return "", errors.New("unexpected multipart presign call")
	}
	return u.presignPartFn(ctx, objectKey, uploadID, partNumber, expiry)
}

func (u *stubUploader) InspectMultipartPart(ctx context.Context, objectKey, uploadID string, partNumber int) (*platformStorage.UploadedPart, error) {
	if u.inspectPartFn == nil {
		return nil, errors.New("unexpected multipart inspect call")
	}
	return u.inspectPartFn(ctx, objectKey, uploadID, partNumber)
}

func (u *stubUploader) OpenObject(ctx context.Context, bucket, objectKey string) (io.ReadCloser, error) {
	if u.openObjectFn != nil {
		return u.openObjectFn(ctx, bucket, objectKey)
	}
	_ = ctx
	_ = bucket
	_ = objectKey
	return io.NopCloser(bytes.NewReader(nil)), nil
}

func (u *stubUploader) RemoveObject(ctx context.Context, bucket, objectKey string) error {
	if u.removeObjectFn == nil {
		return errors.New("unexpected remove object call")
	}
	return u.removeObjectFn(ctx, bucket, objectKey)
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
		Size:       part.Size,
	})
	return nil
}

func TestValidateMultipartPartsRejectsIncorrectPartSizes(t *testing.T) {
	t.Parallel()

	parts := []platformStorage.MultipartCompletePart{
		{PartNumber: 1, ETag: "etag-1", Size: 4},
		{PartNumber: 2, ETag: "etag-2", Size: 4},
	}

	if err := validateMultipartParts(parts, 2, 8, 5); !errors.Is(err, ErrMultipartSessionInvalid) {
		t.Fatalf("expected invalid multipart session, got %v", err)
	}
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
				if _, err := io.Copy(io.Discard, reader); err != nil {
					return nil, err
				}
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

	if err := service.UploadMultipartPart("U100", initResult.SessionID, 1, 5, "", bytes.NewReader([]byte("12345"))); err != nil {
		t.Fatalf("upload part 1: %v", err)
	}
	if err := service.UploadMultipartPart("U100", initResult.SessionID, 2, 3, "", bytes.NewReader([]byte("678"))); err != nil {
		t.Fatalf("upload part 2: %v", err)
	}
	status, err := service.GetMultipartUploadStatus("U100", initResult.SessionID)
	if err != nil {
		t.Fatalf("get multipart upload status: %v", err)
	}
	if len(status.UploadedParts) != 2 || status.UploadedParts[1].Size != 3 {
		t.Fatalf("unexpected multipart status: %+v", status)
	}

	file, err := service.CompleteMultipartUpload("U100", initResult.SessionID)
	if err != nil {
		t.Fatalf("complete multipart upload: %v", err)
	}
	if file == nil || file.UUID == "" {
		t.Fatalf("expected uploaded file record, got %+v", file)
	}
}

func TestFileServiceMultipartChecksumRequiredAndVerified(t *testing.T) {
	t.Parallel()
	content := []byte("data")
	digest := sha256.Sum256(content)
	removed := false
	service := newFileService(&stubFileRepository{}, nil, &stubUploader{
		initiateMultipartFn: func(context.Context, string, string) (*platformStorage.MultipartUpload, error) {
			return &platformStorage.MultipartUpload{Bucket: "dipole-files", ObjectKey: "message-files/data", UploadID: "UPLOAD-CHECKSUM"}, nil
		},
		uploadPartFn: func(_ context.Context, _ string, _ string, partNumber int, reader io.Reader, size int64) (*platformStorage.UploadedPart, error) {
			_, _ = io.Copy(io.Discard, reader)
			return &platformStorage.UploadedPart{PartNumber: partNumber, ETag: "etag-1", Size: size}, nil
		},
		completeMultipartFn: func(_ context.Context, _, objectKey, fileName, contentType string, fileSize int64, _ []platformStorage.MultipartCompletePart) (*platformStorage.UploadedObject, error) {
			return &platformStorage.UploadedObject{Bucket: "dipole-files", ObjectKey: objectKey, FileName: fileName, ContentType: contentType, FileSize: fileSize}, nil
		},
		openObjectFn: func(context.Context, string, string) (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(content)), nil
		},
		removeObjectFn: func(context.Context, string, string) error { removed = true; return nil },
	}, 50*1024*1024, 5, time.Hour, 10*time.Minute)
	service.multipartRequireChecksum = true
	service.sessionStore = &stubMultipartSessionStore{}

	if _, err := service.InitiateMultipartUpload("U100", InitiateMultipartUploadInput{FileName: "data", FileSize: int64(len(content))}); !errors.Is(err, ErrMultipartChecksumRequired) {
		t.Fatalf("expected required checksum error, got %v", err)
	}
	initResult, err := service.InitiateMultipartUpload("U100", InitiateMultipartUploadInput{FileName: "data", FileSize: int64(len(content)), FileSHA256: hex.EncodeToString(digest[:])})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.UploadMultipartPart("U100", initResult.SessionID, 1, int64(len(content)), "", bytes.NewReader(content)); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CompleteMultipartUpload("U100", initResult.SessionID); err != nil {
		t.Fatal(err)
	}
	if removed {
		t.Fatal("matching checksum removed completed object")
	}
}

func TestFileServiceMultipartChecksumMismatchRemovesObject(t *testing.T) {
	t.Parallel()
	removed := false
	service := newFileService(&stubFileRepository{}, nil, &stubUploader{
		initiateMultipartFn: func(context.Context, string, string) (*platformStorage.MultipartUpload, error) {
			return &platformStorage.MultipartUpload{Bucket: "dipole-files", ObjectKey: "message-files/data", UploadID: "UPLOAD-MISMATCH"}, nil
		},
		uploadPartFn: func(_ context.Context, _ string, _ string, partNumber int, reader io.Reader, size int64) (*platformStorage.UploadedPart, error) {
			_, _ = io.Copy(io.Discard, reader)
			return &platformStorage.UploadedPart{PartNumber: partNumber, ETag: "etag-1", Size: size}, nil
		},
		completeMultipartFn: func(_ context.Context, _, objectKey, _, contentType string, fileSize int64, _ []platformStorage.MultipartCompletePart) (*platformStorage.UploadedObject, error) {
			return &platformStorage.UploadedObject{Bucket: "dipole-files", ObjectKey: objectKey, ContentType: contentType, FileSize: fileSize}, nil
		},
		openObjectFn: func(context.Context, string, string) (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader([]byte("actual"))), nil
		},
		removeObjectFn: func(context.Context, string, string) error { removed = true; return nil },
	}, 50*1024*1024, 5, time.Hour, 10*time.Minute)
	service.multipartRequireChecksum = true
	service.sessionStore = &stubMultipartSessionStore{}
	digest := sha256.Sum256([]byte("expected"))
	initResult, err := service.InitiateMultipartUpload("U100", InitiateMultipartUploadInput{FileName: "data", FileSize: 6, FileSHA256: hex.EncodeToString(digest[:])})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.UploadMultipartPart("U100", initResult.SessionID, 1, 5, "", bytes.NewReader([]byte("actua"))); err != nil {
		t.Fatal(err)
	}
	if err := service.UploadMultipartPart("U100", initResult.SessionID, 2, 1, "", bytes.NewReader([]byte("l"))); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CompleteMultipartUpload("U100", initResult.SessionID); !errors.Is(err, ErrMultipartChecksumMismatch) {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
	if !removed {
		t.Fatal("checksum mismatch did not remove completed object")
	}
}

func TestFileServicePresignMultipartPartsValidatesOwnerAndOrdersParts(t *testing.T) {
	t.Parallel()

	requested := make([]int, 0)
	service := newFileService(&stubFileRepository{}, nil, &stubUploader{
		presignPartFn: func(_ context.Context, objectKey, uploadID string, partNumber int, expiry time.Duration) (string, error) {
			if objectKey != "message-files/f.bin" || uploadID != "upload-1" || expiry != multipartPartURLTTL {
				t.Fatalf("unexpected signing input: %s %s %d %s", objectKey, uploadID, partNumber, expiry)
			}
			requested = append(requested, partNumber)
			return fmt.Sprintf("https://minio.test/part/%d", partNumber), nil
		},
	}, 50*1024*1024, 5, time.Hour, time.Minute)
	service.sessionStore = &stubMultipartSessionStore{}
	service.sessionStore.Create(context.Background(), &multipartUploadSession{
		SessionID: "session-1", UploaderUUID: "U100", ObjectKey: "message-files/f.bin", UploadID: "upload-1", TotalParts: 3,
	}, time.Hour)

	parts, err := service.PresignMultipartParts("U100", "session-1", []int{3, 1})
	if err != nil {
		t.Fatalf("presign parts: %v", err)
	}
	if fmt.Sprint(requested) != "[1 3]" || len(parts) != 2 || parts[0].PartNumber != 1 || parts[1].PartNumber != 3 {
		t.Fatalf("unexpected presigned parts: requested=%v parts=%+v", requested, parts)
	}
	if _, err := service.PresignMultipartParts("U200", "session-1", []int{1}); !errors.Is(err, ErrFilePermissionDenied) {
		t.Fatalf("expected owner rejection, got %v", err)
	}
}

func TestFileServiceRegisterMultipartPartVerifiesStorageMetadata(t *testing.T) {
	t.Parallel()

	service := newFileService(&stubFileRepository{}, nil, &stubUploader{
		inspectPartFn: func(_ context.Context, objectKey, uploadID string, partNumber int) (*platformStorage.UploadedPart, error) {
			if objectKey != "message-files/f.bin" || uploadID != "upload-1" || partNumber != 1 {
				t.Fatalf("unexpected inspect input: %s %s %d", objectKey, uploadID, partNumber)
			}
			return &platformStorage.UploadedPart{PartNumber: 1, ETag: "etag-1", Size: 5}, nil
		},
	}, 50*1024*1024, 5, time.Hour, time.Minute)
	store := &stubMultipartSessionStore{}
	service.sessionStore = store
	store.Create(context.Background(), &multipartUploadSession{
		SessionID: "session-1", UploaderUUID: "U100", ObjectKey: "message-files/f.bin", UploadID: "upload-1", TotalParts: 2, ChunkSize: 5,
	}, time.Hour)

	if err := service.RegisterMultipartPart("U100", "session-1", 1, RegisterMultipartPartInput{ETag: "\"etag-1\"", Size: 5}); err != nil {
		t.Fatalf("register part: %v", err)
	}
	if len(store.parts["session-1"]) != 1 || store.parts["session-1"][0].ETag != "etag-1" {
		t.Fatalf("part was not persisted: %+v", store.parts)
	}
	if err := service.RegisterMultipartPart("U100", "session-1", 1, RegisterMultipartPartInput{ETag: "wrong", Size: 5}); !errors.Is(err, ErrMultipartPartInvalid) {
		t.Fatalf("expected metadata rejection, got %v", err)
	}
}

func TestFileServiceMultipartUploadRejectsChecksumMismatch(t *testing.T) {
	t.Parallel()

	repo := &stubFileRepository{}
	sessionStore := &stubMultipartSessionStore{}
	service := newFileService(repo, nil, &stubUploader{
		initiateMultipartFn: func(context.Context, string, string) (*platformStorage.MultipartUpload, error) {
			return &platformStorage.MultipartUpload{Bucket: "b", ObjectKey: "k", UploadID: "u", FileName: "f", ContentType: "application/octet-stream"}, nil
		},
		uploadPartFn: func(_ context.Context, _, _ string, partNumber int, reader io.Reader, size int64) (*platformStorage.UploadedPart, error) {
			if _, err := io.Copy(io.Discard, reader); err != nil {
				return nil, err
			}
			return &platformStorage.UploadedPart{PartNumber: partNumber, ETag: "etag", Size: size}, nil
		},
	}, 50*1024*1024, 5, time.Hour, time.Minute)
	service.sessionStore = sessionStore

	initResult, err := service.InitiateMultipartUpload("U100", InitiateMultipartUploadInput{FileName: "f", FileSize: 4})
	if err != nil {
		t.Fatalf("initiate multipart upload: %v", err)
	}
	checksum := sha256.Sum256([]byte("wrong"))
	if err := service.UploadMultipartPart("U100", initResult.SessionID, 1, 4, hex.EncodeToString(checksum[:]), bytes.NewReader([]byte("data"))); !errors.Is(err, ErrMultipartPartInvalid) {
		t.Fatalf("expected checksum rejection, got %v", err)
	}
	if len(sessionStore.parts[initResult.SessionID]) != 0 {
		t.Fatal("checksum mismatch must not persist the part")
	}
}

func TestFileServiceMultipartUploadRejectsShortPartBody(t *testing.T) {
	t.Parallel()

	service := newFileService(&stubFileRepository{}, nil, &stubUploader{
		initiateMultipartFn: func(context.Context, string, string) (*platformStorage.MultipartUpload, error) {
			return &platformStorage.MultipartUpload{Bucket: "b", ObjectKey: "k", UploadID: "u", FileName: "f", ContentType: "application/octet-stream"}, nil
		},
		uploadPartFn: func(_ context.Context, _, _ string, partNumber int, reader io.Reader, size int64) (*platformStorage.UploadedPart, error) {
			if _, err := io.Copy(io.Discard, reader); err != nil {
				return nil, err
			}
			return &platformStorage.UploadedPart{PartNumber: partNumber, ETag: "etag", Size: size}, nil
		},
	}, 50*1024*1024, 5, time.Hour, time.Minute)
	service.sessionStore = &stubMultipartSessionStore{}

	initResult, err := service.InitiateMultipartUpload("U100", InitiateMultipartUploadInput{FileName: "f", FileSize: 4})
	if err != nil {
		t.Fatalf("initiate multipart upload: %v", err)
	}
	if err := service.UploadMultipartPart("U100", initResult.SessionID, 1, 4, "", bytes.NewReader([]byte("abc"))); !errors.Is(err, ErrMultipartPartInvalid) {
		t.Fatalf("expected short body rejection, got %v", err)
	}
}
