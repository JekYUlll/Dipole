package agentapplication

import (
	"context"
	"fmt"
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
)

type PersistentAgentTaskControlAuthorizerV1 struct {
	store application.AgentPolicyStoreV1
}

var _ application.AgentTaskControlAuthorizerV1 = (*PersistentAgentTaskControlAuthorizerV1)(nil)

func NewPersistentAgentTaskControlAuthorizerV1(store application.AgentPolicyStoreV1) (*PersistentAgentTaskControlAuthorizerV1, error) {
	if store == nil {
		return nil, fmt.Errorf("persistent Agent Task control authorizer requires store")
	}
	return &PersistentAgentTaskControlAuthorizerV1{store: store}, nil
}

func (a *PersistentAgentTaskControlAuthorizerV1) AuthorizeTaskControl(ctx context.Context, taskUUID, principalUUID string) (*application.AgentTaskControlAuthorizationV1, error) {
	taskUUID, principalUUID = strings.TrimSpace(taskUUID), strings.TrimSpace(principalUUID)
	if taskUUID == "" || principalUUID == "" {
		return nil, fmt.Errorf("%w: Agent Task and principal are required", application.ErrAgentExecutionPolicyDenied)
	}
	task, err := a.store.GetTask(ctx, taskUUID)
	if err != nil {
		return nil, fmt.Errorf("get Agent Task control policy: %w", err)
	}
	if task == nil || strings.TrimSpace(task.PrincipalUUID) != principalUUID {
		return nil, fmt.Errorf("%w: Agent Task control policy unavailable", application.ErrAgentExecutionPolicyDenied)
	}
	authorization := &application.AgentTaskControlAuthorizationV1{TaskUUID: task.TaskUUID, Status: task.Status}
	// Only disclose a currently executable shadow run. The MCP endpoint still
	// validates its bearer token and resolves the invocation server-side.
	mcpRunUUID, deriveErr := application.AgentRunUUIDV1(task.TaskUUID, "dipole-agent", "shadow")
	if deriveErr != nil {
		return nil, fmt.Errorf("derive Agent MCP Run: %w", deriveErr)
	}
	mcpRun, runErr := a.store.GetRun(ctx, mcpRunUUID)
	if runErr != nil {
		return nil, fmt.Errorf("get Agent MCP Run: %w", runErr)
	}
	if mcpRun != nil {
		if mcpRun.TaskUUID != task.TaskUUID || mcpRun.RuntimeID != "dipole-agent" || mcpRun.Mode != "shadow" {
			return nil, fmt.Errorf("%w: Agent MCP Run binding is invalid", application.ErrAgentExecutionPolicyDenied)
		}
		if mcpRun.Status == application.AgentRunStatusRunning {
			authorization.MCPRunUUID = mcpRun.RunUUID
		}
	}
	if task.Workflow != nil {
		workflow := *task.Workflow
		authorization.Workflow = &workflow
	}
	return authorization, nil
}
