// Package app owns process-level dependency composition shared by transports.
package app

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	sqlcRepository "github.com/JekYUlll/Dipole/internal/data/mysql/repository"
	"github.com/JekYUlll/Dipole/internal/repository"
)

const (
	MySQLAdapterGORM = "gorm"
	MySQLAdapterSQLC = "sqlc"
)

type RepositoryOptions struct {
	MySQLAdapter string
	SQLDB        *sql.DB
}

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
	repos, err := NewRepositoriesWithOptions(RepositoryOptions{MySQLAdapter: MySQLAdapterGORM})
	if err != nil {
		panic(err)
	}
	return repos
}

func NewRepositoriesWithOptions(options RepositoryOptions) (*Repositories, error) {
	repos := &Repositories{
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

	switch strings.ToLower(strings.TrimSpace(options.MySQLAdapter)) {
	case "", MySQLAdapterGORM:
	case MySQLAdapterSQLC:
		if options.SQLDB == nil {
			return nil, fmt.Errorf("sqlc adapter requires database/sql connection")
		}
		adapter, err := sqlcRepository.NewAICallLogRepository(generated.New(options.SQLDB))
		if err != nil {
			return nil, fmt.Errorf("create sqlc AI call log repository: %w", err)
		}
		repos.AICallLogs = adapter
	default:
		return nil, fmt.Errorf("unsupported data.mysql_adapter %q", options.MySQLAdapter)
	}
	return repos, nil
}
