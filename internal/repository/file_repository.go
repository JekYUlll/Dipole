package repository

import (
	"fmt"

	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/store"
	"gorm.io/gorm"
)

type FileRepository struct{}

func NewFileRepository() *FileRepository {
	return &FileRepository{}
}

func (r *FileRepository) Create(file *model.UploadedFile) error {
	if err := store.DB.Create(file).Error; err != nil {
		return fmt.Errorf("create uploaded file: %w", err)
	}

	return nil
}

func (r *FileRepository) GetByUUID(uuid string) (*model.UploadedFile, error) {
	var file model.UploadedFile
	if err := store.DB.Where("uuid = ?", uuid).First(&file).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get uploaded file by uuid: %w", err)
	}

	return &file, nil
}
