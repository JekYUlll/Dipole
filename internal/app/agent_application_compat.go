package app

import (
	"context"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	agentapplication "github.com/JekYUlll/Dipole/internal/services/agent/application"
)

func NewPersistentAgentToolInvocationAuditServiceV1(store application.AgentToolInvocationStoreV1, resolver application.AgentInvocationResolverV1, approvals application.AgentToolApprovalReaderV1, receipts application.MessageCommandReceiptQuery) (application.AgentToolInvocationAuditServiceV1, error) {
	return agentapplication.NewPersistentAgentToolInvocationAuditServiceV1(store, resolver, approvals, receipts)
}

func NewPersistentAgentToolInvocationAuditServiceV1WithClock(store application.AgentToolInvocationStoreV1, resolver application.AgentInvocationResolverV1, approvals application.AgentToolApprovalReaderV1, receipts application.MessageCommandReceiptQuery, now func() time.Time) (application.AgentToolInvocationAuditServiceV1, error) {
	return agentapplication.NewPersistentAgentToolInvocationAuditServiceV1WithClock(store, resolver, approvals, receipts, now)
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
