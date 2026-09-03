package agentapplication

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/JekYUlll/Dipole/internal/application"
)

type PersistentAgentTaskControlAuthorizerV1 struct {
	store application.AgentPolicyStoreV1
	inbox application.AgentTaskOwnerInboxStoreV1
	now   func() time.Time
}

var _ application.AgentTaskControlAuthorizerV1 = (*PersistentAgentTaskControlAuthorizerV1)(nil)
var _ application.AgentTaskOwnerInboxServiceV1 = (*PersistentAgentTaskControlAuthorizerV1)(nil)

func NewPersistentAgentTaskControlAuthorizerV1(store application.AgentPolicyStoreV1) (*PersistentAgentTaskControlAuthorizerV1, error) {
	return NewPersistentAgentTaskControlAuthorizerV1WithNow(store, time.Now)
}

func NewPersistentAgentTaskControlAuthorizerV1WithNow(store application.AgentPolicyStoreV1, now func() time.Time) (*PersistentAgentTaskControlAuthorizerV1, error) {
	if store == nil {
		return nil, fmt.Errorf("persistent Agent Task control authorizer requires store")
	}
	if now == nil {
		now = time.Now
	}
	authorizer := &PersistentAgentTaskControlAuthorizerV1{store: store, now: now}
	if inbox, ok := store.(application.AgentTaskOwnerInboxStoreV1); ok {
		authorizer.inbox = inbox
	}
	return authorizer, nil
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

func (a *PersistentAgentTaskControlAuthorizerV1) ListOwnedTasks(ctx context.Context, request application.AgentTaskOwnerInboxListRequestV1) (*application.AgentTaskOwnerInboxPageV1, error) {
	if a.inbox == nil {
		return nil, application.ErrAgentTaskInboxUnavailable
	}
	request.TenantID, request.PrincipalUUID, request.AfterTaskUUID = strings.TrimSpace(request.TenantID), strings.TrimSpace(request.PrincipalUUID), strings.TrimSpace(request.AfterTaskUUID)
	if request.TenantID == "" || request.PrincipalUUID == "" || utf8.RuneCountInString(request.TenantID) > 64 ||
		utf8.RuneCountInString(request.PrincipalUUID) > 64 || utf8.RuneCountInString(request.AfterTaskUUID) > 64 ||
		request.Limit < 0 || request.Limit > 100 || (request.AfterUpdatedAt.IsZero() != (request.AfterTaskUUID == "")) {
		return nil, application.ErrAgentTaskInboxInvalid
	}
	if request.Limit == 0 {
		request.Limit = 50
	}
	if request.AfterUpdatedAt.IsZero() {
		request.AfterUpdatedAt = a.now().UTC()
	} else {
		request.AfterUpdatedAt = request.AfterUpdatedAt.UTC()
	}
	storeRequest := request
	storeRequest.Limit++
	items, err := a.inbox.ListOwnedTasks(ctx, storeRequest)
	if err != nil {
		return nil, err
	}
	for index, item := range items {
		if strings.TrimSpace(item.TaskUUID) == "" || item.UpdatedAt.IsZero() {
			return nil, application.ErrAgentTaskInboxConflict
		}
		if item.TenantID != request.TenantID || item.PrincipalUUID != request.PrincipalUUID {
			return nil, application.ErrAgentTaskInboxDenied
		}
		if index > 0 && !agentTaskInboxOrderV1(items[index-1], item) {
			return nil, application.ErrAgentTaskInboxConflict
		}
	}
	page := &application.AgentTaskOwnerInboxPageV1{Tasks: make([]application.AgentTaskInboxItemV1, 0, len(items))}
	if len(items) > request.Limit {
		items = items[:request.Limit]
		last := items[len(items)-1]
		page.NextUpdatedAt, page.NextTaskUUID = last.UpdatedAt.UTC(), last.TaskUUID
	}
	for _, item := range items {
		page.Tasks = append(page.Tasks, application.AgentTaskInboxItemFromTaskV1(item))
	}
	return page, nil
}

func agentTaskInboxOrderV1(previous, current application.AgentTaskV1) bool {
	if previous.UpdatedAt.Equal(current.UpdatedAt) {
		return previous.TaskUUID > current.TaskUUID
	}
	return previous.UpdatedAt.After(current.UpdatedAt)
}
