package app

import (
	"database/sql"

	"github.com/JekYUlll/Dipole/internal/application"
	embedded "github.com/JekYUlll/Dipole/internal/bootstrap/embedded"
	messageapplication "github.com/JekYUlll/Dipole/internal/services/message/application"
)

// The aggregate app package remains a compatibility facade for embedded callers.
// New production composition should use internal/bootstrap/embedded directly.
type Repositories = embedded.Repositories
type MessageProcessRepositories = embedded.MessageProcessRepositories
type MessagingDependencies = embedded.MessagingDependencies
type MessagingServices = embedded.MessagingServices

func NewRepositories(db *sql.DB) (*Repositories, error) { return embedded.NewRepositories(db) }

func NewMessageProcessRepositories(db *sql.DB) (*MessageProcessRepositories, error) {
	return embedded.NewMessageProcessRepositories(db)
}

func NewMessageProcessRepositoriesWithInboxWrites(db *sql.DB, enabled bool) (*MessageProcessRepositories, error) {
	return embedded.NewMessageProcessRepositoriesWithInboxWrites(db, enabled)
}

func NewMessagingServices(repos *Repositories, dependencies MessagingDependencies) *MessagingServices {
	return embedded.NewMessagingServices(repos, dependencies)
}

func NewMessagingServicesFromProcesses(core *CoreProcessRepositories, message *MessageProcessRepositories, sync *SyncProcessRepositories, dependencies MessagingDependencies) *MessagingServices {
	return embedded.NewMessagingServicesFromProcesses(core, message, sync, dependencies)
}

func NewMessageApplication(messages application.MessageStore, core application.CoreCapability, dependencies MessagingDependencies) *messageapplication.LocalApplication {
	return embedded.NewMessageApplication(messages, core, dependencies)
}
