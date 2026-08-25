package app

import (
	"context"

	platformHotGroup "github.com/JekYUlll/Dipole/internal/platform/hotgroup"
	platformStorage "github.com/JekYUlll/Dipole/internal/platform/storage"
	"github.com/JekYUlll/Dipole/internal/service"
)

type EventPublisher interface {
	PublishJSON(ctx context.Context, topic string, key string, payload any, headers map[string]string) error
	PublishEvent(ctx context.Context, topic string, key string, eventType string, payload any, headers map[string]string) error
}

type HotGroupObserver interface {
	ObserveMessage(groupUUID string, memberCount int) (platformHotGroup.Status, error)
	Status(groupUUID string, memberCount int) (platformHotGroup.Status, error)
}

type ConversationNotifier interface {
	NotifyDirectRead(receipt service.ConversationReadReceipt)
}

type MessagingDependencies struct {
	Events               EventPublisher
	HotGroups            HotGroupObserver
	Storage              platformStorage.ObjectStorage
	ConversationNotifier ConversationNotifier
}

type MessagingServices struct {
	Files         *service.FileService
	Messages      *service.MessageService
	Conversations *service.ConversationService
	Sync          *service.SyncService
}

func NewMessagingServices(repos *Repositories, dependencies MessagingDependencies) *MessagingServices {
	files := service.NewFileService(repos.Files, repos.Messages, dependencies.Storage)

	return &MessagingServices{
		Files: files,
		Messages: service.NewMessageService(
			repos.Messages,
			repos.Users,
			repos.Contacts,
			repos.Groups,
			files,
			dependencies.Events,
			dependencies.HotGroups,
		),
		Conversations: service.NewConversationService(
			repos.Conversations,
			repos.Users,
			repos.Groups,
			dependencies.ConversationNotifier,
			dependencies.Events,
		),
		Sync: service.NewSyncService(repos.Sync),
	}
}
