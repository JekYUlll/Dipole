package service

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/textproto"
	"testing"

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
	uploadFn func(ctx context.Context, file multipart.File, header *multipart.FileHeader) (*platformStorage.UploadedObject, error)
}

func (u *stubUploader) UploadMessageFile(ctx context.Context, file multipart.File, header *multipart.FileHeader) (*platformStorage.UploadedObject, error) {
	if u.uploadFn == nil {
		return nil, errors.New("unexpected upload call")
	}
	return u.uploadFn(ctx, file, header)
}

func TestFileServiceUploadMessageFileSuccess(t *testing.T) {
	t.Parallel()

	repo := &stubFileRepository{}
	service := newFileService(repo, &stubUploader{
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
	}, 50*1024*1024)

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

	service := newFileService(&stubFileRepository{}, &stubUploader{}, 4)
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
	}, nil, 0)

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
