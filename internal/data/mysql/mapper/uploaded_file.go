package mapper

import (
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/model"
)

func UploadedFileCreateParams(file *model.UploadedFile) generated.CreateUploadedFileParams {
	if file == nil {
		return generated.CreateUploadedFileParams{}
	}
	return generated.CreateUploadedFileParams{
		Uuid:         file.UUID,
		UploaderUuid: file.UploaderUUID,
		Bucket:       file.Bucket,
		ObjectKey:    file.ObjectKey,
		FileName:     file.FileName,
		FileSize:     file.FileSize,
		ContentType:  file.ContentType,
		Url:          file.URL,
	}
}

func UploadedFile(row generated.UploadedFile) *model.UploadedFile {
	return &model.UploadedFile{
		ID:           uint(row.ID),
		UUID:         row.Uuid,
		UploaderUUID: row.UploaderUuid,
		Bucket:       row.Bucket,
		ObjectKey:    row.ObjectKey,
		FileName:     row.FileName,
		FileSize:     row.FileSize,
		ContentType:  row.ContentType,
		URL:          row.Url,
		CreatedAt:    row.CreatedAt.Time,
		UpdatedAt:    row.UpdatedAt.Time,
	}
}
