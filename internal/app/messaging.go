package app

import (
	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	platformHotGroup "github.com/JekYUlll/Dipole/internal/platform/hotgroup"
	platformStorage "github.com/JekYUlll/Dipole/internal/platform/storage"
	"github.com/JekYUlll/Dipole/internal/service"
	coreapplication "github.com/JekYUlll/Dipole/internal/services/core/application"
	messageapplication "github.com/JekYUlll/Dipole/internal/services/message/application"
	syncapplication "github.com/JekYUlll/Dipole/internal/services/sync/application"
)

type HotGroupObserver interface {
	ObserveMessage(groupUUID string, memberCount int) (platformHotGroup.Status, error)
	Status(groupUUID string, memberCount int) (platformHotGroup.Status, error)
}

type ConversationNotifier interface {
	NotifyDirectRead(receipt service.ConversationReadReceipt)
}

type MessagingDependencies struct {
	Events                     applicationPort.EventPublisher
	Core                       applicationPort.CoreCapability
	HotGroups                  HotGroupObserver
	Storage                    platformStorage.ObjectStorage
	ConversationNotifier       ConversationNotifier
	DuplicateHydrator          applicationPort.SyncMessageHydrator
	DuplicateHydrationObserver func(string)
}

type MessagingServices struct {
	Core          applicationPort.CoreCapability
	Files         *coreapplication.LocalFileApplication
	Messages      *messageapplication.LocalApplication
	Conversations *coreapplication.LocalConversationApplication
	Sync          applicationPort.SyncApplication
}

func NewMessagingServices(repos *Repositories, dependencies MessagingDependencies) *MessagingServices {
	files := coreapplication.NewFileApplication(repos.Files, repos.Messages, dependencies.Storage)
	core := dependencies.Core
	if core == nil {
		core = coreapplication.New(coreapplication.Dependencies{
			Users: repos.Users, Contacts: repos.Contacts, Groups: repos.Groups,
			Files: repos.Files, Conversations: repos.Conversations,
		})
	}

	messages := messageapplication.New(repos.Messages, core, messageapplication.Dependencies{
		Events:                     dependencies.Events,
		HotGroups:                  dependencies.HotGroups,
		DuplicateHydrator:          dependencies.DuplicateHydrator,
		DuplicateHydrationObserver: dependencies.DuplicateHydrationObserver,
	})
	return &MessagingServices{
		Core:     core,
		Files:    files,
		Messages: messages,
		Conversations: coreapplication.NewConversationApplication(
			repos.Conversations,
			repos.Users,
			repos.Groups,
			coreapplication.ConversationDependencies{
				Notifier: dependencies.ConversationNotifier,
				Events:   dependencies.Events,
			},
		),
		Sync: syncapplication.New(repos.Sync, core),
	}
}

func NewMessageApplication(messages applicationPort.MessageStore, core applicationPort.CoreCapability, dependencies MessagingDependencies) *messageapplication.LocalApplication {
	return messageapplication.New(messages, core, messageapplication.Dependencies{
		Events:                     dependencies.Events,
		HotGroups:                  dependencies.HotGroups,
		DuplicateHydrator:          dependencies.DuplicateHydrator,
		DuplicateHydrationObserver: dependencies.DuplicateHydrationObserver,
	})
}
