package http

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/JekYUlll/Dipole/internal/code"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/dto/httpdto"
	"github.com/JekYUlll/Dipole/internal/middleware"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/service"
)

type fileService interface {
	UploadMessageFile(uploaderUUID string, header *multipart.FileHeader) (*model.UploadedFile, error)
	InitiateMultipartUpload(uploaderUUID string, input service.InitiateMultipartUploadInput) (*service.InitiateMultipartUploadResult, error)
	UploadMultipartPart(uploaderUUID, sessionID string, partNumber int, contentLength int64, body io.Reader) error
	CompleteMultipartUpload(uploaderUUID, sessionID string) (*model.UploadedFile, error)
	AbortMultipartUpload(uploaderUUID, sessionID string) error
	CreateDownloadLink(currentUserUUID, fileUUID string) (*service.FileDownloadResult, error)
	OpenContent(currentUserUUID, fileUUID string) (*service.FileContentResult, error)
}

type FileHandler struct {
	service        fileService
	maxUploadBytes int64
	limiter        fileRateLimiter
}

type fileRateLimiter interface {
	AllowFileUpload(userUUID string) (bool, time.Duration)
}

func NewFileHandler(service fileService) *FileHandler {
	return newFileHandler(service, config.StorageConfig().FileMaxSizeMB*1024*1024)
}

func newFileHandler(service fileService, maxUploadBytes int64) *FileHandler {
	return &FileHandler{
		service:        service,
		maxUploadBytes: maxUploadBytes,
	}
}

func (h *FileHandler) WithLimiter(limiter fileRateLimiter) *FileHandler {
	h.limiter = limiter
	return h
}

func (h *FileHandler) Upload(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	if h.limiter != nil {
		allowed, retryAfter := h.limiter.AllowFileUpload(currentUser.UUID)
		if !allowed {
			ErrorWithCode(
				c,
				http.StatusTooManyRequests,
				code.FileUploadRateLimited,
				formatRetryAfterMessage("file upload rate limit exceeded", retryAfter),
			)
			return
		}
	}

	if h.maxUploadBytes > 0 {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxUploadBytes)
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "request body too large") {
			ErrorWithCode(c, http.StatusBadRequest, code.FileTooLarge, "file is too large")
			return
		}
		ErrorWithCode(c, http.StatusBadRequest, code.FileMissing, "file is required")
		return
	}

	file, err := h.service.UploadMessageFile(currentUser.UUID, fileHeader)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrFileMissing):
			ErrorWithCode(c, http.StatusBadRequest, code.FileMissing, "file is required")
		case errors.Is(err, service.ErrFileTooLarge):
			ErrorWithCode(c, http.StatusBadRequest, code.FileTooLarge, "file is too large")
		case errors.Is(err, service.ErrFileStorageUnavailable):
			ErrorWithCode(c, http.StatusServiceUnavailable, code.FileStorageUnavailable, "file storage is unavailable")
		default:
			ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		}
		return
	}

	Success(c, httpdto.ToUploadedFileResponse(file))
}

func (h *FileHandler) Download(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	result, err := h.service.CreateDownloadLink(currentUser.UUID, c.Param("file_id"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrFileNotFound):
			ErrorWithCode(c, http.StatusNotFound, code.FileNotFound, "file not found")
		case errors.Is(err, service.ErrFilePermissionDenied):
			ErrorWithCode(c, http.StatusForbidden, code.FilePermissionDenied, "file permission denied")
		case errors.Is(err, service.ErrFileExpired):
			ErrorWithCode(c, http.StatusForbidden, code.FileExpired, "file is expired")
		case errors.Is(err, service.ErrFileStorageUnavailable):
			ErrorWithCode(c, http.StatusServiceUnavailable, code.FileStorageUnavailable, "file storage is unavailable")
		default:
			ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		}
		return
	}

	Success(c, httpdto.ToFileDownloadResponse(result))
}

func (h *FileHandler) Content(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	result, err := h.service.OpenContent(currentUser.UUID, c.Param("file_id"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrFileNotFound):
			ErrorWithCode(c, http.StatusNotFound, code.FileNotFound, "file not found")
		case errors.Is(err, service.ErrFilePermissionDenied):
			ErrorWithCode(c, http.StatusForbidden, code.FilePermissionDenied, "file permission denied")
		case errors.Is(err, service.ErrFileExpired):
			ErrorWithCode(c, http.StatusForbidden, code.FileExpired, "file is expired")
		case errors.Is(err, service.ErrFileStorageUnavailable):
			ErrorWithCode(c, http.StatusServiceUnavailable, code.FileStorageUnavailable, "file storage is unavailable")
		default:
			ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		}
		return
	}
	defer func() {
		if result.Content != nil {
			_ = result.Content.Close()
		}
		if result.Cleanup != nil {
			result.Cleanup()
		}
	}()

	if result.ContentType != "" {
		c.Header("Content-Type", result.ContentType)
	}
	if result.FileSize > 0 {
		c.Header("Content-Length", strconv.FormatInt(result.FileSize, 10))
	}
	if result.FileName != "" {
		c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", result.FileName))
	}
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, result.Content)
}

func (h *FileHandler) InitiateMultipart(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	if h.limiter != nil {
		allowed, retryAfter := h.limiter.AllowFileUpload(currentUser.UUID)
		if !allowed {
			ErrorWithCode(
				c,
				http.StatusTooManyRequests,
				code.FileUploadRateLimited,
				formatRetryAfterMessage("file upload rate limit exceeded", retryAfter),
			)
			return
		}
	}

	var req httpdto.FileMultipartInitiateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorWithCode(c, http.StatusBadRequest, code.FileMultipartSessionInvalid, "invalid multipart upload request")
		return
	}

	result, err := h.service.InitiateMultipartUpload(currentUser.UUID, service.InitiateMultipartUploadInput{
		FileName:    req.FileName,
		FileSize:    req.FileSize,
		ContentType: req.ContentType,
	})
	if err != nil {
		h.handleMultipartError(c, err)
		return
	}

	Success(c, httpdto.ToFileMultipartInitiateResponse(result))
}

func (h *FileHandler) UploadPart(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	sessionID := c.Param("session_id")
	partNumber, err := strconv.Atoi(c.Param("part_number"))
	if err != nil || partNumber <= 0 {
		ErrorWithCode(c, http.StatusBadRequest, code.FileMultipartPartInvalid, "invalid multipart part number")
		return
	}
	contentLength := c.Request.ContentLength
	if contentLength <= 0 {
		ErrorWithCode(c, http.StatusBadRequest, code.FileMultipartPartInvalid, "multipart part content length is required")
		return
	}

	if err := h.service.UploadMultipartPart(currentUser.UUID, sessionID, partNumber, contentLength, c.Request.Body); err != nil {
		h.handleMultipartError(c, err)
		return
	}

	Success(c, gin.H{"part_number": partNumber})
}

func (h *FileHandler) CompleteMultipart(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	file, err := h.service.CompleteMultipartUpload(currentUser.UUID, c.Param("session_id"))
	if err != nil {
		h.handleMultipartError(c, err)
		return
	}
	Success(c, httpdto.ToUploadedFileResponse(file))
}

func (h *FileHandler) AbortMultipart(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	if err := h.service.AbortMultipartUpload(currentUser.UUID, c.Param("session_id")); err != nil {
		h.handleMultipartError(c, err)
		return
	}
	Success(c, gin.H{"aborted": true})
}

func (h *FileHandler) handleMultipartError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrFileMissing):
		ErrorWithCode(c, http.StatusBadRequest, code.FileMissing, "file is required")
	case errors.Is(err, service.ErrFileTooLarge):
		ErrorWithCode(c, http.StatusBadRequest, code.FileTooLarge, "file is too large")
	case errors.Is(err, service.ErrFilePermissionDenied):
		ErrorWithCode(c, http.StatusForbidden, code.FilePermissionDenied, "file permission denied")
	case errors.Is(err, service.ErrMultipartSessionNotFound):
		ErrorWithCode(c, http.StatusNotFound, code.FileMultipartSessionNotFound, "multipart upload session not found")
	case errors.Is(err, service.ErrMultipartSessionInvalid):
		ErrorWithCode(c, http.StatusBadRequest, code.FileMultipartSessionInvalid, "multipart upload session is invalid")
	case errors.Is(err, service.ErrMultipartPartInvalid):
		ErrorWithCode(c, http.StatusBadRequest, code.FileMultipartPartInvalid, "multipart upload part is invalid")
	case errors.Is(err, service.ErrFileStorageUnavailable):
		ErrorWithCode(c, http.StatusServiceUnavailable, code.FileStorageUnavailable, "file storage is unavailable")
	default:
		// Preserve current response envelope even when the lower layer returns a
		// wrapped error from storage or Redis. This keeps large-file uploads
		// debuggable without leaking stack details to clients.
		ErrorWithCode(c, http.StatusInternalServerError, code.Internal, strings.TrimSpace(err.Error()))
	}
}
