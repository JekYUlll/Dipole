package coreapplication

import (
	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	platformStorage "github.com/JekYUlll/Dipole/internal/platform/storage"
	"github.com/JekYUlll/Dipole/internal/service"
)

// LocalFileApplication keeps file use cases behind the Core service boundary.
type LocalFileApplication struct {
	*service.FileService
}

func NewFileApplication(
	repository applicationPort.FileMetadataStore,
	messages applicationPort.MessageStore,
	storage platformStorage.ObjectStorage,
) *LocalFileApplication {
	return &LocalFileApplication{
		FileService: service.NewFileService(repository, messages, storage),
	}
}
