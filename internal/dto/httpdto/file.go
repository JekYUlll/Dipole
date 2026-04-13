package httpdto

import "github.com/JekYUlll/Dipole/internal/model"

type UploadedFileResponse struct {
	FileID      string `json:"file_id"`
	FileName    string `json:"file_name"`
	FileSize    int64  `json:"file_size"`
	ContentType string `json:"content_type"`
	URL         string `json:"url"`
}

func ToUploadedFileResponse(file *model.UploadedFile) *UploadedFileResponse {
	if file == nil {
		return nil
	}

	return &UploadedFileResponse{
		FileID:      file.UUID,
		FileName:    file.FileName,
		FileSize:    file.FileSize,
		ContentType: file.ContentType,
		URL:         file.URL,
	}
}
