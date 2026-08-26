package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/data/mysql/mapper"
	"github.com/JekYUlll/Dipole/internal/model"
)

var _ application.FileMetadataStore = (*FileRepository)(nil)

type FileRepository struct {
	queries generated.Querier
}

func NewFileRepository(queries generated.Querier) (*FileRepository, error) {
	if queries == nil {
		return nil, errors.New("file queries are required")
	}
	return &FileRepository{queries: queries}, nil
}

func (r *FileRepository) Create(file *model.UploadedFile) error {
	if file == nil {
		return errors.New("create uploaded file with sqlc: file is required")
	}
	if _, err := r.queries.CreateUploadedFile(context.Background(), mapper.UploadedFileCreateParams(file)); err != nil {
		return fmt.Errorf("create uploaded file with sqlc: %w", err)
	}
	stored, err := r.GetByUUID(file.UUID)
	if err != nil {
		return err
	}
	if stored == nil {
		return errors.New("created uploaded file was not found")
	}
	*file = *stored
	return nil
}

func (r *FileRepository) GetByUUID(uuid string) (*model.UploadedFile, error) {
	row, err := r.queries.GetUploadedFileByUUID(context.Background(), uuid)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get uploaded file by UUID with sqlc: %w", err)
	}
	return mapper.UploadedFile(row), nil
}
