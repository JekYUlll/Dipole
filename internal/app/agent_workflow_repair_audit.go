package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

type PersistentAgentWorkflowRepairAuditServiceV1 struct {
	policies application.AgentPolicyStoreV1
	repairs  application.AgentWorkflowRepairAuditStoreV1
	now      func() time.Time
}

func NewPersistentAgentWorkflowRepairAuditServiceV1(policies application.AgentPolicyStoreV1, repairs application.AgentWorkflowRepairAuditStoreV1) (*PersistentAgentWorkflowRepairAuditServiceV1, error) {
	return newPersistentAgentWorkflowRepairAuditServiceV1(policies, repairs, time.Now)
}

func newPersistentAgentWorkflowRepairAuditServiceV1(policies application.AgentPolicyStoreV1, repairs application.AgentWorkflowRepairAuditStoreV1, now func() time.Time) (*PersistentAgentWorkflowRepairAuditServiceV1, error) {
	if policies == nil || repairs == nil || now == nil {
		return nil, errors.New("Agent policy, repair audit store, and clock are required")
	}
	return &PersistentAgentWorkflowRepairAuditServiceV1{policies: policies, repairs: repairs, now: now}, nil
}

func (s *PersistentAgentWorkflowRepairAuditServiceV1) Propose(ctx context.Context, operatorUUID string, request application.AgentWorkflowRepairProposalRequestV1) (*application.AgentWorkflowRepairProposalV1, error) {
	operatorUUID = strings.TrimSpace(operatorUUID)
	if err := s.authorize(ctx, operatorUUID, true); err != nil {
		return nil, err
	}
	task, err := s.policies.GetTask(ctx, strings.TrimSpace(request.TaskUUID))
	if err != nil {
		return nil, fmt.Errorf("get repair Task: %w", err)
	}
	if task == nil || !repairEvidenceMatchesTaskV1(task, request) {
		return nil, fmt.Errorf("%w: repair evidence does not match the current Task projection", application.ErrAgentWorkflowRepairConflict)
	}
	proposal, err := application.NewAgentWorkflowRepairProposalV1(operatorUUID, request)
	if err != nil {
		return nil, err
	}
	if proposal.ProposedAt.After(s.now().UTC().Add(time.Minute)) || !proposal.ExpiresAt.After(s.now().UTC()) {
		return nil, fmt.Errorf("%w: repair proposal timestamp is outside its active window", application.ErrAgentWorkflowRepairConflict)
	}
	if _, err := s.repairs.CreateWorkflowRepairProposal(ctx, *proposal); err != nil {
		return nil, err
	}
	return s.repairs.GetWorkflowRepairProposal(ctx, proposal.ProposalUUID)
}

func (s *PersistentAgentWorkflowRepairAuditServiceV1) Decide(ctx context.Context, operatorUUID, proposalUUID string, decision application.AgentWorkflowRepairDecisionV1) (*application.AgentWorkflowRepairProposalV1, error) {
	operatorUUID, proposalUUID = strings.TrimSpace(operatorUUID), strings.TrimSpace(proposalUUID)
	if err := s.authorize(ctx, operatorUUID, false); err != nil {
		return nil, err
	}
	if decision != application.AgentWorkflowRepairDecisionApproved && decision != application.AgentWorkflowRepairDecisionRejected {
		return nil, fmt.Errorf("%w: repair decision is invalid", application.ErrAgentWorkflowRepairConflict)
	}
	proposal, err := s.repairs.GetWorkflowRepairProposal(ctx, proposalUUID)
	if err != nil {
		return nil, err
	}
	if proposal == nil || proposal.ProposerUUID == operatorUUID {
		return nil, fmt.Errorf("%w: repair proposal is unavailable to this operator", application.ErrAgentWorkflowRepairDenied)
	}
	if _, err := s.repairs.RecordWorkflowRepairDecision(ctx, application.AgentWorkflowRepairDecisionRecordV1{
		ProposalUUID: proposalUUID, ApproverUUID: operatorUUID, Decision: decision, DecidedAt: s.now().UTC(),
	}); err != nil {
		return nil, err
	}
	if err := s.repairs.FinalizeWorkflowRepairProposal(ctx, proposalUUID); err != nil {
		return nil, err
	}
	return s.repairs.GetWorkflowRepairProposal(ctx, proposalUUID)
}

func (s *PersistentAgentWorkflowRepairAuditServiceV1) Get(ctx context.Context, operatorUUID, proposalUUID string) (*application.AgentWorkflowRepairProposalV1, error) {
	operatorUUID = strings.TrimSpace(operatorUUID)
	grant, err := s.repairs.GetWorkflowRepairOperatorGrant(ctx, operatorUUID)
	if err != nil {
		return nil, err
	}
	if grant == nil || !grant.Active(s.now().UTC()) || (!grant.CanPropose && !grant.CanApprove) {
		return nil, fmt.Errorf("%w: inactive repair operator", application.ErrAgentWorkflowRepairDenied)
	}
	proposal, err := s.repairs.GetWorkflowRepairProposal(ctx, strings.TrimSpace(proposalUUID))
	if err != nil {
		return nil, err
	}
	if proposal == nil {
		return nil, fmt.Errorf("%w: repair proposal is unavailable", application.ErrAgentWorkflowRepairDenied)
	}
	return proposal, nil
}

func (s *PersistentAgentWorkflowRepairAuditServiceV1) authorize(ctx context.Context, operatorUUID string, propose bool) error {
	grant, err := s.repairs.GetWorkflowRepairOperatorGrant(ctx, operatorUUID)
	if err != nil {
		return fmt.Errorf("get repair operator grant: %w", err)
	}
	if grant == nil || !grant.Active(s.now().UTC()) || (propose && !grant.CanPropose) || (!propose && !grant.CanApprove) {
		return fmt.Errorf("%w: inactive or insufficient repair operator grant", application.ErrAgentWorkflowRepairDenied)
	}
	return nil
}

func repairEvidenceMatchesTaskV1(task *application.AgentTaskV1, request application.AgentWorkflowRepairProposalRequestV1) bool {
	if task == nil || strings.TrimSpace(request.Temporal.WorkflowID) != "dipole-agent-task/"+task.TaskUUID {
		return false
	}
	if request.Outcome == application.AgentWorkflowRepairOutcomeMissing {
		return task.Workflow == nil && request.Projected == nil
	}
	if task.Workflow == nil || request.Projected == nil {
		return false
	}
	return task.Workflow.WorkflowID == request.Projected.WorkflowID && task.Workflow.RunID == request.Projected.WorkflowRunID &&
		string(task.Workflow.Status) == request.Projected.Status && task.Workflow.Revision == request.Projected.Revision
}
