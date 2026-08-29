package app

import (
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	agentapplication "github.com/JekYUlll/Dipole/internal/services/agent/application"
)

// Agent application implementations live under the Agent service boundary.
// These aliases preserve the embedded composition root during migration.
type PersistentAgentApprovalGrantResolverV1 = agentapplication.PersistentAgentApprovalGrantResolverV1
type PersistentAgentApprovalServiceV1 = agentapplication.PersistentAgentApprovalServiceV1
type PersistentAgentTaskControlAuthorizerV1 = agentapplication.PersistentAgentTaskControlAuthorizerV1
type PersistentAgentDefinitionCatalogV1 = agentapplication.PersistentAgentDefinitionCatalogV1
type PersistentAgentMemoryCandidatePromotionServiceV1 = agentapplication.PersistentAgentMemoryCandidatePromotionServiceV1
type PersistentAgentTaskWorkflowProjectionServiceV1 = agentapplication.PersistentAgentTaskWorkflowProjectionServiceV1

func NewPersistentAgentApprovalGrantResolverV1(store application.AgentApprovalGrantStoreV1) (*PersistentAgentApprovalGrantResolverV1, error) {
	return agentapplication.NewPersistentAgentApprovalGrantResolverV1(store)
}

func NewPersistentAgentApprovalGrantResolverV1WithClock(store application.AgentApprovalGrantStoreV1, now func() time.Time) (*PersistentAgentApprovalGrantResolverV1, error) {
	return agentapplication.NewPersistentAgentApprovalGrantResolverV1WithClock(store, now)
}

func NewPersistentAgentApprovalServiceV1(store application.AgentPolicyStoreV1) (*PersistentAgentApprovalServiceV1, error) {
	return agentapplication.NewPersistentAgentApprovalServiceV1(store)
}

func NewPersistentAgentApprovalServiceV1WithClock(store application.AgentPolicyStoreV1, now func() time.Time) (*PersistentAgentApprovalServiceV1, error) {
	return agentapplication.NewPersistentAgentApprovalServiceV1WithClock(store, now)
}

func NewPersistentAgentTaskControlAuthorizerV1(store application.AgentPolicyStoreV1) (*PersistentAgentTaskControlAuthorizerV1, error) {
	return agentapplication.NewPersistentAgentTaskControlAuthorizerV1(store)
}

func NewPersistentAgentDefinitionCatalogV1(store application.AgentDefinitionCatalogStoreV1, now func() time.Time) (*PersistentAgentDefinitionCatalogV1, error) {
	return agentapplication.NewPersistentAgentDefinitionCatalogV1(store, now)
}

func NewPersistentAgentMemoryCandidatePromotionServiceV1(store application.AgentMemoryCandidatePromotionStoreV1, now func() time.Time) (*PersistentAgentMemoryCandidatePromotionServiceV1, error) {
	return agentapplication.NewPersistentAgentMemoryCandidatePromotionServiceV1(store, now)
}

func NewPersistentAgentTaskWorkflowProjectionServiceV1(store application.AgentPolicyStoreV1) (*PersistentAgentTaskWorkflowProjectionServiceV1, error) {
	return agentapplication.NewPersistentAgentTaskWorkflowProjectionServiceV1(store)
}
