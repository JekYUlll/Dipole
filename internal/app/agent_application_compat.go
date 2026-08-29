package app

import (
	"context"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	agentapplication "github.com/JekYUlll/Dipole/internal/services/agent/application"
)

// Agent application implementations live under the Agent service boundary.
// These aliases preserve the embedded composition root during migration.
type StaticAgentExecutionPolicyV1 = agentapplication.StaticAgentExecutionPolicyV1
type PersistentAgentExecutionPolicyV1 = agentapplication.PersistentAgentExecutionPolicyV1
type AgentMemoryTaskReaderV1 = agentapplication.AgentMemoryTaskReaderV1

func NewPersistentAgentToolInvocationAuditServiceV1(store application.AgentToolInvocationStoreV1, resolver application.AgentInvocationResolverV1, approvals application.AgentToolApprovalReaderV1, receipts application.MessageCommandReceiptQuery) (application.AgentToolInvocationAuditServiceV1, error) {
	return agentapplication.NewPersistentAgentToolInvocationAuditServiceV1(store, resolver, approvals, receipts)
}

func NewPersistentAgentToolInvocationAuditServiceV1WithClock(store application.AgentToolInvocationStoreV1, resolver application.AgentInvocationResolverV1, approvals application.AgentToolApprovalReaderV1, receipts application.MessageCommandReceiptQuery, now func() time.Time) (application.AgentToolInvocationAuditServiceV1, error) {
	return agentapplication.NewPersistentAgentToolInvocationAuditServiceV1WithClock(store, resolver, approvals, receipts, now)
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

func EnsureEmbeddedAgentDefinitionV1(ctx context.Context, store application.AgentPolicyStoreV1, tenantID, agentUUID string, permissions []string, scopes []application.AgentResourceScopeV1) error {
	return agentapplication.EnsureEmbeddedAgentDefinitionV1(ctx, store, tenantID, agentUUID, permissions, scopes)
}

func agentTaskUUIDV1(request application.AgentExecutionPolicyStartV1) string {
	return agentapplication.AgentTaskUUIDV1(request)
}

func clonePolicyScopesV1(scopes []application.AgentResourceScopeV1) []application.AgentResourceScopeV1 {
	return agentapplication.ClonePolicyScopesV1(scopes)
}
