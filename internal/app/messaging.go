package app

import (
	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	platformHotGroup "github.com/JekYUlll/Dipole/internal/platform/hotgroup"
	platformStorage "github.com/JekYUlll/Dipole/internal/platform/storage"
	"github.com/JekYUlll/Dipole/internal/service"
)

type HotGroupObserver interface {
	ObserveMessage(groupUUID string, memberCount int) (platformHotGroup.Status, error)
	Status(groupUUID string, memberCount int) (platformHotGroup.Status, error)
}

type ConversationNotifier interface {
	NotifyDirectRead(receipt service.ConversationReadReceipt)
}

type MessagingDependencies struct {
	Events               applicationPort.EventPublisher
	Core                 applicationPort.CoreCapability
	HotGroups            HotGroupObserver
	Storage              platformStorage.ObjectStorage
	ConversationNotifier ConversationNotifier
}

type MessagingServices struct {
	Files         *service.FileService
	Messages      *LocalMessageApplication
	Conversations *service.ConversationService
	Sync          *LocalSyncApplication
}

type LocalMessageApplication struct {
	*service.MessageService
}

type LocalSyncApplication struct {
	*service.SyncService
}

func NewMessagingServices(repos *Repositories, dependencies MessagingDependencies) *MessagingServices {
	files := service.NewFileService(repos.Files, repos.Messages, dependencies.Storage)
	core := dependencies.Core
	if core == nil {
		core = NewLocalCoreCapability(repos)
	}

	return &MessagingServices{
		Files: files,
		Messages: &LocalMessageApplication{MessageService: service.NewMessageServiceWithCore(
			repos.Messages,
			core,
			nil,
			dependencies.Events,
			dependencies.HotGroups,
		)},
		Conversations: service.NewConversationService(
			repos.Conversations,
			repos.Users,
			repos.Groups,
			dependencies.ConversationNotifier,
			dependencies.Events,
		),
		Sync: &LocalSyncApplication{SyncService: service.NewSyncService(repos.Sync)},
	}
}

func NewMessageApplication(messages applicationPort.MessageStore, core applicationPort.CoreCapability, dependencies MessagingDependencies) *LocalMessageApplication {
	return &LocalMessageApplication{MessageService: service.NewMessageServiceWithCore(
		messages,
		core,
		nil,
		dependencies.Events,
		dependencies.HotGroups,
	)}
}
