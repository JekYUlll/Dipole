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
	if repos == nil {
		repos = &Repositories{}
	}
	coreRepos := repos.CoreProcess
	if coreRepos == nil {
		coreRepos = &CoreProcessRepositories{
			Users: repos.Users, Files: repos.Files, Conversations: repos.Conversations,
			Contacts: repos.Contacts, Groups: repos.Groups, Admin: repos.Admin,
		}
	}
	messageRepos := repos.MessageProcess
	if messageRepos == nil {
		messageRepos = &MessageProcessRepositories{Messages: repos.Messages, Outbox: repos.Outbox}
	}
	syncRepos := repos.SyncProcess
	if syncRepos == nil {
		syncRepos = &SyncProcessRepositories{Sync: repos.Sync}
	}
	return NewMessagingServicesFromProcesses(coreRepos, messageRepos, syncRepos, dependencies)
}

// NewMessagingServicesFromProcesses composes the messaging facade from
// service-owned repository groups. The aggregate constructor above remains a
// compatibility entry point for embedded callers.
func NewMessagingServicesFromProcesses(
	coreRepos *CoreProcessRepositories,
	messageRepos *MessageProcessRepositories,
	syncRepos *SyncProcessRepositories,
	dependencies MessagingDependencies,
) *MessagingServices {
	if coreRepos == nil {
		coreRepos = &CoreProcessRepositories{}
	}
	if messageRepos == nil {
		messageRepos = &MessageProcessRepositories{}
	}
	if syncRepos == nil {
		syncRepos = &SyncProcessRepositories{}
	}
	files := coreapplication.NewFileApplication(coreRepos.Files, messageRepos.Messages, dependencies.Storage)
	core := dependencies.Core
	if core == nil {
		core = coreapplication.New(coreapplication.Dependencies{
			Users: coreRepos.Users, Contacts: coreRepos.Contacts, Groups: coreRepos.Groups,
			Files: coreRepos.Files, Conversations: coreRepos.Conversations,
		})
	}

	messages := messageapplication.New(messageRepos.Messages, core, messageapplication.Dependencies{
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
			coreRepos.Conversations,
			coreRepos.Users,
			coreRepos.Groups,
			coreapplication.ConversationDependencies{
				Notifier: dependencies.ConversationNotifier,
				Events:   dependencies.Events,
			},
		),
		Sync: syncapplication.New(syncRepos.Sync, core),
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
