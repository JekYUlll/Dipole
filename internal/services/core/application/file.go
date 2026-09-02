package coreapplication

import (
	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	platformStorage "github.com/JekYUlll/Dipole/internal/platform/storage"
	corefile "github.com/JekYUlll/Dipole/internal/services/core/domain/file"
)

// LocalFileApplication keeps file use cases behind the Core service boundary.
type LocalFileApplication struct {
	*corefile.FileService
}

func NewFileApplication(
	repository applicationPort.FileMetadataStore,
	messages applicationPort.MessageStore,
	storage platformStorage.ObjectStorage,
) *LocalFileApplication {
	return &LocalFileApplication{
		FileService: corefile.NewFileService(repository, messages, storage),
	}
}
