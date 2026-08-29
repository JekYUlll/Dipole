package app

import (
	"database/sql"

	agentmysql "github.com/JekYUlll/Dipole/internal/services/agent/infrastructure/mysql"
)

// Agent repository symbols remain available to embedded callers during
// service extraction. New standalone code should use agentmysql directly.
type AgentProcessRepositories = agentmysql.ProcessRepositories

func NewAgentProcessRepositories(db *sql.DB) (*AgentProcessRepositories, error) {
	return agentmysql.NewProcessRepositories(db)
}
