package model

import "time"

type UploadedFile struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UUID         string    `gorm:"size:24;uniqueIndex;not null" json:"uuid"`
	UploaderUUID string    `gorm:"column:uploader_uuid;size:24;index;not null" json:"uploader_uuid"`
	Bucket       string    `gorm:"size:128;not null" json:"bucket"`
	ObjectKey    string    `gorm:"column:object_key;size:255;uniqueIndex;not null" json:"object_key"`
	FileName     string    `gorm:"column:file_name;size:255;not null" json:"file_name"`
	FileSize     int64     `gorm:"column:file_size;not null" json:"file_size"`
	ContentType  string    `gorm:"column:content_type;size:255;not null" json:"content_type"`
	URL          string    `gorm:"size:512;not null" json:"url"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (UploadedFile) TableName() string {
	return "uploaded_files"
}
