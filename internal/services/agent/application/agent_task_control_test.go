package agentapplication_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	agentapplication "github.com/JekYUlll/Dipole/internal/services/agent/application"
)

func TestPersistentAgentTaskControlAuthorizerUsesStoredPrincipal(t *testing.T) {
	store := &agentPolicyStoreStub{tasks: map[string]*application.AgentTaskV1{
		"TASK-1": {TaskUUID: "TASK-1", PrincipalUUID: "U100", Status: application.AgentTaskStatusWaitingApproval},
	}}
	mcpRunUUID, err := application.AgentRunUUIDV1("TASK-1", "dipole-agent", "shadow")
	if err != nil {
		t.Fatalf("derive MCP Run UUID: %v", err)
	}
	store.runs = map[string]*application.AgentRunV1{mcpRunUUID: {
		RunUUID: mcpRunUUID, TaskUUID: "TASK-1", RuntimeID: "dipole-agent", Mode: "shadow", Status: application.AgentRunStatusRunning,
	}}
	authorizer, err := agentapplication.NewPersistentAgentTaskControlAuthorizerV1(store)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	authorization, err := authorizer.AuthorizeTaskControl(context.Background(), " TASK-1 ", " U100 ")
	if err != nil || authorization.TaskUUID != "TASK-1" || authorization.Status != application.AgentTaskStatusWaitingApproval || authorization.MCPRunUUID != mcpRunUUID {
		t.Fatalf("unexpected authorization: authorization=%+v err=%v", authorization, err)
	}
}

func TestPersistentAgentTaskControlAuthorizerOmitsTerminalMCPRun(t *testing.T) {
	mcpRunUUID, err := application.AgentRunUUIDV1("TASK-1", "dipole-agent", "shadow")
	if err != nil {
		t.Fatalf("derive MCP Run UUID: %v", err)
	}
	store := &agentPolicyStoreStub{
		tasks: map[string]*application.AgentTaskV1{"TASK-1": {TaskUUID: "TASK-1", PrincipalUUID: "U100", Status: application.AgentTaskStatusCompleted}},
		runs:  map[string]*application.AgentRunV1{mcpRunUUID: {RunUUID: mcpRunUUID, TaskUUID: "TASK-1", RuntimeID: "dipole-agent", Mode: "shadow", Status: application.AgentRunStatusCompleted}},
	}
	authorizer, _ := agentapplication.NewPersistentAgentTaskControlAuthorizerV1(store)

	authorization, err := authorizer.AuthorizeTaskControl(context.Background(), "TASK-1", "U100")
	if err != nil || authorization.MCPRunUUID != "" {
		t.Fatalf("terminal MCP Run must be omitted: authorization=%+v err=%v", authorization, err)
	}
}

func TestPersistentAgentTaskControlAuthorizerListsOwnerInboxPage(t *testing.T) {
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	store := &agentTaskInboxStoreStub{owned: []application.AgentTaskV1{
		agentTaskInboxFixture("TASK-3", "U100", now.Add(-time.Minute), application.AgentTaskStatusRunning, application.AgentTaskWorkflowStatusWaitingApproval, 4, "审批这条摘要"),
		agentTaskInboxFixture("TASK-2", "U100", now.Add(-2*time.Minute), application.AgentTaskStatusRunning, application.AgentTaskWorkflowStatusWaitingInput, 3, "补充输入"),
		agentTaskInboxFixture("TASK-1", "U100", now.Add(-3*time.Minute), application.AgentTaskStatusCompleted, "", 2, "已经完成"),
	}}
	authorizer, err := agentapplication.NewPersistentAgentTaskControlAuthorizerV1WithNow(store, func() time.Time { return now })
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}
	page, err := authorizer.ListOwnedTasks(context.Background(), application.AgentTaskOwnerInboxListRequestV1{TenantID: "dipole", PrincipalUUID: "U100", Limit: 2})
	if err != nil || len(page.Tasks) != 2 || page.NextTaskUUID != "TASK-2" || !page.NextUpdatedAt.Equal(now.Add(-2*time.Minute)) {
		t.Fatalf("owner inbox page=%+v err=%v", page, err)
	}
	if page.Tasks[0].TaskUUID != "TASK-3" || page.Tasks[0].Status != "waiting_approval" || page.Tasks[0].PendingKind != application.AgentTaskInboxPendingApproval ||
		page.Tasks[0].Revision != 4 || page.Tasks[1].PendingKind != application.AgentTaskInboxPendingInput {
		t.Fatalf("public inbox projection=%+v", page.Tasks)
	}
	if store.listRequest.Limit != 3 || !store.listRequest.AfterUpdatedAt.Equal(now) {
		t.Fatalf("store request = %+v", store.listRequest)
	}
}

func TestPersistentAgentTaskControlAuthorizerHidesForeignInboxRows(t *testing.T) {
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	store := &agentTaskInboxStoreStub{owned: []application.AgentTaskV1{
		agentTaskInboxFixture("TASK-1", "U999", now.Add(-time.Minute), application.AgentTaskStatusRunning, "", 1, "别人的任务"),
	}}
	authorizer, _ := agentapplication.NewPersistentAgentTaskControlAuthorizerV1WithNow(store, func() time.Time { return now })
	if _, err := authorizer.ListOwnedTasks(context.Background(), application.AgentTaskOwnerInboxListRequestV1{TenantID: "dipole", PrincipalUUID: "U100", Limit: 20}); !errors.Is(err, application.ErrAgentTaskInboxDenied) {
		t.Fatalf("foreign inbox error = %v", err)
	}
	if _, err := authorizer.ListOwnedTasks(context.Background(), application.AgentTaskOwnerInboxListRequestV1{TenantID: "dipole", PrincipalUUID: "", Limit: 20}); !errors.Is(err, application.ErrAgentTaskInboxInvalid) {
		t.Fatalf("blank principal error = %v", err)
	}
}

func TestPersistentAgentTaskControlAuthorizerHidesMissingAndForeignTasks(t *testing.T) {
	store := &agentPolicyStoreStub{tasks: map[string]*application.AgentTaskV1{
		"TASK-1": {TaskUUID: "TASK-1", PrincipalUUID: "U100", Status: application.AgentTaskStatusRunning},
	}}
	authorizer, _ := agentapplication.NewPersistentAgentTaskControlAuthorizerV1(store)

	for _, input := range [][2]string{{"TASK-1", "U999"}, {"TASK-404", "U100"}, {"", "U100"}} {
		if _, err := authorizer.AuthorizeTaskControl(context.Background(), input[0], input[1]); !errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
			t.Fatalf("AuthorizeTaskControl(%q, %q) error = %v", input[0], input[1], err)
		}
	}
}

type agentTaskInboxStoreStub struct {
	agentPolicyStoreStub
	owned       []application.AgentTaskV1
	listRequest application.AgentTaskOwnerInboxListRequestV1
}

func (s *agentTaskInboxStoreStub) ListOwnedTasks(_ context.Context, request application.AgentTaskOwnerInboxListRequestV1) ([]application.AgentTaskV1, error) {
	s.listRequest = request
	return append([]application.AgentTaskV1(nil), s.owned...), nil
}

func agentTaskInboxFixture(taskUUID, principal string, updatedAt time.Time, status application.AgentTaskStatusV1, workflowStatus application.AgentTaskWorkflowStatusV1, revision uint64, goal string) application.AgentTaskV1 {
	task := application.AgentTaskV1{
		TaskUUID: taskUUID, DefinitionUUID: "DEF-1", DefinitionVersion: 1, TenantID: "dipole",
		PrincipalUUID: principal, AgentUUID: "UAI", Status: status, TriggerType: "manual", TriggerRef: "inbox",
		Goal: goal, UpdatedAt: updatedAt,
	}
	if workflowStatus != "" {
		task.Workflow = &application.AgentTaskWorkflowProjectionV1{
			TaskUUID: taskUUID, WorkflowID: "wf/" + taskUUID, RunID: "RUN-1", Status: workflowStatus, Revision: revision, UpdatedAt: updatedAt,
		}
	}
	return task
}
