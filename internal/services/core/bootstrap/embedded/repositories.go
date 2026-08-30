// Package embedded owns the local composition used by the compatibility server.
package embedded

import (
	"database/sql"
	"fmt"

	agentmysql "github.com/JekYUlll/Dipole/internal/services/agent/infrastructure/mysql"
	coremysql "github.com/JekYUlll/Dipole/internal/services/core/infrastructure/mysql"
	messagemysql "github.com/JekYUlll/Dipole/internal/services/message/infrastructure/mysql"
	syncmysql "github.com/JekYUlll/Dipole/internal/services/sync/infrastructure/mysql"
)

// Repositories contains one repository instance for each application process.
type Repositories struct {
	CoreProcess    *CoreProcessRepositories
	MessageProcess *MessageProcessRepositories
	SyncProcess    *SyncProcessRepositories
	AgentProcess   *AgentProcessRepositories
}

type CoreProcessRepositories = coremysql.ProcessRepositories
type SyncProcessRepositories = syncmysql.ProcessRepositories
type AgentProcessRepositories = agentmysql.ProcessRepositories

type MessageProcessRepositories = messagemysql.ProcessRepositories

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
	messageRepos, err := messagemysql.NewProcessRepositories(db, true)
	if err != nil {
		return nil, fmt.Errorf("compose Message repositories: %w", err)
	}
	repos.MessageProcess = messageRepos
	syncRepos, err := syncmysql.NewProcessRepositories(db, nil)
	if err != nil {
		return nil, fmt.Errorf("compose Sync repositories: %w", err)
	}
	repos.SyncProcess = syncRepos
	return repos, nil
}
