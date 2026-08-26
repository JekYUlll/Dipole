// Package app owns process-level dependency composition shared by transports.
package app

import (
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/repository"
)

// Repositories contains one repository instance for each application process.
type Repositories struct {
	Users         *repository.UserRepository
	Messages      *repository.MessageRepository
	Files         *repository.FileRepository
	Conversations *repository.ConversationRepository
	Contacts      *repository.ContactRepository
	Groups        *repository.GroupRepository
	Admin         *repository.AdminRepository
	Sync          *repository.SyncRepository
	AICallLogs    application.AICallLogStore
	Outbox        *repository.OutboxRepository
}

func NewRepositories() *Repositories {
	return &Repositories{
		Users:         repository.NewUserRepository(),
		Messages:      repository.NewMessageRepository(),
		Files:         repository.NewFileRepository(),
		Conversations: repository.NewConversationRepository(),
		Contacts:      repository.NewContactRepository(),
		Groups:        repository.NewGroupRepository(),
		Admin:         repository.NewAdminRepository(),
		Sync:          repository.NewSyncRepository(),
		AICallLogs:    repository.NewAICallLogRepository(),
		Outbox:        repository.NewOutboxRepository(),
	}
}
