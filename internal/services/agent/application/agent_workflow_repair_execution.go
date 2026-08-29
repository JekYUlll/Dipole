package agentapplication

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

type PersistentAgentWorkflowRepairPrepareServiceV1 struct {
	policies   application.AgentPolicyStoreV1
	repairs    application.AgentWorkflowRepairAuditStoreV1
	executions application.AgentWorkflowRepairExecutionStoreV1
	now        func() time.Time
}

var _ application.AgentWorkflowRepairPrepareServiceV1 = (*PersistentAgentWorkflowRepairPrepareServiceV1)(nil)

func NewPersistentAgentWorkflowRepairPrepareServiceV1(
	policies application.AgentPolicyStoreV1,
	repairs application.AgentWorkflowRepairAuditStoreV1,
	executions application.AgentWorkflowRepairExecutionStoreV1,
) (*PersistentAgentWorkflowRepairPrepareServiceV1, error) {
	return NewPersistentAgentWorkflowRepairPrepareServiceV1WithClock(policies, repairs, executions, time.Now)
}

// NewPersistentAgentWorkflowRepairPrepareServiceV1WithClock keeps repair tests deterministic.
func NewPersistentAgentWorkflowRepairPrepareServiceV1WithClock(
	policies application.AgentPolicyStoreV1,
	repairs application.AgentWorkflowRepairAuditStoreV1,
	executions application.AgentWorkflowRepairExecutionStoreV1,
	now func() time.Time,
) (*PersistentAgentWorkflowRepairPrepareServiceV1, error) {
	if policies == nil || repairs == nil || executions == nil || now == nil {
		return nil, fmt.Errorf("Agent policy, repair audit, execution store, and clock are required")
	}
	return &PersistentAgentWorkflowRepairPrepareServiceV1{policies: policies, repairs: repairs, executions: executions, now: now}, nil
}

// Prepare records an approved, short-lived execution intent. It deliberately does not mutate the projection.
func (s *PersistentAgentWorkflowRepairPrepareServiceV1) Prepare(ctx context.Context, request application.AgentWorkflowRepairPrepareRequestV1) (*application.AgentWorkflowRepairExecutionV1, error) {
	execution := request.Execution
	if err := execution.Validate(); err != nil {
		return nil, fmt.Errorf("%w: invalid repair execution: %v", application.ErrAgentWorkflowRepairDenied, err)
	}
	proposal, err := s.repairs.GetWorkflowRepairProposal(ctx, strings.TrimSpace(execution.ProposalUUID))
	if err != nil {
		return nil, fmt.Errorf("get repair proposal for preparation: %w", err)
	}
	now := s.now().UTC()
	if proposal == nil || proposal.Status != application.AgentWorkflowRepairStatusApproved ||
		!proposal.ExpiresAt.After(now) || proposal.TaskUUID != execution.TaskUUID ||
		proposal.ProposerUUID == execution.ExecutorUUID {
		return nil, fmt.Errorf("%w: approved repair proposal is unavailable for preparation", application.ErrAgentWorkflowRepairDenied)
	}
	counts, err := s.repairs.CountWorkflowRepairDecisions(ctx, proposal.ProposalUUID)
	if err != nil {
		return nil, fmt.Errorf("count repair approvals for preparation: %w", err)
	}
	if counts.Rejected != 0 || counts.Approved < uint64(proposal.RequiredApprovals) {
		return nil, fmt.Errorf("%w: repair approval quorum is unavailable", application.ErrAgentWorkflowRepairDenied)
	}
	task, err := s.policies.GetTask(ctx, execution.TaskUUID)
	if err != nil {
		return nil, fmt.Errorf("get repair task for preparation: %w", err)
	}
	if task == nil {
		return nil, fmt.Errorf("%w: repair task is unavailable", application.ErrAgentWorkflowRepairDenied)
	}
	if (proposal.Projected == nil) != (execution.ExpectedCurrentSHA256 == "") {
		return nil, fmt.Errorf("%w: expected current projection binding is invalid", application.ErrAgentWorkflowRepairConflict)
	}
	if _, err := s.executions.CreateWorkflowRepairExecution(ctx, execution); err != nil {
		return nil, fmt.Errorf("prepare repair execution: %w", err)
	}
	prepared, err := s.executions.GetWorkflowRepairExecution(ctx, execution.ExecutionUUID)
	if err != nil {
		return nil, fmt.Errorf("reload prepared repair execution: %w", err)
	}
	if prepared == nil {
		return nil, fmt.Errorf("%w: prepared repair execution did not converge", application.ErrAgentWorkflowRepairConflict)
	}
	return prepared, nil
}
