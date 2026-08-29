package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
)

func (r *AgentPolicyRepository) CreateWorkflowRepairExecution(ctx context.Context, execution application.AgentWorkflowRepairExecutionV1) (bool, error) {
	if err := execution.Validate(); err != nil {
		return false, fmt.Errorf("validate Workflow repair execution: %w", err)
	}
	rows, err := r.queries.InsertAgentWorkflowRepairExecution(ctx, generated.InsertAgentWorkflowRepairExecutionParams{
		ExecutionUuid: execution.ExecutionUUID, PlanID: execution.PlanID, ProposalUuid: execution.ProposalUUID,
		TaskUuid: execution.TaskUUID, ExecutorUuid: execution.ExecutorUUID, ExecutorGrantVersion: execution.ExecutorGrantVersion,
		ExpectedCurrentSha256: nullableString(execution.ExpectedCurrentSHA256), TargetSha256: execution.TargetSHA256,
		RollbackSha256: nullableString(execution.RollbackSHA256),
	})
	if err != nil {
		return false, fmt.Errorf("create Workflow repair execution: %w", err)
	}
	if rows > 0 {
		return true, nil
	}
	existing, err := r.GetWorkflowRepairExecution(ctx, execution.ExecutionUUID)
	if err != nil {
		return false, err
	}
	if existing == nil || !reflect.DeepEqual(*existing, execution) {
		return false, fmt.Errorf("%w: execution_uuid=%s", ErrAgentPolicyConflict, strings.TrimSpace(execution.ExecutionUUID))
	}
	return false, nil
}

func (r *AgentPolicyRepository) GetWorkflowRepairExecution(ctx context.Context, executionUUID string) (*application.AgentWorkflowRepairExecutionV1, error) {
	row, err := r.queries.GetAgentWorkflowRepairExecution(ctx, strings.TrimSpace(executionUUID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get Workflow repair execution: %w", err)
	}
	execution := &application.AgentWorkflowRepairExecutionV1{
		ExecutionUUID: row.ExecutionUuid, PlanID: row.PlanID, ProposalUUID: row.ProposalUuid, TaskUUID: row.TaskUuid,
		ExecutorUUID: row.ExecutorUuid, ExecutorGrantVersion: row.ExecutorGrantVersion, TargetSHA256: row.TargetSha256,
		Status: application.AgentWorkflowRepairExecutionStatusV1(row.Status),
	}
	if row.ExpectedCurrentSha256.Valid {
		execution.ExpectedCurrentSHA256 = row.ExpectedCurrentSha256.String
	}
	if row.RollbackSha256.Valid {
		execution.RollbackSHA256 = row.RollbackSha256.String
	}
	return execution, nil
}

func (r *AgentPolicyRepository) ClaimWorkflowRepairExecution(ctx context.Context, executionUUID, executorUUID string, grantVersion uint64, startedAt time.Time) (bool, error) {
	executionUUID, executorUUID = strings.TrimSpace(executionUUID), strings.TrimSpace(executorUUID)
	if executionUUID == "" || executorUUID == "" || grantVersion == 0 || startedAt.IsZero() {
		return false, fmt.Errorf("validate Workflow repair execution claim: %w", application.ErrAgentWorkflowRepairConflict)
	}
	rows, err := r.queries.ClaimAgentWorkflowRepairExecution(ctx, generated.ClaimAgentWorkflowRepairExecutionParams{
		StartedAt: sql.NullTime{Time: startedAt.UTC(), Valid: true}, ExecutionUuid: executionUUID, ExecutorUuid: executorUUID, ExecutorGrantVersion: grantVersion,
	})
	if err != nil {
		return false, fmt.Errorf("claim Workflow repair execution: %w", err)
	}
	return rows > 0, nil
}

func (r *AgentPolicyRepository) FailWorkflowRepairExecution(ctx context.Context, executionUUID, executorUUID, failureCode string, finishedAt time.Time) (bool, error) {
	executionUUID, executorUUID, failureCode = strings.TrimSpace(executionUUID), strings.TrimSpace(executorUUID), strings.TrimSpace(failureCode)
	if executionUUID == "" || executorUUID == "" || failureCode == "" || len(failureCode) > 64 || finishedAt.IsZero() {
		return false, fmt.Errorf("validate Workflow repair execution failure: %w", application.ErrAgentWorkflowRepairConflict)
	}
	rows, err := r.queries.FailAgentWorkflowRepairExecution(ctx, generated.FailAgentWorkflowRepairExecutionParams{
		FailureCode: sql.NullString{String: failureCode, Valid: true}, FinishedAt: sql.NullTime{Time: finishedAt.UTC(), Valid: true}, ExecutionUuid: executionUUID, ExecutorUuid: executorUUID,
	})
	if err != nil {
		return false, fmt.Errorf("fail Workflow repair execution: %w", err)
	}
	return rows > 0, nil
}

func (r *AgentPolicyRepository) ApplyWorkflowRepairProjection(ctx context.Context, expected *application.AgentTaskWorkflowProjectionV1, target application.AgentTaskWorkflowProjectionV1) (bool, error) {
	target.TaskUUID = strings.TrimSpace(target.TaskUUID)
	if err := target.Validate(); err != nil || target.TaskUUID == "" {
		return false, fmt.Errorf("validate Workflow repair target projection: %w", application.ErrAgentWorkflowRepairPrecondition)
	}
	workflowID := sql.NullString{String: target.WorkflowID, Valid: true}
	workflowRunID := sql.NullString{String: target.RunID, Valid: true}
	workflowStatus := sql.NullString{String: string(target.Status), Valid: true}
	workflowRevision := sql.NullInt64{Int64: int64(target.Revision), Valid: true}
	var rows int64
	var err error
	if expected == nil {
		rows, err = r.queries.ApplyAgentWorkflowRepairProjectionMissingCurrent(ctx, generated.ApplyAgentWorkflowRepairProjectionMissingCurrentParams{
			WorkflowID: workflowID, WorkflowRunID: workflowRunID, WorkflowStatus: workflowStatus, WorkflowRevision: workflowRevision, TaskUuid: target.TaskUUID,
		})
	} else {
		if err := expected.Validate(); err != nil || expected.TaskUUID != target.TaskUUID {
			return false, fmt.Errorf("validate Workflow repair expected projection: %w", application.ErrAgentWorkflowRepairPrecondition)
		}
		rows, err = r.queries.ApplyAgentWorkflowRepairProjectionExpectedCurrent(ctx, generated.ApplyAgentWorkflowRepairProjectionExpectedCurrentParams{
			WorkflowID: workflowID, WorkflowRunID: workflowRunID, WorkflowStatus: workflowStatus, WorkflowRevision: workflowRevision,
			TaskUuid: target.TaskUUID, WorkflowID_2: sql.NullString{String: expected.WorkflowID, Valid: true}, WorkflowRunID_2: sql.NullString{String: expected.RunID, Valid: true},
			WorkflowStatus_2: sql.NullString{String: string(expected.Status), Valid: true}, WorkflowRevision_2: sql.NullInt64{Int64: int64(expected.Revision), Valid: true},
		})
	}
	if err != nil {
		return false, fmt.Errorf("apply Workflow repair projection: %w", err)
	}
	return rows > 0, nil
}

var errRepairExecutionCASNoop = errors.New("Workflow repair execution CAS did not apply")

func (r *AgentPolicyRepository) CommitWorkflowRepairProjection(ctx context.Context, executionUUID, executorUUID string, grantVersion uint64, expected *application.AgentTaskWorkflowProjectionV1, target application.AgentTaskWorkflowProjectionV1, finishedAt time.Time) (bool, error) {
	if r.store == nil || strings.TrimSpace(executionUUID) == "" || strings.TrimSpace(executorUUID) == "" || grantVersion == 0 || finishedAt.IsZero() {
		return false, fmt.Errorf("validate Workflow repair transactional commit: %w", application.ErrAgentWorkflowRepairPrecondition)
	}
	target.TaskUUID = strings.TrimSpace(target.TaskUUID)
	if err := target.Validate(); err != nil || target.TaskUUID == "" {
		return false, fmt.Errorf("validate Workflow repair transactional target: %w", application.ErrAgentWorkflowRepairPrecondition)
	}
	if expected != nil && (expected.Validate() != nil || expected.TaskUUID != target.TaskUUID) {
		return false, fmt.Errorf("validate Workflow repair transactional expected: %w", application.ErrAgentWorkflowRepairPrecondition)
	}
	workflowID := sql.NullString{String: target.WorkflowID, Valid: true}
	workflowRunID := sql.NullString{String: target.RunID, Valid: true}
	workflowStatus := sql.NullString{String: string(target.Status), Valid: true}
	workflowRevision := sql.NullInt64{Int64: int64(target.Revision), Valid: true}
	err := r.store.WithinTx(ctx, nil, func(q *generated.Queries) error {
		var rows int64
		var applyErr error
		if expected == nil {
			rows, applyErr = q.ApplyAgentWorkflowRepairProjectionMissingCurrent(ctx, generated.ApplyAgentWorkflowRepairProjectionMissingCurrentParams{
				WorkflowID: workflowID, WorkflowRunID: workflowRunID, WorkflowStatus: workflowStatus, WorkflowRevision: workflowRevision, TaskUuid: target.TaskUUID,
			})
		} else {
			rows, applyErr = q.ApplyAgentWorkflowRepairProjectionExpectedCurrent(ctx, generated.ApplyAgentWorkflowRepairProjectionExpectedCurrentParams{
				WorkflowID: workflowID, WorkflowRunID: workflowRunID, WorkflowStatus: workflowStatus, WorkflowRevision: workflowRevision,
				TaskUuid: target.TaskUUID, WorkflowID_2: sql.NullString{String: expected.WorkflowID, Valid: true}, WorkflowRunID_2: sql.NullString{String: expected.RunID, Valid: true},
				WorkflowStatus_2: sql.NullString{String: string(expected.Status), Valid: true}, WorkflowRevision_2: sql.NullInt64{Int64: int64(expected.Revision), Valid: true},
			})
		}
		if applyErr != nil {
			return fmt.Errorf("apply Workflow repair projection in transaction: %w", applyErr)
		}
		if rows == 0 {
			return errRepairExecutionCASNoop
		}
		committed, commitErr := q.CommitAgentWorkflowRepairExecution(ctx, generated.CommitAgentWorkflowRepairExecutionParams{
			FinishedAt: sql.NullTime{Time: finishedAt.UTC(), Valid: true}, ExecutionUuid: strings.TrimSpace(executionUUID), ExecutorUuid: strings.TrimSpace(executorUUID), ExecutorGrantVersion: grantVersion,
		})
		if commitErr != nil {
			return fmt.Errorf("commit Workflow repair execution in transaction: %w", commitErr)
		}
		if committed == 0 {
			return errRepairExecutionCASNoop
		}
		return nil
	})
	if errors.Is(err, errRepairExecutionCASNoop) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *AgentPolicyRepository) RollbackWorkflowRepairProjection(ctx context.Context, executionUUID, executorUUID string, grantVersion uint64, expected application.AgentTaskWorkflowProjectionV1, rollback *application.AgentTaskWorkflowProjectionV1, finishedAt time.Time) (bool, error) {
	if r.store == nil || strings.TrimSpace(executionUUID) == "" || strings.TrimSpace(executorUUID) == "" || grantVersion == 0 || finishedAt.IsZero() {
		return false, fmt.Errorf("validate Workflow repair rollback: %w", application.ErrAgentWorkflowRepairPrecondition)
	}
	if err := expected.Validate(); err != nil {
		return false, fmt.Errorf("validate Workflow repair rollback expected: %w", application.ErrAgentWorkflowRepairPrecondition)
	}
	if rollback != nil && (rollback.Validate() != nil || rollback.TaskUUID != expected.TaskUUID) {
		return false, fmt.Errorf("validate Workflow repair rollback target: %w", application.ErrAgentWorkflowRepairPrecondition)
	}
	err := r.store.WithinTx(ctx, nil, func(q *generated.Queries) error {
		var rows int64
		var rollbackErr error
		expectedWorkflowID := sql.NullString{String: expected.WorkflowID, Valid: true}
		expectedRunID := sql.NullString{String: expected.RunID, Valid: true}
		expectedStatus := sql.NullString{String: string(expected.Status), Valid: true}
		expectedRevision := sql.NullInt64{Int64: int64(expected.Revision), Valid: true}
		if rollback == nil {
			rows, rollbackErr = q.ClearAgentWorkflowRepairProjection(ctx, generated.ClearAgentWorkflowRepairProjectionParams{
				TaskUuid: expected.TaskUUID, WorkflowID: expectedWorkflowID, WorkflowRunID: expectedRunID, WorkflowStatus: expectedStatus, WorkflowRevision: expectedRevision,
			})
		} else {
			rows, rollbackErr = q.RollbackAgentWorkflowRepairProjection(ctx, generated.RollbackAgentWorkflowRepairProjectionParams{
				WorkflowID: sql.NullString{String: rollback.WorkflowID, Valid: true}, WorkflowRunID: sql.NullString{String: rollback.RunID, Valid: true}, WorkflowStatus: sql.NullString{String: string(rollback.Status), Valid: true}, WorkflowRevision: sql.NullInt64{Int64: int64(rollback.Revision), Valid: true},
				TaskUuid: expected.TaskUUID, WorkflowID_2: expectedWorkflowID, WorkflowRunID_2: expectedRunID, WorkflowStatus_2: expectedStatus, WorkflowRevision_2: expectedRevision,
			})
		}
		if rollbackErr != nil {
			return fmt.Errorf("rollback Workflow repair projection in transaction: %w", rollbackErr)
		}
		if rows == 0 {
			return errRepairExecutionCASNoop
		}
		marked, markErr := q.MarkAgentWorkflowRepairExecutionRolledBack(ctx, generated.MarkAgentWorkflowRepairExecutionRolledBackParams{
			FinishedAt: sql.NullTime{Time: finishedAt.UTC(), Valid: true}, ExecutionUuid: strings.TrimSpace(executionUUID), ExecutorUuid: strings.TrimSpace(executorUUID), ExecutorGrantVersion: grantVersion,
		})
		if markErr != nil {
			return fmt.Errorf("mark Workflow repair execution rolled back in transaction: %w", markErr)
		}
		if marked == 0 {
			return errRepairExecutionCASNoop
		}
		return nil
	})
	if errors.Is(err, errRepairExecutionCASNoop) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
