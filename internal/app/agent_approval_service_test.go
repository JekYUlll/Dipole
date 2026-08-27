package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

func TestPersistentAgentApprovalServiceBindsRequestAndResolutionToTaskPrincipal(t *testing.T) {
	now := time.Date(2026, 8, 27, 14, 0, 0, 0, time.UTC)
	store := &agentPolicyStoreStub{
		tasks: map[string]*application.AgentTaskV1{"TASK-1": {TaskUUID: "TASK-1", PrincipalUUID: "U100"}},
		runs: map[string]*application.AgentRunV1{"RUN-1": {
			RunUUID: "RUN-1", TaskUUID: "TASK-1", RuntimeID: "dipole-agent", Mode: "shadow", Status: application.AgentRunStatusRunning,
		}},
	}
	service := &PersistentAgentApprovalServiceV1{store: store, now: func() time.Time { return now }}
	scope := application.AgentResourceScopeV1{ResourceType: "conversation", ResourceID: "G1", Actions: []string{"write"}}
	scopeHash, _ := application.AgentResourceScopeSHA256V1(scope)
	approval := application.AgentApprovalV1{
		ApprovalUUID: "APR-1", TaskUUID: "TASK-1", CapabilityID: "message.bulk.send", ResourceScope: scope,
		ScopeSHA256: scopeHash, ArgumentsSHA256: strings.Repeat("a", 64), NonceSHA256: strings.Repeat("b", 64),
		Status: application.AgentApprovalStatusPending, ExpiresAt: now.Add(time.Hour),
	}
	request := application.AgentApprovalRequestV1{TaskUUID: "TASK-1", RunUUID: "RUN-1", RuntimeID: "dipole-agent", Mode: "shadow", Approval: approval}
	if first, err := service.Request(context.Background(), request); err != nil || first.Status != application.AgentApprovalStatusPending {
		t.Fatalf("request Approval: approval=%+v err=%v", first, err)
	}
	if replay, err := service.Request(context.Background(), request); err != nil || replay.ApprovalUUID != "APR-1" {
		t.Fatalf("replay Approval request: approval=%+v err=%v", replay, err)
	}
	resolution := application.AgentApprovalResolutionV1{
		TaskUUID: "TASK-1", RunUUID: "RUN-1", RuntimeID: "dipole-agent", Mode: "shadow",
		ApprovalUUID: "APR-1", ActorUUID: "U100", Decision: application.AgentApprovalDecisionApproved,
	}
	if resolved, err := service.Resolve(context.Background(), resolution); err != nil || resolved.Status != application.AgentApprovalStatusApproved || resolved.ApprovedByUUID != "U100" {
		t.Fatalf("resolve Approval: approval=%+v err=%v", resolved, err)
	}
	if replay, err := service.Resolve(context.Background(), resolution); err != nil || replay.Status != application.AgentApprovalStatusApproved {
		t.Fatalf("replay Approval resolution: approval=%+v err=%v", replay, err)
	}
	deniedApproval := approval
	deniedApproval.ApprovalUUID, deniedApproval.NonceSHA256 = "APR-2", strings.Repeat("c", 64)
	deniedRequest := request
	deniedRequest.Approval = deniedApproval
	if _, err := service.Request(context.Background(), deniedRequest); err != nil {
		t.Fatalf("request denied Approval: %v", err)
	}
	deniedResolution := resolution
	deniedResolution.ApprovalUUID, deniedResolution.Decision = "APR-2", application.AgentApprovalDecisionDenied
	if denied, err := service.Resolve(context.Background(), deniedResolution); err != nil || denied.Status != application.AgentApprovalStatusRevoked {
		t.Fatalf("deny Approval: approval=%+v err=%v", denied, err)
	}
	if replay, err := service.Resolve(context.Background(), deniedResolution); err != nil || replay.Status != application.AgentApprovalStatusRevoked {
		t.Fatalf("replay denied Approval: approval=%+v err=%v", replay, err)
	}
}

func TestPersistentAgentApprovalServiceRejectsForgedActorAndCrossTaskApproval(t *testing.T) {
	now := time.Now().UTC()
	store := &agentPolicyStoreStub{
		tasks:     map[string]*application.AgentTaskV1{"TASK-1": {TaskUUID: "TASK-1", PrincipalUUID: "U100"}},
		runs:      map[string]*application.AgentRunV1{"RUN-1": {RunUUID: "RUN-1", TaskUUID: "TASK-1", RuntimeID: "dipole-agent", Mode: "shadow", Status: application.AgentRunStatusRunning}},
		approvals: map[string]*application.AgentApprovalV1{"APR-2": {ApprovalUUID: "APR-2", TaskUUID: "TASK-2", Status: application.AgentApprovalStatusPending, ExpiresAt: now.Add(time.Hour)}},
	}
	service := &PersistentAgentApprovalServiceV1{store: store, now: func() time.Time { return now }}
	for _, resolution := range []application.AgentApprovalResolutionV1{
		{TaskUUID: "TASK-1", RunUUID: "RUN-1", RuntimeID: "dipole-agent", Mode: "shadow", ApprovalUUID: "APR-2", ActorUUID: "U100", Decision: application.AgentApprovalDecisionDenied},
		{TaskUUID: "TASK-1", RunUUID: "RUN-1", RuntimeID: "dipole-agent", Mode: "shadow", ApprovalUUID: "APR-2", ActorUUID: "U999", Decision: application.AgentApprovalDecisionApproved},
	} {
		if _, err := service.Resolve(context.Background(), resolution); !errors.Is(err, application.ErrAgentApprovalDenied) {
			t.Fatalf("forged resolution error = %v", err)
		}
	}
}
