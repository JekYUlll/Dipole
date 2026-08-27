package application

import (
	"context"
	"errors"
)

var ErrAgentExecutionPolicyDenied = errors.New("agent execution policy denied")

type AgentExecutionPolicyStartV1 struct {
	TenantID        string
	PrincipalUUID   string
	AgentUUID       string
	DelegatedByUUID string
	TriggerType     string
	TriggerRef      string
	RequestID       string
	TraceID         string
	EventID         string
}

type AgentPolicyExecutionV1 struct {
	TaskUUID   string
	Invocation AgentInvocationV1
}

type AgentExecutionPolicyV1 interface {
	Start(ctx context.Context, request AgentExecutionPolicyStartV1) (*AgentPolicyExecutionV1, error)
	Complete(ctx context.Context, execution AgentPolicyExecutionV1) error
	Fail(ctx context.Context, execution AgentPolicyExecutionV1) error
}
