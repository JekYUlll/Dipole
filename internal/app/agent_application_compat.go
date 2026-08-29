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
type PersistentAgentMCPReadinessEvidenceResolverV1 = agentapplication.PersistentAgentMCPReadinessEvidenceResolverV1
type PersistentAgentMCPReadinessEvidencePublisherV1 = agentapplication.PersistentAgentMCPReadinessEvidencePublisherV1
type AgentRuntimePromotionEvidenceReviewServiceV1 = agentapplication.AgentRuntimePromotionEvidenceReviewServiceV1
type PersistentAgentWorkflowRepairAuditServiceV1 = agentapplication.PersistentAgentWorkflowRepairAuditServiceV1
type PersistentAgentArtifactServiceV1 = agentapplication.PersistentAgentArtifactServiceV1
type PersistentAgentMemoryOwnerControlV1 = agentapplication.PersistentAgentMemoryOwnerControlV1

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

func NewPersistentAgentMCPReadinessEvidenceResolverV1(store application.AgentMCPReadinessEvidenceStoreV1, now func() time.Time) (*PersistentAgentMCPReadinessEvidenceResolverV1, error) {
	return agentapplication.NewPersistentAgentMCPReadinessEvidenceResolverV1(store, now)
}

func NewPersistentAgentMCPReadinessEvidencePublisherV1(store application.AgentMCPReadinessEvidenceStoreV1) (*PersistentAgentMCPReadinessEvidencePublisherV1, error) {
	return agentapplication.NewPersistentAgentMCPReadinessEvidencePublisherV1(store)
}

func NewPersistentAgentMCPToolRoundServiceV1(store application.AgentMCPToolRoundStoreV1, invocations application.AgentToolInvocationReaderV1) (application.AgentMCPToolRoundServiceV1, error) {
	return agentapplication.NewPersistentAgentMCPToolRoundServiceV1(store, invocations)
}

func NewPersistentAgentToolInvocationAuditServiceV1(store application.AgentToolInvocationStoreV1, resolver application.AgentInvocationResolverV1, approvals application.AgentToolApprovalReaderV1, receipts application.MessageCommandReceiptQuery) (application.AgentToolInvocationAuditServiceV1, error) {
	return agentapplication.NewPersistentAgentToolInvocationAuditServiceV1(store, resolver, approvals, receipts)
}

func NewPersistentAgentToolInvocationAuditServiceV1WithClock(store application.AgentToolInvocationStoreV1, resolver application.AgentInvocationResolverV1, approvals application.AgentToolApprovalReaderV1, receipts application.MessageCommandReceiptQuery, now func() time.Time) (application.AgentToolInvocationAuditServiceV1, error) {
	return agentapplication.NewPersistentAgentToolInvocationAuditServiceV1WithClock(store, resolver, approvals, receipts, now)
}

func NewAgentRuntimePromotionEvidenceReviewServiceV1(controls application.AgentRuntimePromotionControlServiceV1, reader application.AgentRuntimePromotionEvidenceReaderV1) (*AgentRuntimePromotionEvidenceReviewServiceV1, error) {
	return agentapplication.NewAgentRuntimePromotionEvidenceReviewServiceV1(controls, reader)
}

func NewPersistentAgentWorkflowRepairAuditServiceV1(policies application.AgentPolicyStoreV1, repairs application.AgentWorkflowRepairAuditStoreV1) (*PersistentAgentWorkflowRepairAuditServiceV1, error) {
	return agentapplication.NewPersistentAgentWorkflowRepairAuditServiceV1(policies, repairs)
}

func NewPersistentAgentWorkflowRepairAuditServiceV1WithClock(policies application.AgentPolicyStoreV1, repairs application.AgentWorkflowRepairAuditStoreV1, now func() time.Time) (*PersistentAgentWorkflowRepairAuditServiceV1, error) {
	return agentapplication.NewPersistentAgentWorkflowRepairAuditServiceV1WithClock(policies, repairs, now)
}

func NewPersistentAgentArtifactServiceV1(policies agentapplication.AgentArtifactPolicyStoreV1, artifacts application.AgentArtifactStoreV1, blobs application.AgentArtifactBlobStoreV1) (*PersistentAgentArtifactServiceV1, error) {
	return agentapplication.NewPersistentAgentArtifactServiceV1(policies, artifacts, blobs)
}

func NewPersistentAgentMemoryOwnerControlV1(store application.AgentMemoryOwnerStoreV1, now func() time.Time) (*PersistentAgentMemoryOwnerControlV1, error) {
	return agentapplication.NewPersistentAgentMemoryOwnerControlV1(store, now)
}
