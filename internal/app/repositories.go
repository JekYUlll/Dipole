// Package app owns process-level dependency composition shared by transports.
package app

import (
	"database/sql"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/application"
	mysqlData "github.com/JekYUlll/Dipole/internal/data/mysql"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	sqlcRepository "github.com/JekYUlll/Dipole/internal/data/mysql/repository"
)

// Repositories contains one repository instance for each application process.
type Repositories struct {
	Users         application.UserStore
	Messages      application.MessageStore
	Files         application.FileMetadataStore
	Conversations application.ConversationStore
	Contacts      application.ContactStore
	Groups        application.GroupStore
	Admin         application.AdminOverviewStore
	Sync          application.SyncStore
	AICallLogs    application.AICallLogStore
	Outbox        application.OutboxRelayStore
}

type MessageProcessRepositories struct {
	Messages application.MessageStore
	Outbox   application.OutboxRelayStore
}

func NewMessageProcessRepositories(db *sql.DB) (*MessageProcessRepositories, error) {
	if db == nil {
		return nil, fmt.Errorf("message repository composition requires database/sql connection")
	}
	mysqlStore, err := mysqlData.NewStore(db)
	if err != nil {
		return nil, fmt.Errorf("create message transaction store: %w", err)
	}
	messages, err := sqlcRepository.NewMessageRepository(mysqlStore)
	if err != nil {
		return nil, fmt.Errorf("create message repository: %w", err)
	}
	outbox, err := sqlcRepository.NewOutboxRepository(mysqlStore)
	if err != nil {
		return nil, fmt.Errorf("create message outbox repository: %w", err)
	}
	return &MessageProcessRepositories{Messages: messages, Outbox: outbox}, nil
}

func NewRepositories(db *sql.DB) (*Repositories, error) {
	if db == nil {
		return nil, fmt.Errorf("repository composition requires database/sql connection")
	}
	repos := &Repositories{}
	adapter, err := sqlcRepository.NewAICallLogRepository(generated.New(db))
	if err != nil {
		return nil, fmt.Errorf("create sqlc AI call log repository: %w", err)
	}
	repos.AICallLogs = adapter
	adminAdapter, err := sqlcRepository.NewAdminRepository(generated.New(db))
	if err != nil {
		return nil, fmt.Errorf("create sqlc admin repository: %w", err)
	}
	repos.Admin = adminAdapter
	fileAdapter, err := sqlcRepository.NewFileRepository(generated.New(db))
	if err != nil {
		return nil, fmt.Errorf("create sqlc file repository: %w", err)
	}
	repos.Files = fileAdapter
	userAdapter, err := sqlcRepository.NewUserRepository(generated.New(db))
	if err != nil {
		return nil, fmt.Errorf("create sqlc user repository: %w", err)
	}
	repos.Users = NewCachedUserStore(userAdapter)
	contactAdapter, err := sqlcRepository.NewContactRepository(generated.New(db))
	if err != nil {
		return nil, fmt.Errorf("create sqlc contact repository: %w", err)
	}
	repos.Contacts = NewCachedContactStore(contactAdapter)
	mysqlStore, err := mysqlData.NewStore(db)
	if err != nil {
		return nil, fmt.Errorf("create sqlc transaction store: %w", err)
	}
	messageAdapter, err := sqlcRepository.NewMessageRepository(mysqlStore)
	if err != nil {
		return nil, fmt.Errorf("create sqlc message repository: %w", err)
	}
	repos.Messages = messageAdapter
	syncAdapter, err := sqlcRepository.NewSyncRepository(generated.New(db))
	if err != nil {
		return nil, fmt.Errorf("create sqlc sync repository: %w", err)
	}
	repos.Sync = syncAdapter
	groupAdapter, err := sqlcRepository.NewGroupRepository(mysqlStore)
	if err != nil {
		return nil, fmt.Errorf("create sqlc group repository: %w", err)
	}
	repos.Groups = NewCachedGroupStore(groupAdapter)
	conversationAdapter, err := sqlcRepository.NewConversationRepository(generated.New(db))
	if err != nil {
		return nil, fmt.Errorf("create sqlc conversation repository: %w", err)
	}
	repos.Conversations = conversationAdapter
	outboxAdapter, err := sqlcRepository.NewOutboxRepository(mysqlStore)
	if err != nil {
		return nil, fmt.Errorf("create sqlc outbox relay repository: %w", err)
	}
	repos.Outbox = outboxAdapter
	return repos, nil
}
