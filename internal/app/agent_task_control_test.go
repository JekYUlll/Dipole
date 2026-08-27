package app

import (
	"context"
	"errors"
	"testing"

	"github.com/JekYUlll/Dipole/internal/application"
)

func TestPersistentAgentTaskControlAuthorizerUsesStoredPrincipal(t *testing.T) {
	store := &agentPolicyStoreStub{tasks: map[string]*application.AgentTaskV1{
		"TASK-1": {TaskUUID: "TASK-1", PrincipalUUID: "U100", Status: application.AgentTaskStatusWaitingApproval},
	}}
	authorizer, err := NewPersistentAgentTaskControlAuthorizerV1(store)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	authorization, err := authorizer.AuthorizeTaskControl(context.Background(), " TASK-1 ", " U100 ")
	if err != nil || authorization.TaskUUID != "TASK-1" || authorization.Status != application.AgentTaskStatusWaitingApproval {
		t.Fatalf("unexpected authorization: authorization=%+v err=%v", authorization, err)
	}
}

func TestPersistentAgentTaskControlAuthorizerHidesMissingAndForeignTasks(t *testing.T) {
	store := &agentPolicyStoreStub{tasks: map[string]*application.AgentTaskV1{
		"TASK-1": {TaskUUID: "TASK-1", PrincipalUUID: "U100", Status: application.AgentTaskStatusRunning},
	}}
	authorizer, _ := NewPersistentAgentTaskControlAuthorizerV1(store)

	for _, input := range [][2]string{{"TASK-1", "U999"}, {"TASK-404", "U100"}, {"", "U100"}} {
		if _, err := authorizer.AuthorizeTaskControl(context.Background(), input[0], input[1]); !errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
			t.Fatalf("AuthorizeTaskControl(%q, %q) error = %v", input[0], input[1], err)
		}
	}
}
