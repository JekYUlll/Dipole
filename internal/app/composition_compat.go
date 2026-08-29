package app

import (
	"database/sql"

	embedded "github.com/JekYUlll/Dipole/internal/bootstrap/embedded"
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

func NewMessagingServices(repos *Repositories, dependencies MessagingDependencies) *MessagingServices {
	return embedded.NewMessagingServices(repos, dependencies)
}

func NewMessagingServicesFromProcesses(core *CoreProcessRepositories, message *MessageProcessRepositories, sync *SyncProcessRepositories, dependencies MessagingDependencies) *MessagingServices {
	return embedded.NewMessagingServicesFromProcesses(core, message, sync, dependencies)
}
