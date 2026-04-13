package http

import (
	"errors"
	"mime/multipart"
	"net/http"
	"strings"

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
}

type FileHandler struct {
	service        fileService
	maxUploadBytes int64
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

func (h *FileHandler) Upload(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
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
