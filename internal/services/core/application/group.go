package coreapplication

import (
	"time"

	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	platformHotGroup "github.com/JekYUlll/Dipole/internal/platform/hotgroup"
	platformStorage "github.com/JekYUlll/Dipole/internal/platform/storage"
	coregroup "github.com/JekYUlll/Dipole/internal/services/core/domain/group"
)

type GroupHotGroupReader interface {
	Status(groupUUID string, memberCount int) (platformHotGroup.Status, error)
}

type GroupDependencies struct {
	Events          applicationPort.EventPublisher
	HotGroups       GroupHotGroupReader
	Files           applicationPort.FileMetadataStore
	Storage         platformStorage.ObjectStorage
	AvatarMaxBytes  int64
	AvatarURLTTL    time.Duration
	SystemMessenger interface {
		SendSystemGroupMessage(groupUUID, content string) error
	}
}

// LocalGroupApplication keeps Core group use cases behind the service boundary.
type LocalGroupApplication struct {
	*coregroup.GroupService
}

func NewGroupApplication(
	repository applicationPort.GroupStore,
	users applicationPort.UserStore,
	dependencies GroupDependencies,
) *LocalGroupApplication {
	groupService := coregroup.NewGroupService(repository, users, dependencies.Events, dependencies.HotGroups).
		WithAvatarStorage(dependencies.Files, dependencies.Storage, dependencies.AvatarMaxBytes, dependencies.AvatarURLTTL).
		WithSystemMessenger(dependencies.SystemMessenger)
	return &LocalGroupApplication{GroupService: groupService}
}
