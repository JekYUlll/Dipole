package agentmysql

import (
	"database/sql"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/application"
	mysqlData "github.com/JekYUlll/Dipole/internal/platform/mysql"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
)

// ProcessRepositories contains the repositories owned by the Agent service.
// Keeping this composition beside the Agent SQLC implementations makes the
// standalone service boundary explicit while embedded callers use a wrapper.
type ProcessRepositories struct {
	AICallLogs            application.AICallLogStore
	Policy                application.AgentPolicyStoreV1
	TaskTimeline          application.AgentTaskTimelineStoreV1
	DefinitionCatalog     application.AgentDefinitionCatalogStoreV1
	ApprovalGrants        application.AgentApprovalGrantStoreV1
	Promotions            application.AgentRuntimePromotionGrantStoreV1
	PromotionControls     application.AgentRuntimePromotionControlStoreV1
	ReadinessEvidence     application.AgentMCPReadinessEvidenceStoreV1
	Subscriptions         application.AgentEventSubscriptionStoreV1
	Repairs               application.AgentWorkflowRepairAuditStoreV1
	RepairExecutions      application.AgentWorkflowRepairExecutionStoreV1
	RepairTransactions    application.AgentWorkflowRepairTransactionalStoreV1
	Artifacts             application.AgentArtifactStoreV1
	Memories              application.AgentMemoryStoreV1
	MemoryOwners          application.AgentMemoryOwnerStoreV1
	MemoryPromotions      application.AgentMemoryCandidatePromotionStoreV1
	ToolAudits            application.AgentToolInvocationStoreV1
	ToolRounds            application.AgentMCPToolRoundStoreV1
	OAuthTransactions     application.AgentOAuthAuthorizationTransactionStoreV1
	OAuthCallbackHandoffs application.AgentOAuthCallbackHandoffStoreV1
}

func NewProcessRepositories(db *sql.DB) (*ProcessRepositories, error) {
	if db == nil {
		return nil, fmt.Errorf("agent repository composition requires database/sql connection")
	}
	mysqlStore, err := mysqlData.NewStore(db)
	if err != nil {
		return nil, fmt.Errorf("create agent transaction store: %w", err)
	}
	queries := generated.New(db)
	aiCallLogs, err := NewAICallLogRepository(queries)
	if err != nil {
		return nil, fmt.Errorf("create sqlc AI call log repository: %w", err)
	}
	policy, err := NewAgentPolicyRepositoryWithTransactions(mysqlStore)
	if err != nil {
		return nil, fmt.Errorf("create sqlc Agent Policy repository: %w", err)
	}
	artifacts, err := NewAgentArtifactRepository(queries)
	if err != nil {
		return nil, fmt.Errorf("create sqlc Agent Artifact repository: %w", err)
	}
	memories, err := NewAgentMemoryRepositoryWithTransactions(mysqlStore)
	if err != nil {
		return nil, fmt.Errorf("create sqlc Agent Memory repository: %w", err)
	}
	toolAudits, err := NewAgentToolInvocationRepository(queries)
	if err != nil {
		return nil, fmt.Errorf("create sqlc Agent Tool invocation repository: %w", err)
	}
	toolRounds, err := NewAgentMCPToolRoundRepository(queries)
	if err != nil {
		return nil, fmt.Errorf("create sqlc Agent MCP Tool round repository: %w", err)
	}
	oauthTransactions, err := NewAgentOAuthAuthorizationTransactionRepository(queries)
	if err != nil {
		return nil, fmt.Errorf("create sqlc Agent OAuth authorization transaction repository: %w", err)
	}
	oauthCallbackHandoffs, err := NewAgentOAuthCallbackHandoffRepository(queries)
	if err != nil {
		return nil, fmt.Errorf("create sqlc Agent OAuth callback handoff repository: %w", err)
	}
	promotionControls, err := NewAgentRuntimePromotionControlRepository(mysqlStore)
	if err != nil {
		return nil, fmt.Errorf("create sqlc Agent Runtime promotion control repository: %w", err)
	}
	readinessEvidence, err := NewAgentMCPReadinessEvidenceRepository(queries)
	if err != nil {
		return nil, fmt.Errorf("create sqlc Agent MCP readiness evidence repository: %w", err)
	}
	return &ProcessRepositories{
		AICallLogs: aiCallLogs, Policy: policy, TaskTimeline: policy,
		DefinitionCatalog: policy, ApprovalGrants: policy, Promotions: policy,
		Subscriptions: policy, Repairs: policy, RepairExecutions: policy, RepairTransactions: policy, Artifacts: artifacts,
		Memories: memories, MemoryOwners: memories, MemoryPromotions: memories,
		ToolAudits: toolAudits, ToolRounds: toolRounds, OAuthTransactions: oauthTransactions, OAuthCallbackHandoffs: oauthCallbackHandoffs,
		PromotionControls: promotionControls, ReadinessEvidence: readinessEvidence,
	}, nil
}
