package agentapplication_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JekYUlll/Dipole/internal/application"
	agentapplication "github.com/JekYUlll/Dipole/internal/services/agent/application"
)

func TestPersistentAgentTaskWorkflowProjectionServiceBindsTaskRunAndRevision(t *testing.T) {
	store := &agentPolicyStoreStub{
		tasks: map[string]*application.AgentTaskV1{"TASK-1": {TaskUUID: "TASK-1", PrincipalUUID: "U100", Status: application.AgentTaskStatusRunning}},
		runs:  map[string]*application.AgentRunV1{"RUN-1": {RunUUID: "RUN-1", TaskUUID: "TASK-1", RuntimeID: "dipole-agent", Mode: "shadow", Status: application.AgentRunStatusRunning}},
	}
	service, err := agentapplication.NewPersistentAgentTaskWorkflowProjectionServiceV1(store)
	if err != nil {
		t.Fatalf("new projection service: %v", err)
	}
	request := application.AgentTaskWorkflowProjectionRequestV1{
		Projection: application.AgentTaskWorkflowProjectionV1{
			TaskUUID: "TASK-1", WorkflowID: "dipole-agent-task/TASK-1", RunID: "temporal-run-1",
			Status: application.AgentTaskWorkflowStatusWaitingApproval, Revision: 2,
		},
		RunUUID: "RUN-1", RuntimeID: "dipole-agent", Mode: "shadow",
	}
	for attempt := 0; attempt < 2; attempt++ {
		projection, projectErr := service.Project(context.Background(), request)
		if projectErr != nil || projection.Status != application.AgentTaskWorkflowStatusWaitingApproval || projection.Revision != 2 {
			t.Fatalf("project attempt %d: projection=%+v err=%v", attempt, projection, projectErr)
		}
	}
}

func TestPersistentAgentTaskWorkflowProjectionServiceRejectsBindingAndTerminalDrift(t *testing.T) {
	store := &agentPolicyStoreStub{
		tasks: map[string]*application.AgentTaskV1{"TASK-1": {TaskUUID: "TASK-1", PrincipalUUID: "U100", Status: application.AgentTaskStatusRunning}},
		runs:  map[string]*application.AgentRunV1{"RUN-1": {RunUUID: "RUN-1", TaskUUID: "TASK-1", RuntimeID: "dipole-agent", Mode: "shadow", Status: application.AgentRunStatusCompleted}},
	}
	service, _ := agentapplication.NewPersistentAgentTaskWorkflowProjectionServiceV1(store)
	base := application.AgentTaskWorkflowProjectionRequestV1{
		Projection: application.AgentTaskWorkflowProjectionV1{
			TaskUUID: "TASK-1", WorkflowID: "dipole-agent-task/TASK-1", RunID: "temporal-run-1",
			Status: application.AgentTaskWorkflowStatusCompleted, Revision: 3,
		},
		RunUUID: "RUN-1", RuntimeID: "dipole-agent", Mode: "shadow",
	}
	for _, mutate := range []func(*application.AgentTaskWorkflowProjectionRequestV1){
		func(request *application.AgentTaskWorkflowProjectionRequestV1) {
			request.Projection.WorkflowID = "other-workflow"
		},
		func(request *application.AgentTaskWorkflowProjectionRequestV1) { request.RuntimeID = "forged-runtime" },
		func(request *application.AgentTaskWorkflowProjectionRequestV1) {
			request.Projection.Status = application.AgentTaskWorkflowStatusRunning
		},
	} {
		request := base
		mutate(&request)
		if _, err := service.Project(context.Background(), request); !errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
			t.Fatalf("projection drift error = %v", err)
		}
	}
}

func TestPersistentAgentTaskWorkflowProjectionListsFixedShadowCohort(t *testing.T) {
	store := &agentPolicyStoreStub{tasks: map[string]*application.AgentTaskV1{
		"TASK-1": {TaskUUID: "TASK-1", Workflow: &application.AgentTaskWorkflowProjectionV1{TaskUUID: "TASK-1", WorkflowID: "dipole-agent-task/TASK-1"}},
	}}
	service, _ := agentapplication.NewPersistentAgentTaskWorkflowProjectionServiceV1(store)
	page, err := service.ListProjectionSnapshots(context.Background(), "", 10)
	if err != nil || len(page.Tasks) != 1 || page.Tasks[0].TaskUUID != "TASK-1" || page.NextCursor != "" {
		t.Fatalf("projection page: %+v err=%v", page, err)
	}
	if _, err := service.ListProjectionSnapshots(context.Background(), "", 0); !errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
		t.Fatalf("invalid page size: %v", err)
	}
}
