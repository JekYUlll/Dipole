package model

import "time"

type UploadedFile struct {
	ID           uint      `json:"id"`
	UUID         string    `json:"uuid"`
	UploaderUUID string    `json:"uploader_uuid"`
	Bucket       string    `json:"bucket"`
	ObjectKey    string    `json:"object_key"`
	FileName     string    `json:"file_name"`
	FileSize     int64     `json:"file_size"`
	ContentType  string    `json:"content_type"`
	URL          string    `json:"url"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
