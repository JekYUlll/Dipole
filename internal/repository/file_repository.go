package repository

import (
	"fmt"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/store"
	"gorm.io/gorm"
)

var _ application.FileMetadataStore = (*FileRepository)(nil)

type FileRepository struct {
	db *gorm.DB
}

func NewFileRepository() *FileRepository {
	return &FileRepository{}
}

func NewFileRepositoryWithDB(db *gorm.DB) *FileRepository {
	return &FileRepository{db: db}
}

func (r *FileRepository) Create(file *model.UploadedFile) error {
	if err := r.database().Create(file).Error; err != nil {
		return fmt.Errorf("create uploaded file: %w", err)
	}

	return nil
}

func (r *FileRepository) GetByUUID(uuid string) (*model.UploadedFile, error) {
	var file model.UploadedFile
	if err := r.database().Where("uuid = ?", uuid).First(&file).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get uploaded file by uuid: %w", err)
	}

	return &file, nil
}

func (r *FileRepository) database() *gorm.DB {
	if r != nil && r.db != nil {
		return r.db
	}
	return store.DB
}
