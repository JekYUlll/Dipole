// Package embedded owns the local composition used by the compatibility server.
package embedded

import (
	"database/sql"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
	agentmysql "github.com/JekYUlll/Dipole/internal/services/agent/infrastructure/mysql"
	coremysql "github.com/JekYUlll/Dipole/internal/services/core/infrastructure/mysql"
	messagemysql "github.com/JekYUlll/Dipole/internal/services/message/infrastructure/mysql"
	searchmysql "github.com/JekYUlll/Dipole/internal/services/search/infrastructure/mysql"
	syncmysql "github.com/JekYUlll/Dipole/internal/services/sync/infrastructure/mysql"
)

// Repositories contains one repository instance for each application process.
type Repositories struct {
	CoreProcess    *CoreProcessRepositories
	MessageProcess *MessageProcessRepositories
	SyncProcess    *SyncProcessRepositories
	AgentProcess   *AgentProcessRepositories
	Search         application.SearchIndex
}

type CoreProcessRepositories = coremysql.ProcessRepositories
type SyncProcessRepositories = syncmysql.ProcessRepositories
type AgentProcessRepositories = agentmysql.ProcessRepositories

type MessageProcessRepositories = messagemysql.ProcessRepositories

func NewMessageProcessRepositories(db *sql.DB) (*MessageProcessRepositories, error) {
	return NewMessageProcessRepositoriesWithInboxWrites(db, true)
}

func NewMessageProcessRepositoriesWithInboxWrites(db *sql.DB, enabled bool) (*MessageProcessRepositories, error) {
	return messagemysql.NewProcessRepositories(db, enabled)
}

func NewRepositories(db *sql.DB) (*Repositories, error) {
	if db == nil {
		return nil, fmt.Errorf("repository composition requires database/sql connection")
	}
	repos := &Repositories{}
	coreRepos, err := coremysql.NewProcessRepositories(db)
	if err != nil {
		return nil, fmt.Errorf("compose Core repositories: %w", err)
	}
	repos.CoreProcess = coreRepos
	agentRepos, err := agentmysql.NewProcessRepositories(db)
	if err != nil {
		return nil, fmt.Errorf("compose Agent repositories: %w", err)
	}
	repos.AgentProcess = agentRepos
	messageRepos, err := NewMessageProcessRepositoriesWithInboxWrites(db, true)
	if err != nil {
		return nil, fmt.Errorf("compose Message repositories: %w", err)
	}
	repos.MessageProcess = messageRepos
	syncRepos, err := syncmysql.NewProcessRepositories(db, nil)
	if err != nil {
		return nil, fmt.Errorf("compose Sync repositories: %w", err)
	}
	repos.SyncProcess = syncRepos
	searchAdapter, err := searchmysql.NewSearchIndexRepository(generated.New(db))
	if err != nil {
		return nil, fmt.Errorf("create sqlc search index repository: %w", err)
	}
	repos.Search = searchAdapter
	return repos, nil
}
