package app

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

func TestWorkflowRepairAuditRequiresGrantAndTwoDistinctApprovers(t *testing.T) {
	now := time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)
	policies := &agentPolicyStoreStub{tasks: map[string]*application.AgentTaskV1{"TASK-1": {
		TaskUUID: "TASK-1", Workflow: &application.AgentTaskWorkflowProjectionV1{TaskUUID: "TASK-1", WorkflowID: "dipole-agent-task/TASK-1", RunID: "WR-1", Status: application.AgentTaskWorkflowStatusRunning, Revision: 2},
	}}}
	repairs := newRepairAuditStoreStubV1(now)
	service, _ := NewPersistentAgentWorkflowRepairAuditServiceV1WithClock(policies, repairs, func() time.Time { return now })
	request := repairProposalRequestV1(now)
	if _, err := service.Propose(context.Background(), "UNGRANTED", request); !errors.Is(err, application.ErrAgentWorkflowRepairDenied) {
		t.Fatalf("ungranted propose: %v", err)
	}
	proposal, err := service.Propose(context.Background(), "PROPOSER", request)
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	if _, err := service.Decide(context.Background(), "PROPOSER", proposal.ProposalUUID, application.AgentWorkflowRepairDecisionApproved); !errors.Is(err, application.ErrAgentWorkflowRepairDenied) {
		t.Fatalf("self approval: %v", err)
	}
	first, err := service.Decide(context.Background(), "APPROVER-1", proposal.ProposalUUID, application.AgentWorkflowRepairDecisionApproved)
	if err != nil || first.Status != application.AgentWorkflowRepairStatusProposed {
		t.Fatalf("first approval: %+v err=%v", first, err)
	}
	final, err := service.Decide(context.Background(), "APPROVER-2", proposal.ProposalUUID, application.AgentWorkflowRepairDecisionApproved)
	if err != nil || final.Status != application.AgentWorkflowRepairStatusApproved {
		t.Fatalf("second approval: %+v err=%v", final, err)
	}
}

func TestWorkflowRepairAuditRejectsConflictingReplayAndRejectionWins(t *testing.T) {
	now := time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)
	policies := &agentPolicyStoreStub{tasks: map[string]*application.AgentTaskV1{"TASK-1": {TaskUUID: "TASK-1", Workflow: &application.AgentTaskWorkflowProjectionV1{TaskUUID: "TASK-1", WorkflowID: "dipole-agent-task/TASK-1", RunID: "WR-1", Status: application.AgentTaskWorkflowStatusRunning, Revision: 2}}}}
	repairs := newRepairAuditStoreStubV1(now)
	service, _ := NewPersistentAgentWorkflowRepairAuditServiceV1WithClock(policies, repairs, func() time.Time { return now })
	proposal, _ := service.Propose(context.Background(), "PROPOSER", repairProposalRequestV1(now))
	if _, err := service.Decide(context.Background(), "APPROVER-1", proposal.ProposalUUID, application.AgentWorkflowRepairDecisionRejected); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Decide(context.Background(), "APPROVER-1", proposal.ProposalUUID, application.AgentWorkflowRepairDecisionApproved); !errors.Is(err, application.ErrAgentWorkflowRepairConflict) {
		t.Fatalf("conflicting replay: %v", err)
	}
	final, _ := service.Get(context.Background(), "PROPOSER", proposal.ProposalUUID)
	if final.Status != application.AgentWorkflowRepairStatusRejected {
		t.Fatalf("status = %s", final.Status)
	}
}

func repairProposalRequestV1(now time.Time) application.AgentWorkflowRepairProposalRequestV1 {
	return application.AgentWorkflowRepairProposalRequestV1{TaskUUID: "TASK-1", Outcome: application.AgentWorkflowRepairOutcomeStale, TicketRef: "INC-1", Reason: "verified drift",
		Projected: &application.AgentWorkflowEvidenceV1{WorkflowID: "dipole-agent-task/TASK-1", WorkflowRunID: "WR-1", Status: "running", Revision: 2},
		Temporal:  application.AgentWorkflowEvidenceV1{WorkflowID: "dipole-agent-task/TASK-1", WorkflowRunID: "WR-1", Status: "completed", Revision: 3}, ProposedAt: now, ExpiresAt: now.Add(time.Hour)}
}

type repairAuditStoreStubV1 struct {
	mu        sync.Mutex
	grants    map[string]*application.AgentWorkflowRepairOperatorGrantV1
	proposals map[string]*application.AgentWorkflowRepairProposalV1
	decisions map[string]application.AgentWorkflowRepairDecisionRecordV1
}

func newRepairAuditStoreStubV1(now time.Time) *repairAuditStoreStubV1 {
	grants := map[string]*application.AgentWorkflowRepairOperatorGrantV1{}
	for _, id := range []string{"PROPOSER", "APPROVER-1", "APPROVER-2"} {
		grants[id] = &application.AgentWorkflowRepairOperatorGrantV1{UserUUID: id, CanPropose: id == "PROPOSER", CanApprove: id != "PROPOSER", ValidFrom: now.Add(-time.Hour)}
	}
	return &repairAuditStoreStubV1{grants: grants, proposals: map[string]*application.AgentWorkflowRepairProposalV1{}, decisions: map[string]application.AgentWorkflowRepairDecisionRecordV1{}}
}
func (s *repairAuditStoreStubV1) GetWorkflowRepairOperatorGrant(_ context.Context, id string) (*application.AgentWorkflowRepairOperatorGrantV1, error) {
	return s.grants[id], nil
}
func (s *repairAuditStoreStubV1) CreateWorkflowRepairProposal(_ context.Context, p application.AgentWorkflowRepairProposalV1) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.proposals[p.ProposalUUID]; ok {
		return false, nil
	}
	c := p
	s.proposals[p.ProposalUUID] = &c
	return true, nil
}
func (s *repairAuditStoreStubV1) GetWorkflowRepairProposal(_ context.Context, id string) (*application.AgentWorkflowRepairProposalV1, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.proposals[id]
	if p == nil {
		return nil, nil
	}
	c := *p
	return &c, nil
}
func (s *repairAuditStoreStubV1) RecordWorkflowRepairDecision(_ context.Context, d application.AgentWorkflowRepairDecisionRecordV1) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := d.ProposalUUID + "\x00" + d.ApproverUUID
	if old, ok := s.decisions[k]; ok {
		if old.Decision != d.Decision {
			return false, application.ErrAgentWorkflowRepairConflict
		}
		return false, nil
	}
	p := s.proposals[d.ProposalUUID]
	if p == nil || p.Status != application.AgentWorkflowRepairStatusProposed || p.ProposerUUID == d.ApproverUUID {
		return false, application.ErrAgentWorkflowRepairDenied
	}
	s.decisions[k] = d
	return true, nil
}
func (s *repairAuditStoreStubV1) GetWorkflowRepairDecision(_ context.Context, p, a string) (*application.AgentWorkflowRepairDecisionRecordV1, error) {
	d, ok := s.decisions[p+"\x00"+a]
	if !ok {
		return nil, nil
	}
	return &d, nil
}
func (s *repairAuditStoreStubV1) CountWorkflowRepairDecisions(_ context.Context, p string) (application.AgentWorkflowRepairDecisionCountsV1, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var c application.AgentWorkflowRepairDecisionCountsV1
	for _, d := range s.decisions {
		if d.ProposalUUID == p {
			if d.Decision == application.AgentWorkflowRepairDecisionApproved {
				c.Approved++
			} else {
				c.Rejected++
			}
		}
	}
	return c, nil
}
func (s *repairAuditStoreStubV1) FinalizeWorkflowRepairProposal(ctx context.Context, p string) error {
	c, _ := s.CountWorkflowRepairDecisions(ctx, p)
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.Rejected > 0 {
		s.proposals[p].Status = application.AgentWorkflowRepairStatusRejected
	} else if c.Approved >= 2 {
		s.proposals[p].Status = application.AgentWorkflowRepairStatusApproved
	}
	return nil
}
