package http

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/JekYUlll/Dipole/internal/code"
	"github.com/JekYUlll/Dipole/internal/middleware"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/service"
)

type stubFileService struct {
	uploadFn   func(uploaderUUID string, header *multipart.FileHeader) (*model.UploadedFile, error)
	downloadFn func(currentUserUUID, fileUUID string) (*service.FileDownloadResult, error)
}

type stubFileLimiter struct {
	allowFileUploadFn func(userUUID string) (bool, time.Duration)
}

func (s *stubFileService) UploadMessageFile(uploaderUUID string, header *multipart.FileHeader) (*model.UploadedFile, error) {
	if s.uploadFn == nil {
		return nil, nil
	}
	return s.uploadFn(uploaderUUID, header)
}

func (s *stubFileService) CreateDownloadLink(currentUserUUID, fileUUID string) (*service.FileDownloadResult, error) {
	if s.downloadFn == nil {
		return nil, nil
	}
	return s.downloadFn(currentUserUUID, fileUUID)
}

func (s *stubFileLimiter) AllowFileUpload(userUUID string) (bool, time.Duration) {
	if s.allowFileUploadFn == nil {
		return true, 0
	}

	return s.allowFileUploadFn(userUUID)
}

func TestFileHandlerUploadSuccess(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := newFileHandler(&stubFileService{
		uploadFn: func(uploaderUUID string, header *multipart.FileHeader) (*model.UploadedFile, error) {
			if uploaderUUID != "U100" {
				t.Fatalf("unexpected uploader uuid: %s", uploaderUUID)
			}
			return &model.UploadedFile{
				UUID:        "F100",
				FileName:    header.Filename,
				FileSize:    header.Size,
				ContentType: header.Header.Get("Content-Type"),
				URL:         "http://127.0.0.1:9000/dipole-files/message-files/F100.txt",
			}, nil
		},
	}, 50*1024*1024)

	body, contentType := buildMultipartFileBody(t, "hello.txt", []byte("hello"))
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/files", body)
	context.Request.Header.Set("Content-Type", contentType)
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.Upload(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if int(response["code"].(float64)) != code.Success {
		t.Fatalf("expected business code %d, got %v", code.Success, response["code"])
	}
}

func TestFileHandlerUploadRateLimited(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := newFileHandler(&stubFileService{
		uploadFn: func(uploaderUUID string, header *multipart.FileHeader) (*model.UploadedFile, error) {
			t.Fatalf("upload service should not be called when rate limited")
			return nil, nil
		},
	}, 50*1024*1024).WithLimiter(&stubFileLimiter{
		allowFileUploadFn: func(userUUID string) (bool, time.Duration) {
			if userUUID != "U100" {
				t.Fatalf("unexpected uploader uuid: %s", userUUID)
			}
			return false, 20 * time.Second
		},
	})

	body, contentType := buildMultipartFileBody(t, "hello.txt", []byte("hello"))
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/files", body)
	context.Request.Header.Set("Content-Type", contentType)
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.Upload(context)

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d", recorder.Code)
	}

	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if int(response["code"].(float64)) != code.FileUploadRateLimited {
		t.Fatalf("expected business code %d, got %v", code.FileUploadRateLimited, response["code"])
	}
}

func TestFileHandlerDownloadSuccess(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := newFileHandler(&stubFileService{
		downloadFn: func(currentUserUUID, fileUUID string) (*service.FileDownloadResult, error) {
			if currentUserUUID != "U100" || fileUUID != "F100" {
				t.Fatalf("unexpected download args: %s %s", currentUserUUID, fileUUID)
			}
			return &service.FileDownloadResult{
				FileID:      "F100",
				FileName:    "hello.txt",
				FileSize:    5,
				ContentType: "text/plain",
				DownloadURL: "https://signed.example/download",
			}, nil
		},
	}, 50*1024*1024)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/files/F100/download", nil)
	context.Params = gin.Params{{Key: "file_id", Value: "F100"}}
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.Download(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func buildMultipartFileBody(t *testing.T, fileName string, content []byte) (*bytes.Buffer, string) {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write multipart content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	return &body, writer.FormDataContentType()
}
