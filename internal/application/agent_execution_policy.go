package application

import (
	"context"
	"errors"
)

var ErrAgentExecutionPolicyDenied = errors.New("agent execution policy denied")

type AgentExecutionPolicyStartV1 struct {
	TenantID         string
	PrincipalUUID    string
	AgentUUID        string
	DelegatedByUUID  string
	TriggerType      string
	TriggerRef       string
	RequestID        string
	TraceID          string
	EventID          string
	SubscriptionUUID string
}

type AgentPolicyExecutionV1 struct {
	TaskUUID   string
	RunUUID    string
	Invocation AgentInvocationV1
}

type AgentExecutionPolicyV1 interface {
	Start(ctx context.Context, request AgentExecutionPolicyStartV1) (*AgentPolicyExecutionV1, error)
	Complete(ctx context.Context, execution AgentPolicyExecutionV1) error
	Fail(ctx context.Context, execution AgentPolicyExecutionV1) error
}

type AgentInvocationResolverV1 interface {
	Resolve(ctx context.Context, taskUUID, runUUID string) (AgentInvocationV1, error)
}

type AgentTaskControlAuthorizationV1 struct {
	TaskUUID   string
	Status     AgentTaskStatusV1
	MCPRunUUID string
	Workflow   *AgentTaskWorkflowProjectionV1
}

type AgentTaskControlAuthorizerV1 interface {
	AuthorizeTaskControl(ctx context.Context, taskUUID, principalUUID string) (*AgentTaskControlAuthorizationV1, error)
}

type AgentRunAdmissionRequestV1 struct {
	AgentExecutionPolicyStartV1
	RuntimeID        string
	Mode             string
	CandidateVersion string
}

type AgentActiveRunPromotionRequestV1 struct {
	RuntimeID        string
	CandidateVersion string
	Task             AgentTaskV1
	Definition       AgentDefinitionVersionV1
}

type AgentActiveRunPromotionAuthorizerV1 interface {
	AuthorizeActiveRun(ctx context.Context, request AgentActiveRunPromotionRequestV1) error
}

type AgentRunAdmissionV1 struct {
	TaskUUID   string
	RunUUID    string
	RunStatus  AgentRunStatusV1
	Invocation AgentInvocationV1
}

type AgentRunAdmissionServiceV1 interface {
	Admit(ctx context.Context, request AgentRunAdmissionRequestV1) (*AgentRunAdmissionV1, error)
	Finish(ctx context.Context, taskUUID, runUUID, runtimeID, mode string, runStatus AgentRunStatusV1, lastError string) error
}
