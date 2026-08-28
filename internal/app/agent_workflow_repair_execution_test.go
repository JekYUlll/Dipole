package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

type repairExecutionStoreStubV1 struct {
	execution *application.AgentWorkflowRepairExecutionV1
}

func (s *repairExecutionStoreStubV1) CreateWorkflowRepairExecution(_ context.Context, execution application.AgentWorkflowRepairExecutionV1) (bool, error) {
	if s.execution != nil {
		if *s.execution != execution {
			return false, application.ErrAgentPolicyInvalid
		}
		return false, nil
	}
	copy := execution
	s.execution = &copy
	return true, nil
}

func (s *repairExecutionStoreStubV1) GetWorkflowRepairExecution(_ context.Context, _ string) (*application.AgentWorkflowRepairExecutionV1, error) {
	if s.execution == nil {
		return nil, nil
	}
	copy := *s.execution
	return &copy, nil
}

func TestPersistentAgentWorkflowRepairPrepareRequiresApprovedQuorumAndIsIdempotent(t *testing.T) {
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	policies := &agentPolicyStoreStub{tasks: map[string]*application.AgentTaskV1{"TASK-1": {TaskUUID: "TASK-1"}}}
	repairs := newRepairAuditStoreStubV1(now)
	proposal, err := application.NewAgentWorkflowRepairProposalV1("PROPOSER", repairProposalRequestV1(now))
	if err != nil {
		t.Fatal(err)
	}
	proposal.Status = application.AgentWorkflowRepairStatusApproved
	repairs.proposals[proposal.ProposalUUID] = proposal
	repairs.decisions[proposal.ProposalUUID+"\x00APPROVER-1"] = application.AgentWorkflowRepairDecisionRecordV1{ProposalUUID: proposal.ProposalUUID, ApproverUUID: "APPROVER-1", Decision: application.AgentWorkflowRepairDecisionApproved}
	repairs.decisions[proposal.ProposalUUID+"\x00APPROVER-2"] = application.AgentWorkflowRepairDecisionRecordV1{ProposalUUID: proposal.ProposalUUID, ApproverUUID: "APPROVER-2", Decision: application.AgentWorkflowRepairDecisionApproved}
	executions := &repairExecutionStoreStubV1{}
	service, err := newPersistentAgentWorkflowRepairPrepareServiceV1(policies, repairs, executions, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	execution := application.AgentWorkflowRepairExecutionV1{
		ExecutionUUID: "repair-execution:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		PlanID:        "repair-plan:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ProposalUUID:  proposal.ProposalUUID, TaskUUID: "TASK-1", ExecutorUUID: "EXECUTOR", ExecutorGrantVersion: 1,
		ExpectedCurrentSHA256: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		TargetSHA256:          "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd", Status: application.AgentWorkflowRepairExecutionStatusPrepared,
	}
	for attempt := 0; attempt < 2; attempt++ {
		prepared, prepareErr := service.Prepare(context.Background(), application.AgentWorkflowRepairPrepareRequestV1{Execution: execution})
		if prepareErr != nil || prepared == nil || prepared.ExecutionUUID != execution.ExecutionUUID {
			t.Fatalf("prepare attempt %d: %+v err=%v", attempt, prepared, prepareErr)
		}
	}
}

func TestPersistentAgentWorkflowRepairPrepareRejectsUnapprovedOrMismatchedExecution(t *testing.T) {
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	policies := &agentPolicyStoreStub{tasks: map[string]*application.AgentTaskV1{"TASK-1": {TaskUUID: "TASK-1"}}}
	repairs := newRepairAuditStoreStubV1(now)
	proposal, _ := application.NewAgentWorkflowRepairProposalV1("PROPOSER", repairProposalRequestV1(now))
	repairs.proposals[proposal.ProposalUUID] = proposal
	service, _ := newPersistentAgentWorkflowRepairPrepareServiceV1(policies, repairs, &repairExecutionStoreStubV1{}, func() time.Time { return now })
	execution := application.AgentWorkflowRepairExecutionV1{ExecutionUUID: "repair-execution:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", PlanID: "repair-plan:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ProposalUUID: proposal.ProposalUUID, TaskUUID: "TASK-1", ExecutorUUID: "EXECUTOR", ExecutorGrantVersion: 1, TargetSHA256: "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd", Status: application.AgentWorkflowRepairExecutionStatusPrepared}
	if _, err := service.Prepare(context.Background(), application.AgentWorkflowRepairPrepareRequestV1{Execution: execution}); !errors.Is(err, application.ErrAgentWorkflowRepairDenied) {
		t.Fatalf("unapproved proposal error = %v", err)
	}
}
