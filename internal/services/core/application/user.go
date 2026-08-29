package coreapplication

import (
	"time"

	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	platformStorage "github.com/JekYUlll/Dipole/internal/platform/storage"
	"github.com/JekYUlll/Dipole/internal/service"
)

type UserDependencies struct {
	Files          applicationPort.FileMetadataStore
	Storage        platformStorage.ObjectStorage
	AvatarMaxBytes int64
	AvatarURLTTL   time.Duration
}

// LocalUserApplication keeps Core user use cases behind the Core service boundary.
type LocalUserApplication struct {
	*service.UserService
}

func NewUserApplication(repository applicationPort.UserStore, dependencies UserDependencies) *LocalUserApplication {
	userService := service.NewUserService(repository).WithAvatarStorage(
		dependencies.Files,
		dependencies.Storage,
		dependencies.AvatarMaxBytes,
		dependencies.AvatarURLTTL,
	)
	return &LocalUserApplication{UserService: userService}
}
