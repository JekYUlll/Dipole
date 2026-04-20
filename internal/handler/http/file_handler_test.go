package http

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/JekYUlll/Dipole/internal/code"
	"github.com/JekYUlll/Dipole/internal/middleware"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/service"
)

type stubFileService struct {
	uploadFn            func(uploaderUUID string, header *multipart.FileHeader) (*model.UploadedFile, error)
	initiateMultipartFn func(uploaderUUID string, input service.InitiateMultipartUploadInput) (*service.InitiateMultipartUploadResult, error)
	uploadPartFn        func(uploaderUUID, sessionID string, partNumber int, contentLength int64, body io.Reader) error
	completeMultipartFn func(uploaderUUID, sessionID string) (*model.UploadedFile, error)
	abortMultipartFn    func(uploaderUUID, sessionID string) error
	downloadFn          func(currentUserUUID, fileUUID string) (*service.FileDownloadResult, error)
	openContentFn       func(currentUserUUID, fileUUID string) (*service.FileContentResult, error)
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

func (s *stubFileService) OpenContent(currentUserUUID, fileUUID string) (*service.FileContentResult, error) {
	if s.openContentFn == nil {
		return nil, nil
	}
	return s.openContentFn(currentUserUUID, fileUUID)
}

func (s *stubFileService) InitiateMultipartUpload(uploaderUUID string, input service.InitiateMultipartUploadInput) (*service.InitiateMultipartUploadResult, error) {
	if s.initiateMultipartFn == nil {
		return nil, nil
	}
	return s.initiateMultipartFn(uploaderUUID, input)
}

func (s *stubFileService) UploadMultipartPart(uploaderUUID, sessionID string, partNumber int, contentLength int64, body io.Reader) error {
	if s.uploadPartFn == nil {
		return nil
	}
	return s.uploadPartFn(uploaderUUID, sessionID, partNumber, contentLength, body)
}

func (s *stubFileService) CompleteMultipartUpload(uploaderUUID, sessionID string) (*model.UploadedFile, error) {
	if s.completeMultipartFn == nil {
		return nil, nil
	}
	return s.completeMultipartFn(uploaderUUID, sessionID)
}

func (s *stubFileService) AbortMultipartUpload(uploaderUUID, sessionID string) error {
	if s.abortMultipartFn == nil {
		return nil
	}
	return s.abortMultipartFn(uploaderUUID, sessionID)
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

func TestFileHandlerContentSuccess(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := newFileHandler(&stubFileService{
		openContentFn: func(currentUserUUID, fileUUID string) (*service.FileContentResult, error) {
			if currentUserUUID != "U100" || fileUUID != "F100" {
				t.Fatalf("unexpected open content args: %s %s", currentUserUUID, fileUUID)
			}
			return &service.FileContentResult{
				FileID:      "F100",
				FileName:    "image.png",
				ContentType: "image/png",
				FileSize:    5,
				Content:     io.NopCloser(strings.NewReader("hello")),
			}, nil
		},
	}, 50*1024*1024)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/files/F100/content", nil)
	context.Params = gin.Params{{Key: "file_id", Value: "F100"}}
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.Content(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("expected content type image/png, got %s", got)
	}
	if got := recorder.Body.String(); got != "hello" {
		t.Fatalf("expected body hello, got %s", got)
	}
}

func TestFileHandlerInitiateMultipartSuccess(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := newFileHandler(&stubFileService{
		initiateMultipartFn: func(uploaderUUID string, input service.InitiateMultipartUploadInput) (*service.InitiateMultipartUploadResult, error) {
			if uploaderUUID != "U100" || input.FileName != "big.bin" || input.FileSize != 12 {
				t.Fatalf("unexpected multipart init args: %s %+v", uploaderUUID, input)
			}
			return &service.InitiateMultipartUploadResult{
				SessionID:  "MU100",
				ChunkSize:  5,
				TotalParts: 3,
			}, nil
		},
	}, 50*1024*1024)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/files/uploads/initiate", bytes.NewBufferString(`{"file_name":"big.bin","file_size":12,"content_type":"application/octet-stream"}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.InitiateMultipart(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestFileHandlerUploadPartSuccess(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := newFileHandler(&stubFileService{
		uploadPartFn: func(uploaderUUID, sessionID string, partNumber int, contentLength int64, body io.Reader) error {
			if uploaderUUID != "U100" || sessionID != "MU100" || partNumber != 1 || contentLength != 5 {
				t.Fatalf("unexpected multipart part args: %s %s %d %d", uploaderUUID, sessionID, partNumber, contentLength)
			}
			data, err := io.ReadAll(body)
			if err != nil {
				t.Fatalf("read multipart body: %v", err)
			}
			if string(data) != "hello" {
				t.Fatalf("unexpected multipart body: %s", string(data))
			}
			return nil
		},
	}, 50*1024*1024)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/files/uploads/MU100/parts/1", bytes.NewBufferString("hello"))
	req.ContentLength = 5
	context.Request = req
	context.Params = gin.Params{
		{Key: "session_id", Value: "MU100"},
		{Key: "part_number", Value: "1"},
	}
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.UploadPart(context)

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
