// Package app owns process-level dependency composition shared by transports.
package app

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
	mysqlData "github.com/JekYUlll/Dipole/internal/data/mysql"
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
	Users         application.UserStore
	Messages      *repository.MessageRepository
	Files         application.FileMetadataStore
	Conversations application.ConversationStore
	Contacts      application.ContactStore
	Groups        application.GroupStore
	Admin         application.AdminOverviewStore
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
		Users:         NewCachedUserStore(repository.NewUserRepository()),
		Messages:      repository.NewMessageRepository(),
		Files:         repository.NewFileRepository(),
		Conversations: repository.NewConversationRepository(),
		Contacts:      NewCachedContactStore(repository.NewContactRepository()),
		Groups:        NewCachedGroupStore(repository.NewGroupRepository()),
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
		adminAdapter, err := sqlcRepository.NewAdminRepository(generated.New(options.SQLDB))
		if err != nil {
			return nil, fmt.Errorf("create sqlc admin repository: %w", err)
		}
		repos.Admin = adminAdapter
		fileAdapter, err := sqlcRepository.NewFileRepository(generated.New(options.SQLDB))
		if err != nil {
			return nil, fmt.Errorf("create sqlc file repository: %w", err)
		}
		repos.Files = fileAdapter
		userAdapter, err := sqlcRepository.NewUserRepository(generated.New(options.SQLDB))
		if err != nil {
			return nil, fmt.Errorf("create sqlc user repository: %w", err)
		}
		repos.Users = NewCachedUserStore(userAdapter)
		contactAdapter, err := sqlcRepository.NewContactRepository(generated.New(options.SQLDB))
		if err != nil {
			return nil, fmt.Errorf("create sqlc contact repository: %w", err)
		}
		repos.Contacts = NewCachedContactStore(contactAdapter)
		mysqlStore, err := mysqlData.NewStore(options.SQLDB)
		if err != nil {
			return nil, fmt.Errorf("create sqlc transaction store: %w", err)
		}
		groupAdapter, err := sqlcRepository.NewGroupRepository(mysqlStore)
		if err != nil {
			return nil, fmt.Errorf("create sqlc group repository: %w", err)
		}
		repos.Groups = NewCachedGroupStore(groupAdapter)
		conversationAdapter, err := sqlcRepository.NewConversationRepository(generated.New(options.SQLDB))
		if err != nil {
			return nil, fmt.Errorf("create sqlc conversation repository: %w", err)
		}
		repos.Conversations = conversationAdapter
	default:
		return nil, fmt.Errorf("unsupported data.mysql_adapter %q", options.MySQLAdapter)
	}
	return repos, nil
}
