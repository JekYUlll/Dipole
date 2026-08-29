package agentapplication

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

const (
	repairFailurePreconditionV1 = "precondition_failed"
	repairFailureCommitV1       = "projection_commit_failed"
)

// PersistentAgentWorkflowRepairExecutorV1 is deliberately not exposed by a
// transport yet. It provides the application seam for a future operator-only
// command while keeping authorization and mutation in one controlled flow.
type PersistentAgentWorkflowRepairExecutorV1 struct {
	policies    application.AgentPolicyStoreV1
	repairs     application.AgentWorkflowRepairAuditStoreV1
	executions  application.AgentWorkflowRepairExecutionStoreV1
	transaction application.AgentWorkflowRepairTransactionalStoreV1
	now         func() time.Time
}

var _ application.AgentWorkflowRepairExecutorV1 = (*PersistentAgentWorkflowRepairExecutorV1)(nil)

func NewPersistentAgentWorkflowRepairExecutorV1(
	policies application.AgentPolicyStoreV1,
	repairs application.AgentWorkflowRepairAuditStoreV1,
	executions application.AgentWorkflowRepairExecutionStoreV1,
	transaction application.AgentWorkflowRepairTransactionalStoreV1,
) (*PersistentAgentWorkflowRepairExecutorV1, error) {
	return NewPersistentAgentWorkflowRepairExecutorV1WithClock(policies, repairs, executions, transaction, time.Now)
}

// NewPersistentAgentWorkflowRepairExecutorV1WithClock keeps repair tests deterministic.
func NewPersistentAgentWorkflowRepairExecutorV1WithClock(
	policies application.AgentPolicyStoreV1,
	repairs application.AgentWorkflowRepairAuditStoreV1,
	executions application.AgentWorkflowRepairExecutionStoreV1,
	transaction application.AgentWorkflowRepairTransactionalStoreV1,
	now func() time.Time,
) (*PersistentAgentWorkflowRepairExecutorV1, error) {
	if policies == nil || repairs == nil || executions == nil || transaction == nil || now == nil {
		return nil, fmt.Errorf("Agent repair executor dependencies and clock are required")
	}
	return &PersistentAgentWorkflowRepairExecutorV1{policies: policies, repairs: repairs, executions: executions, transaction: transaction, now: now}, nil
}

func (s *PersistentAgentWorkflowRepairExecutorV1) Execute(ctx context.Context, request application.AgentWorkflowRepairExecuteRequestV1) (*application.AgentWorkflowRepairExecutionV1, error) {
	executionUUID, executorUUID := strings.TrimSpace(request.ExecutionUUID), strings.TrimSpace(request.ExecutorUUID)
	if executionUUID == "" || executorUUID == "" {
		return nil, fmt.Errorf("%w: execution and executor are required", application.ErrAgentWorkflowRepairPrecondition)
	}
	execution, err := s.executions.GetWorkflowRepairExecution(ctx, executionUUID)
	if err != nil {
		return nil, fmt.Errorf("load Workflow repair execution: %w", err)
	}
	if execution == nil || execution.ExecutorUUID != executorUUID {
		return nil, fmt.Errorf("%w: execution is unavailable for executor", application.ErrAgentWorkflowRepairDenied)
	}
	if execution.Status == application.AgentWorkflowRepairExecutionStatusCommitted {
		return execution, nil
	}
	if execution.Status != application.AgentWorkflowRepairExecutionStatusPrepared {
		return nil, fmt.Errorf("%w: execution is not prepared", application.ErrAgentWorkflowRepairConflict)
	}
	if err := validateRollbackPayload(execution, request.Rollback); err != nil {
		return nil, err
	}
	grant, err := s.repairs.GetWorkflowRepairOperatorGrant(ctx, executorUUID)
	if err != nil {
		return nil, fmt.Errorf("load Workflow repair executor grant: %w", err)
	}
	now := s.now().UTC()
	if grant == nil || grant.UserUUID != executorUUID || grant.Version != execution.ExecutorGrantVersion || !grant.CanExecute || !grant.Active(now) {
		return nil, fmt.Errorf("%w: executor grant is unavailable", application.ErrAgentWorkflowRepairDenied)
	}
	claimed, err := s.executions.ClaimWorkflowRepairExecution(ctx, executionUUID, executorUUID, grant.Version, now)
	if err != nil {
		return nil, fmt.Errorf("claim Workflow repair execution: %w", err)
	}
	if !claimed {
		return nil, fmt.Errorf("%w: execution claim was lost", application.ErrAgentWorkflowRepairConflict)
	}
	execution.Status = application.AgentWorkflowRepairExecutionStatusExecuting
	task, err := s.policies.GetTask(ctx, execution.TaskUUID)
	if err != nil {
		return s.fail(ctx, execution, executorUUID, "task_lookup_failed", err)
	}
	if task == nil {
		return s.fail(ctx, execution, executorUUID, repairFailurePreconditionV1, fmt.Errorf("task is unavailable"))
	}
	precondition := application.AgentWorkflowRepairPreconditionV1{Execution: *execution, Grant: *grant, Current: task.Workflow, Target: request.Target, At: now}
	if err := precondition.Validate(); err != nil {
		return s.fail(ctx, execution, executorUUID, repairFailurePreconditionV1, err)
	}
	committed, err := s.transaction.CommitWorkflowRepairProjection(ctx, executionUUID, executorUUID, grant.Version, task.Workflow, request.Target, now)
	if err != nil {
		return s.fail(ctx, execution, executorUUID, repairFailureCommitV1, err)
	}
	if !committed {
		return s.fail(ctx, execution, executorUUID, repairFailureCommitV1, fmt.Errorf("transactional commit did not apply"))
	}
	result, err := s.executions.GetWorkflowRepairExecution(ctx, executionUUID)
	if err != nil {
		return nil, fmt.Errorf("reload committed Workflow repair execution: %w", err)
	}
	if result == nil || result.Status != application.AgentWorkflowRepairExecutionStatusCommitted {
		return nil, fmt.Errorf("%w: committed execution did not converge", application.ErrAgentWorkflowRepairConflict)
	}
	return result, nil
}

func (s *PersistentAgentWorkflowRepairExecutorV1) Rollback(ctx context.Context, request application.AgentWorkflowRepairRollbackRequestV1) (*application.AgentWorkflowRepairExecutionV1, error) {
	executionUUID, executorUUID := strings.TrimSpace(request.ExecutionUUID), strings.TrimSpace(request.ExecutorUUID)
	execution, err := s.executions.GetWorkflowRepairExecution(ctx, executionUUID)
	if err != nil {
		return nil, fmt.Errorf("load Workflow repair execution for rollback: %w", err)
	}
	if execution == nil || execution.ExecutorUUID != executorUUID || execution.Status != application.AgentWorkflowRepairExecutionStatusCommitted {
		return nil, fmt.Errorf("%w: only the original committed executor may rollback", application.ErrAgentWorkflowRepairDenied)
	}
	grant, err := s.repairs.GetWorkflowRepairOperatorGrant(ctx, executorUUID)
	if err != nil {
		return nil, fmt.Errorf("load Workflow repair rollback grant: %w", err)
	}
	now := s.now().UTC()
	if grant == nil || grant.UserUUID != executorUUID || grant.Version != execution.ExecutorGrantVersion || !grant.CanExecute || !grant.Active(now) {
		return nil, fmt.Errorf("%w: rollback executor grant is unavailable", application.ErrAgentWorkflowRepairDenied)
	}
	task, err := s.policies.GetTask(ctx, execution.TaskUUID)
	if err != nil {
		return nil, fmt.Errorf("load Workflow repair rollback task: %w", err)
	}
	if task == nil || task.Workflow == nil {
		return nil, fmt.Errorf("%w: target projection is unavailable for rollback", application.ErrAgentWorkflowRepairPrecondition)
	}
	if application.WorkflowProjectionSHA256V1(task.Workflow) != execution.TargetSHA256 {
		return nil, fmt.Errorf("%w: current projection is no longer the committed target", application.ErrAgentWorkflowRepairPrecondition)
	}
	if request.Rollback == nil {
		if execution.RollbackSHA256 != "" {
			return nil, fmt.Errorf("%w: rollback projection is required", application.ErrAgentWorkflowRepairPrecondition)
		}
	} else if request.Rollback.Validate() != nil || execution.RollbackSHA256 == "" || application.WorkflowProjectionSHA256V1(request.Rollback) != execution.RollbackSHA256 || request.Rollback.TaskUUID != execution.TaskUUID {
		return nil, fmt.Errorf("%w: rollback projection hash mismatch", application.ErrAgentWorkflowRepairPrecondition)
	}
	rolledBack, err := s.transaction.RollbackWorkflowRepairProjection(ctx, executionUUID, executorUUID, grant.Version, *task.Workflow, request.Rollback, now)
	if err != nil {
		return nil, fmt.Errorf("rollback Workflow repair projection: %w", err)
	}
	if !rolledBack {
		return nil, fmt.Errorf("%w: transactional rollback did not apply", application.ErrAgentWorkflowRepairConflict)
	}
	result, err := s.executions.GetWorkflowRepairExecution(ctx, executionUUID)
	if err != nil {
		return nil, fmt.Errorf("reload rolled-back Workflow repair execution: %w", err)
	}
	if result == nil || result.Status != application.AgentWorkflowRepairExecutionStatusRolledBack {
		return nil, fmt.Errorf("%w: rolled-back execution did not converge", application.ErrAgentWorkflowRepairConflict)
	}
	return result, nil
}

func (s *PersistentAgentWorkflowRepairExecutorV1) fail(ctx context.Context, execution *application.AgentWorkflowRepairExecutionV1, executorUUID, code string, cause error) (*application.AgentWorkflowRepairExecutionV1, error) {
	if _, err := s.executions.FailWorkflowRepairExecution(ctx, execution.ExecutionUUID, executorUUID, code, s.now().UTC()); err != nil {
		return nil, fmt.Errorf("%w: fail execution after %v: %v", application.ErrAgentWorkflowRepairConflict, cause, err)
	}
	return nil, fmt.Errorf("%w: %v", application.ErrAgentWorkflowRepairPrecondition, cause)
}

func validateRollbackPayload(execution *application.AgentWorkflowRepairExecutionV1, rollback *application.AgentTaskWorkflowProjectionV1) error {
	if rollback == nil {
		if execution.RollbackSHA256 != "" {
			return fmt.Errorf("%w: rollback projection is required by execution plan", application.ErrAgentWorkflowRepairPrecondition)
		}
		return nil
	}
	if rollback.Validate() != nil || execution.RollbackSHA256 == "" || rollback.TaskUUID != execution.TaskUUID || application.WorkflowProjectionSHA256V1(rollback) != execution.RollbackSHA256 {
		return fmt.Errorf("%w: rollback projection hash mismatch", application.ErrAgentWorkflowRepairPrecondition)
	}
	return nil
}
