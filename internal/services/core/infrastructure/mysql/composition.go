package coremysql

import (
	"database/sql"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/application"
	mysqlData "github.com/JekYUlll/Dipole/internal/data/mysql"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
)

// ProcessRepositories contains the repositories owned by the Core service.
// The composition is kept beside the Core SQLC implementations so the
// standalone Core runtime does not depend on the aggregate app package.
type ProcessRepositories struct {
	Users         application.UserStore
	Files         application.FileMetadataStore
	Conversations application.ConversationStore
	Contacts      application.ContactStore
	Groups        application.GroupStore
	Admin         application.AdminOverviewStore
}

func NewProcessRepositories(db *sql.DB) (*ProcessRepositories, error) {
	if db == nil {
		return nil, fmt.Errorf("core repository composition requires database/sql connection")
	}
	mysqlStore, err := mysqlData.NewStore(db)
	if err != nil {
		return nil, fmt.Errorf("create core transaction store: %w", err)
	}
	return newProcessRepositories(db, mysqlStore)
}

func newProcessRepositories(db *sql.DB, mysqlStore *mysqlData.Store) (*ProcessRepositories, error) {
	queries := generated.New(db)
	admin, err := NewAdminRepository(queries)
	if err != nil {
		return nil, fmt.Errorf("create sqlc admin repository: %w", err)
	}
	files, err := NewFileRepository(queries)
	if err != nil {
		return nil, fmt.Errorf("create sqlc file repository: %w", err)
	}
	users, err := NewUserRepository(queries)
	if err != nil {
		return nil, fmt.Errorf("create sqlc user repository: %w", err)
	}
	contacts, err := NewContactRepository(queries)
	if err != nil {
		return nil, fmt.Errorf("create sqlc contact repository: %w", err)
	}
	groups, err := NewGroupRepository(mysqlStore)
	if err != nil {
		return nil, fmt.Errorf("create sqlc group repository: %w", err)
	}
	conversations, err := NewConversationRepository(queries)
	if err != nil {
		return nil, fmt.Errorf("create sqlc conversation repository: %w", err)
	}
	return &ProcessRepositories{
		Admin: admin, Files: files,
		Users: NewCachedUserStore(users), Contacts: NewCachedContactStore(contacts),
		Groups: NewCachedGroupStore(groups), Conversations: conversations,
	}, nil
}
