package httpdto

import (
	"github.com/JekYUlll/Dipole/internal/model"
	corefile "github.com/JekYUlll/Dipole/internal/services/core/domain/file"
	"time"
)

type UploadedFileResponse struct {
	FileID       string `json:"file_id"`
	FileName     string `json:"file_name"`
	FileSize     int64  `json:"file_size"`
	ContentType  string `json:"content_type"`
	DownloadPath string `json:"download_path"`
	ContentPath  string `json:"content_path"`
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
		ContentPath:  FileContentPath(file.UUID),
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

type FileMultipartInitiateRequest struct {
	FileName    string `json:"file_name"`
	FileSize    int64  `json:"file_size"`
	ContentType string `json:"content_type"`
	FileSHA256  string `json:"file_sha256,omitempty"`
}

type FileMultipartInitiateResponse struct {
	SessionID  string `json:"session_id"`
	ChunkSize  int64  `json:"chunk_size"`
	TotalParts int    `json:"total_parts"`
}

type FileMultipartPartStatus struct {
	PartNumber int    `json:"part_number"`
	ETag       string `json:"etag"`
	Size       int64  `json:"size"`
}

type FileMultipartStatusResponse struct {
	SessionID     string                    `json:"session_id"`
	FileName      string                    `json:"file_name"`
	FileSize      int64                     `json:"file_size"`
	ChunkSize     int64                     `json:"chunk_size"`
	TotalParts    int                       `json:"total_parts"`
	UploadedParts []FileMultipartPartStatus `json:"uploaded_parts"`
}

type FileMultipartPresignRequest struct {
	PartNumbers []int `json:"part_numbers" binding:"required,min=1"`
}

type FileMultipartPresignPart struct {
	PartNumber int       `json:"part_number"`
	URL        string    `json:"url"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type FileMultipartPresignResponse struct {
	Parts []FileMultipartPresignPart `json:"parts"`
}

type FileMultipartPartRegisterRequest struct {
	ETag string `json:"etag" binding:"required"`
	Size int64  `json:"size" binding:"required,min=1"`
}

func ToFileMultipartInitiateResponse(result *corefile.InitiateMultipartUploadResult) *FileMultipartInitiateResponse {
	if result == nil {
		return nil
	}

	return &FileMultipartInitiateResponse{
		SessionID:  result.SessionID,
		ChunkSize:  result.ChunkSize,
		TotalParts: result.TotalParts,
	}
}

func ToFileMultipartStatusResponse(result *corefile.MultipartUploadStatus) *FileMultipartStatusResponse {
	if result == nil {
		return nil
	}
	parts := make([]FileMultipartPartStatus, 0, len(result.UploadedParts))
	for _, part := range result.UploadedParts {
		parts = append(parts, FileMultipartPartStatus{
			PartNumber: part.PartNumber,
			ETag:       part.ETag,
			Size:       part.Size,
		})
	}
	return &FileMultipartStatusResponse{
		SessionID:     result.SessionID,
		FileName:      result.FileName,
		FileSize:      result.FileSize,
		ChunkSize:     result.ChunkSize,
		TotalParts:    result.TotalParts,
		UploadedParts: parts,
	}
}

func ToFileMultipartPresignResponse(result []corefile.MultipartPartUploadURL) *FileMultipartPresignResponse {
	parts := make([]FileMultipartPresignPart, 0, len(result))
	for _, part := range result {
		parts = append(parts, FileMultipartPresignPart{PartNumber: part.PartNumber, URL: part.URL, ExpiresAt: part.ExpiresAt})
	}
	return &FileMultipartPresignResponse{Parts: parts}
}

func ToFileDownloadResponse(result *corefile.FileDownloadResult) *FileDownloadResponse {
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

func FileContentPath(fileUUID string) string {
	return "/api/v1/files/" + fileUUID + "/content"
}
