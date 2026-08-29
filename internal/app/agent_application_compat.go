package app

import (
	"context"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	agentapplication "github.com/JekYUlll/Dipole/internal/services/agent/application"
)

// Agent application implementations live under the Agent service boundary.
// These aliases preserve the embedded composition root during migration.
type PersistentAgentApprovalServiceV1 = agentapplication.PersistentAgentApprovalServiceV1
type PersistentAgentTaskControlAuthorizerV1 = agentapplication.PersistentAgentTaskControlAuthorizerV1
type PersistentAgentTaskWorkflowProjectionServiceV1 = agentapplication.PersistentAgentTaskWorkflowProjectionServiceV1
type PersistentAgentWorkflowRepairAuditServiceV1 = agentapplication.PersistentAgentWorkflowRepairAuditServiceV1
type LocalAgentCommandV1 = agentapplication.LocalAgentCommandV1
type LocalAgentCapabilityV1 = agentapplication.LocalAgentCapabilityV1
type PersistentAgentWorkflowRepairPrepareServiceV1 = agentapplication.PersistentAgentWorkflowRepairPrepareServiceV1
type PersistentAgentWorkflowRepairExecutorV1 = agentapplication.PersistentAgentWorkflowRepairExecutorV1
type StaticAgentExecutionPolicyV1 = agentapplication.StaticAgentExecutionPolicyV1
type PersistentAgentExecutionPolicyV1 = agentapplication.PersistentAgentExecutionPolicyV1
type PersistentAgentInvocationResolverV1 = agentapplication.PersistentAgentInvocationResolverV1
type PersistentAgentRunAdmissionV1 = agentapplication.PersistentAgentRunAdmissionV1
type AgentMemoryTaskReaderV1 = agentapplication.AgentMemoryTaskReaderV1
type AgentMessageCommandExecutionServiceV1 = agentapplication.AgentMessageCommandExecutionServiceV1
type PersistentAgentRuntimePromotionControlServiceV1 = agentapplication.PersistentAgentRuntimePromotionControlServiceV1
type PersistentAgentActiveRunPromotionAuthorizerV1 = agentapplication.PersistentAgentActiveRunPromotionAuthorizerV1

func NewPersistentAgentApprovalServiceV1(store application.AgentPolicyStoreV1) (*PersistentAgentApprovalServiceV1, error) {
	return agentapplication.NewPersistentAgentApprovalServiceV1(store)
}

func NewPersistentAgentApprovalServiceV1WithClock(store application.AgentPolicyStoreV1, now func() time.Time) (*PersistentAgentApprovalServiceV1, error) {
	return agentapplication.NewPersistentAgentApprovalServiceV1WithClock(store, now)
}

func NewPersistentAgentTaskControlAuthorizerV1(store application.AgentPolicyStoreV1) (*PersistentAgentTaskControlAuthorizerV1, error) {
	return agentapplication.NewPersistentAgentTaskControlAuthorizerV1(store)
}

func NewPersistentAgentTaskWorkflowProjectionServiceV1(store application.AgentPolicyStoreV1) (*PersistentAgentTaskWorkflowProjectionServiceV1, error) {
	return agentapplication.NewPersistentAgentTaskWorkflowProjectionServiceV1(store)
}

func NewPersistentAgentToolInvocationAuditServiceV1(store application.AgentToolInvocationStoreV1, resolver application.AgentInvocationResolverV1, approvals application.AgentToolApprovalReaderV1, receipts application.MessageCommandReceiptQuery) (application.AgentToolInvocationAuditServiceV1, error) {
	return agentapplication.NewPersistentAgentToolInvocationAuditServiceV1(store, resolver, approvals, receipts)
}

func NewPersistentAgentToolInvocationAuditServiceV1WithClock(store application.AgentToolInvocationStoreV1, resolver application.AgentInvocationResolverV1, approvals application.AgentToolApprovalReaderV1, receipts application.MessageCommandReceiptQuery, now func() time.Time) (application.AgentToolInvocationAuditServiceV1, error) {
	return agentapplication.NewPersistentAgentToolInvocationAuditServiceV1WithClock(store, resolver, approvals, receipts, now)
}

func NewPersistentAgentWorkflowRepairAuditServiceV1(policies application.AgentPolicyStoreV1, repairs application.AgentWorkflowRepairAuditStoreV1) (*PersistentAgentWorkflowRepairAuditServiceV1, error) {
	return agentapplication.NewPersistentAgentWorkflowRepairAuditServiceV1(policies, repairs)
}

func NewPersistentAgentWorkflowRepairAuditServiceV1WithClock(policies application.AgentPolicyStoreV1, repairs application.AgentWorkflowRepairAuditStoreV1, now func() time.Time) (*PersistentAgentWorkflowRepairAuditServiceV1, error) {
	return agentapplication.NewPersistentAgentWorkflowRepairAuditServiceV1WithClock(policies, repairs, now)
}

func NewLocalAgentCommandV1(messages agentapplication.AgentCommandMessages) (*LocalAgentCommandV1, error) {
	return agentapplication.NewLocalAgentCommandV1(messages)
}

func NewLocalAgentCapabilityV1(core application.CoreCapability, messages agentapplication.AgentCapabilityMessages, conversations agentapplication.AgentCapabilityConversations, commands application.AgentCommandV1) (*LocalAgentCapabilityV1, error) {
	return agentapplication.NewLocalAgentCapabilityV1(core, messages, conversations, commands)
}

func NewPersistentAgentInvocationResolverV1(store application.AgentPolicyStoreV1, activeAuthorizers ...application.AgentActiveRunPromotionAuthorizerV1) (*PersistentAgentInvocationResolverV1, error) {
	return agentapplication.NewPersistentAgentInvocationResolverV1(store, activeAuthorizers...)
}

func NewPersistentAgentInvocationResolverV1WithClock(store application.AgentPolicyStoreV1, now func() time.Time, activeAuthorizers ...application.AgentActiveRunPromotionAuthorizerV1) (*PersistentAgentInvocationResolverV1, error) {
	return agentapplication.NewPersistentAgentInvocationResolverV1WithClock(store, now, activeAuthorizers...)
}

func NewPersistentAgentRunAdmissionV1(store application.AgentPolicyStoreV1, activeAuthorizers ...application.AgentActiveRunPromotionAuthorizerV1) (*PersistentAgentRunAdmissionV1, error) {
	return agentapplication.NewPersistentAgentRunAdmissionV1(store, activeAuthorizers...)
}

func NewPersistentAgentRunAdmissionV1WithClock(store application.AgentPolicyStoreV1, now func() time.Time, activeAuthorizers ...application.AgentActiveRunPromotionAuthorizerV1) (*PersistentAgentRunAdmissionV1, error) {
	return agentapplication.NewPersistentAgentRunAdmissionV1WithClock(store, now, activeAuthorizers...)
}

func NewPersistentAgentExecutionPolicyV1WithClock(store application.AgentPolicyStoreV1, now func() time.Time) (*PersistentAgentExecutionPolicyV1, error) {
	return agentapplication.NewPersistentAgentExecutionPolicyV1WithClock(store, now)
}

func NewStaticAgentExecutionPolicyV1(permissions []string, scopes []application.AgentResourceScopeV1) (*StaticAgentExecutionPolicyV1, error) {
	return agentapplication.NewStaticAgentExecutionPolicyV1(permissions, scopes)
}

func NewPersistentAgentExecutionPolicyV1(store application.AgentPolicyStoreV1) (*PersistentAgentExecutionPolicyV1, error) {
	return agentapplication.NewPersistentAgentExecutionPolicyV1(store)
}

func NewAgentMessageCommandExecutionV1(tools application.AgentToolInvocationReaderV1, resolver application.AgentInvocationResolverV1, commands application.AgentCommandV1) (*AgentMessageCommandExecutionServiceV1, error) {
	return agentapplication.NewAgentMessageCommandExecutionV1(tools, resolver, commands)
}

func NewPersistentAgentRuntimePromotionControlServiceV1(policies application.AgentPolicyStoreV1, artifacts application.AgentArtifactStoreV1, control application.AgentRuntimePromotionControlStoreV1) (*PersistentAgentRuntimePromotionControlServiceV1, error) {
	return agentapplication.NewPersistentAgentRuntimePromotionControlServiceV1(policies, artifacts, control)
}

func newPersistentAgentRuntimePromotionControlServiceV1(policies application.AgentPolicyStoreV1, artifacts application.AgentArtifactStoreV1, control application.AgentRuntimePromotionControlStoreV1, now func() time.Time) (*PersistentAgentRuntimePromotionControlServiceV1, error) {
	return agentapplication.NewPersistentAgentRuntimePromotionControlServiceV1WithClock(policies, artifacts, control, now)
}

func NewPersistentAgentActiveRunPromotionAuthorizerV1(store application.AgentRuntimePromotionGrantStoreV1) (*PersistentAgentActiveRunPromotionAuthorizerV1, error) {
	return agentapplication.NewPersistentAgentActiveRunPromotionAuthorizerV1(store)
}

func EnsureEmbeddedAgentDefinitionV1(ctx context.Context, store application.AgentPolicyStoreV1, tenantID, agentUUID string, permissions []string, scopes []application.AgentResourceScopeV1) error {
	return agentapplication.EnsureEmbeddedAgentDefinitionV1(ctx, store, tenantID, agentUUID, permissions, scopes)
}

func agentTaskUUIDV1(request application.AgentExecutionPolicyStartV1) string {
	return agentapplication.AgentTaskUUIDV1(request)
}

func clonePolicyScopesV1(scopes []application.AgentResourceScopeV1) []application.AgentResourceScopeV1 {
	return agentapplication.ClonePolicyScopesV1(scopes)
}

func NewPersistentAgentWorkflowRepairPrepareServiceV1(policies application.AgentPolicyStoreV1, repairs application.AgentWorkflowRepairAuditStoreV1, executions application.AgentWorkflowRepairExecutionStoreV1) (*PersistentAgentWorkflowRepairPrepareServiceV1, error) {
	return agentapplication.NewPersistentAgentWorkflowRepairPrepareServiceV1(policies, repairs, executions)
}

func NewPersistentAgentWorkflowRepairPrepareServiceV1WithClock(policies application.AgentPolicyStoreV1, repairs application.AgentWorkflowRepairAuditStoreV1, executions application.AgentWorkflowRepairExecutionStoreV1, now func() time.Time) (*PersistentAgentWorkflowRepairPrepareServiceV1, error) {
	return agentapplication.NewPersistentAgentWorkflowRepairPrepareServiceV1WithClock(policies, repairs, executions, now)
}

func NewPersistentAgentWorkflowRepairExecutorV1(policies application.AgentPolicyStoreV1, repairs application.AgentWorkflowRepairAuditStoreV1, executions application.AgentWorkflowRepairExecutionStoreV1, transaction application.AgentWorkflowRepairTransactionalStoreV1) (*PersistentAgentWorkflowRepairExecutorV1, error) {
	return agentapplication.NewPersistentAgentWorkflowRepairExecutorV1(policies, repairs, executions, transaction)
}

func NewPersistentAgentWorkflowRepairExecutorV1WithClock(policies application.AgentPolicyStoreV1, repairs application.AgentWorkflowRepairAuditStoreV1, executions application.AgentWorkflowRepairExecutionStoreV1, transaction application.AgentWorkflowRepairTransactionalStoreV1, now func() time.Time) (*PersistentAgentWorkflowRepairExecutorV1, error) {
	return agentapplication.NewPersistentAgentWorkflowRepairExecutorV1WithClock(policies, repairs, executions, transaction, now)
}
