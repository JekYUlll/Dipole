package application

import (
	"context"
	"time"
)

type AgentApprovalGrantRequestV1 struct {
	TaskUUID        string
	RunUUID         string
	RuntimeID       string
	Mode            string
	CapabilityID    string
	ResourceScope   AgentResourceScopeV1
	ArgumentsSHA256 string
}

type AgentApprovalGrantResolverV1 interface {
	ResolveGrant(ctx context.Context, request AgentApprovalGrantRequestV1) (*AgentApprovalV1, error)
}

type AgentApprovalGrantStoreV1 interface {
	GetRun(ctx context.Context, runUUID string) (*AgentRunV1, error)
	GetTask(ctx context.Context, taskUUID string) (*AgentTaskV1, error)
	ListApprovedAgentApprovalGrants(ctx context.Context, taskUUID, capabilityID, scopeSHA256, argumentsSHA256 string, at time.Time, limit int) ([]AgentApprovalV1, error)
}
