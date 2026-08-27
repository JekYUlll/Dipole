package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
)

const agentTaskWorkflowPrefixV1 = "dipole-agent-task/"

type PersistentAgentTaskWorkflowProjectionServiceV1 struct {
	store application.AgentPolicyStoreV1
}

var _ application.AgentTaskWorkflowProjectionServiceV1 = (*PersistentAgentTaskWorkflowProjectionServiceV1)(nil)

func NewPersistentAgentTaskWorkflowProjectionServiceV1(store application.AgentPolicyStoreV1) (*PersistentAgentTaskWorkflowProjectionServiceV1, error) {
	if store == nil {
		return nil, fmt.Errorf("persistent Agent Task Workflow projection requires store")
	}
	return &PersistentAgentTaskWorkflowProjectionServiceV1{store: store}, nil
}

func (s *PersistentAgentTaskWorkflowProjectionServiceV1) Project(ctx context.Context, request application.AgentTaskWorkflowProjectionRequestV1) (*application.AgentTaskWorkflowProjectionV1, error) {
	projection := request.Projection
	projection.TaskUUID = strings.TrimSpace(projection.TaskUUID)
	projection.WorkflowID = strings.TrimSpace(projection.WorkflowID)
	projection.RunID = strings.TrimSpace(projection.RunID)
	request.RunUUID, request.RuntimeID, request.Mode = strings.TrimSpace(request.RunUUID), strings.TrimSpace(request.RuntimeID), strings.TrimSpace(request.Mode)
	if err := projection.Validate(); err != nil || request.RunUUID == "" || request.RuntimeID == "" || request.Mode == "" ||
		projection.WorkflowID != agentTaskWorkflowPrefixV1+projection.TaskUUID {
		return nil, fmt.Errorf("%w: Agent Task Workflow projection identity is invalid", application.ErrAgentExecutionPolicyDenied)
	}
	task, err := s.store.GetTask(ctx, projection.TaskUUID)
	if err != nil {
		return nil, fmt.Errorf("get Agent Task Workflow projection policy: %w", err)
	}
	run, err := s.store.GetRun(ctx, request.RunUUID)
	if err != nil {
		return nil, fmt.Errorf("get Agent Run Workflow projection policy: %w", err)
	}
	if task == nil || run == nil || run.TaskUUID != projection.TaskUUID || run.RuntimeID != request.RuntimeID || run.Mode != request.Mode ||
		!workflowProjectionMatchesRunStatus(projection.Status, run.Status) {
		return nil, fmt.Errorf("%w: Agent Task Workflow projection binding is unavailable", application.ErrAgentExecutionPolicyDenied)
	}
	if _, err := s.store.ProjectTaskWorkflowState(ctx, projection); err != nil {
		if errors.Is(err, application.ErrAgentWorkflowProjectionConflict) {
			return nil, fmt.Errorf("%w: %v", application.ErrAgentWorkflowProjectionConflict, err)
		}
		return nil, fmt.Errorf("project Agent Task Workflow state: %w", err)
	}
	current, err := s.store.GetTask(ctx, projection.TaskUUID)
	if err != nil {
		return nil, fmt.Errorf("reload Agent Task Workflow projection: %w", err)
	}
	if current == nil || current.Workflow == nil || !sameAgentTaskWorkflowProjectionV1(*current.Workflow, projection) {
		return nil, fmt.Errorf("%w: projected Agent Task state did not converge", application.ErrAgentWorkflowProjectionConflict)
	}
	result := *current.Workflow
	return &result, nil
}

func (s *PersistentAgentTaskWorkflowProjectionServiceV1) ListProjectionSnapshots(ctx context.Context, afterTaskUUID string, limit int) (*application.AgentTaskWorkflowProjectionPageV1, error) {
	afterTaskUUID = strings.TrimSpace(afterTaskUUID)
	if limit < 1 || limit > 1000 {
		return nil, fmt.Errorf("%w: Agent Task Workflow projection page size is invalid", application.ErrAgentExecutionPolicyDenied)
	}
	tasks, err := s.store.ListTaskWorkflowProjectionSnapshots(ctx, "dipole-agent", "shadow", afterTaskUUID, limit)
	if err != nil {
		return nil, fmt.Errorf("list Agent Task Workflow projection snapshots: %w", err)
	}
	page := &application.AgentTaskWorkflowProjectionPageV1{Tasks: tasks}
	if len(tasks) == limit {
		page.NextCursor = tasks[len(tasks)-1].TaskUUID
	}
	return page, nil
}

func workflowProjectionMatchesRunStatus(workflow application.AgentTaskWorkflowStatusV1, run application.AgentRunStatusV1) bool {
	switch run {
	case application.AgentRunStatusRunning:
		return true
	case application.AgentRunStatusCompleted:
		return workflow == application.AgentTaskWorkflowStatusCompleted
	case application.AgentRunStatusFailed:
		return workflow == application.AgentTaskWorkflowStatusFailed
	case application.AgentRunStatusCancelled:
		return workflow == application.AgentTaskWorkflowStatusCancelled
	default:
		return false
	}
}

func sameAgentTaskWorkflowProjectionV1(left, right application.AgentTaskWorkflowProjectionV1) bool {
	return left.TaskUUID == right.TaskUUID && left.WorkflowID == right.WorkflowID && left.RunID == right.RunID &&
		left.Status == right.Status && left.Revision == right.Revision
}
