package bootstrap

import (
	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	platformHotGroup "github.com/JekYUlll/Dipole/internal/platform/hotgroup"
	platformStorage "github.com/JekYUlll/Dipole/internal/platform/storage"
	coreapplication "github.com/JekYUlll/Dipole/internal/services/core/application"
	coremysql "github.com/JekYUlll/Dipole/internal/services/core/infrastructure/mysql"
	coreserver "github.com/JekYUlll/Dipole/internal/services/core/server"
	messageapplication "github.com/JekYUlll/Dipole/internal/services/message/application"
	syncapplication "github.com/JekYUlll/Dipole/internal/services/sync/application"
)

// newCoreMessagingServices builds the local application ports needed by the
// standalone Core HTTP surface without importing embedded composition.
func newCoreMessagingServices(
	coreRepos *coremysql.ProcessRepositories,
	events applicationPort.EventPublisher,
	hotGroups interface {
		ObserveMessage(string, int) (platformHotGroup.Status, error)
		Status(string, int) (platformHotGroup.Status, error)
	},
	storage platformStorage.ObjectStorage,
) *coreserver.MessagingServices {
	if coreRepos == nil {
		coreRepos = &coremysql.ProcessRepositories{}
	}
	core := coreapplication.New(coreapplication.Dependencies{
		Users: coreRepos.Users, Contacts: coreRepos.Contacts, Groups: coreRepos.Groups,
		Files: coreRepos.Files, Conversations: coreRepos.Conversations,
	})
	return &coreserver.MessagingServices{
		Core:  core,
		Files: coreapplication.NewFileApplication(coreRepos.Files, nil, storage),
		Messages: messageapplication.New(nil, core, messageapplication.Dependencies{
			Events: events, HotGroups: hotGroups,
		}),
		Conversations: coreapplication.NewConversationApplication(
			coreRepos.Conversations, coreRepos.Users, coreRepos.Groups,
			coreapplication.ConversationDependencies{Events: events},
		),
		Sync: syncapplication.New(nil, core),
	}
}
