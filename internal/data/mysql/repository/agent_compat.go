package repository

import (
	"context"
	"database/sql"

	mysqlData "github.com/JekYUlll/Dipole/internal/data/mysql"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	agentmysql "github.com/JekYUlll/Dipole/internal/services/agent/infrastructure/mysql"
)

// Agent repository aliases keep embedded and maintenance callers source-compatible
// while the implementations live under the Agent service boundary.
type AICallLogRepository = agentmysql.AICallLogRepository
type AgentArtifactRepository = agentmysql.AgentArtifactRepository
type AgentMCPReadinessEvidenceRepository = agentmysql.AgentMCPReadinessEvidenceRepository
type AgentMCPToolRoundRepository = agentmysql.AgentMCPToolRoundRepository
type AgentMemoryRepository = agentmysql.AgentMemoryRepository
type AgentPolicyRepository = agentmysql.AgentPolicyRepository
type AgentRuntimePromotionControlRepository = agentmysql.AgentRuntimePromotionControlRepository
type AgentToolInvocationRepository = agentmysql.AgentToolInvocationRepository

func NewAICallLogRepository(queries generated.Querier) (*AICallLogRepository, error) {
	return agentmysql.NewAICallLogRepository(queries)
}

func NewAgentArtifactRepository(queries generated.Querier) (*AgentArtifactRepository, error) {
	return agentmysql.NewAgentArtifactRepository(queries)
}

func NewAgentMCPReadinessEvidenceRepository(queries generated.Querier) (*AgentMCPReadinessEvidenceRepository, error) {
	return agentmysql.NewAgentMCPReadinessEvidenceRepository(queries)
}

func NewAgentMCPToolRoundRepository(queries generated.Querier) (*AgentMCPToolRoundRepository, error) {
	return agentmysql.NewAgentMCPToolRoundRepository(queries)
}

func NewAgentMemoryRepository(queries generated.Querier) (*AgentMemoryRepository, error) {
	return agentmysql.NewAgentMemoryRepository(queries)
}

func NewAgentMemoryRepositoryWithTransactions(store interface {
	Queries() *generated.Queries
	WithinTx(context.Context, *sql.TxOptions, func(*generated.Queries) error) error
}) (*AgentMemoryRepository, error) {
	return agentmysql.NewAgentMemoryRepositoryWithTransactions(store)
}

func NewAgentPolicyRepository(queries generated.Querier) (*AgentPolicyRepository, error) {
	return agentmysql.NewAgentPolicyRepository(queries)
}

func NewAgentPolicyRepositoryWithTransactions(store mysqlData.TransactionStore) (*AgentPolicyRepository, error) {
	return agentmysql.NewAgentPolicyRepositoryWithTransactions(store)
}

func NewAgentRuntimePromotionControlRepository(store *mysqlData.Store) (*AgentRuntimePromotionControlRepository, error) {
	return agentmysql.NewAgentRuntimePromotionControlRepository(store)
}
