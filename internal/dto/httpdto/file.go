package httpdto

import (
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/service"
	"time"
)

type UploadedFileResponse struct {
	FileID       string `json:"file_id"`
	FileName     string `json:"file_name"`
	FileSize     int64  `json:"file_size"`
	ContentType  string `json:"content_type"`
	DownloadPath string `json:"download_path"`
}

func ToUploadedFileResponse(file *model.UploadedFile) *UploadedFileResponse {
	if file == nil {
		return nil
	}

	return &UploadedFileResponse{
		FileID:       file.UUID,
		FileName:     file.FileName,
		FileSize:     file.FileSize,
		ContentType:  file.ContentType,
		DownloadPath: FileDownloadPath(file.UUID),
	}
}

type FileDownloadResponse struct {
	FileID      string     `json:"file_id"`
	FileName    string     `json:"file_name"`
	FileSize    int64      `json:"file_size"`
	ContentType string     `json:"content_type"`
	DownloadURL string     `json:"download_url"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

func ToFileDownloadResponse(result *service.FileDownloadResult) *FileDownloadResponse {
	if result == nil {
		return nil
	}

	return &FileDownloadResponse{
		FileID:      result.FileID,
		FileName:    result.FileName,
		FileSize:    result.FileSize,
		ContentType: result.ContentType,
		DownloadURL: result.DownloadURL,
		ExpiresAt:   result.ExpiresAt,
	}
}

func FileDownloadPath(fileUUID string) string {
	return "/api/v1/files/" + fileUUID + "/download"
}
