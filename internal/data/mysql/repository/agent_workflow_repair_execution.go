package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"

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
