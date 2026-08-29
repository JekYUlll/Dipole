package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	agentapplication "github.com/JekYUlll/Dipole/internal/services/agent/application"
)

type repairExecutorPolicyStub struct {
	application.AgentPolicyStoreV1
	task  *application.AgentTaskV1
	grant *application.AgentWorkflowRepairOperatorGrantV1
}

type repairExecutorAuditStub struct {
	application.AgentWorkflowRepairAuditStoreV1
	grant *application.AgentWorkflowRepairOperatorGrantV1
}

func (s *repairExecutorAuditStub) GetWorkflowRepairOperatorGrant(context.Context, string) (*application.AgentWorkflowRepairOperatorGrantV1, error) {
	if s.grant == nil {
		return nil, nil
	}
	copy := *s.grant
	return &copy, nil
}

func (s *repairExecutorPolicyStub) GetTask(context.Context, string) (*application.AgentTaskV1, error) {
	if s.task == nil {
		return nil, nil
	}
	copy := *s.task
	return &copy, nil
}

func (s *repairExecutorPolicyStub) GetWorkflowRepairOperatorGrant(context.Context, string) (*application.AgentWorkflowRepairOperatorGrantV1, error) {
	if s.grant == nil {
		return nil, nil
	}
	copy := *s.grant
	return &copy, nil
}

type repairExecutorTransactionStub struct {
	executions *repairExecutionStoreStubV1
	commits    int
	rollbacks  int
}

func (s *repairExecutorTransactionStub) CommitWorkflowRepairProjection(_ context.Context, executionUUID, executorUUID string, grantVersion uint64, expected *application.AgentTaskWorkflowProjectionV1, target application.AgentTaskWorkflowProjectionV1, _ time.Time) (bool, error) {
	if s.executions.execution == nil || s.executions.execution.ExecutionUUID != executionUUID || s.executions.execution.ExecutorUUID != executorUUID || s.executions.execution.ExecutorGrantVersion != grantVersion || s.executions.execution.Status != application.AgentWorkflowRepairExecutionStatusExecuting {
		return false, nil
	}
	if s.executions.execution.ExpectedCurrentSHA256 != projectionHash(expected) || target.TaskUUID != s.executions.execution.TaskUUID {
		return false, nil
	}
	s.commits++
	s.executions.execution.Status = application.AgentWorkflowRepairExecutionStatusCommitted
	return true, nil
}

func (s *repairExecutorTransactionStub) RollbackWorkflowRepairProjection(_ context.Context, executionUUID, executorUUID string, grantVersion uint64, expected application.AgentTaskWorkflowProjectionV1, _ *application.AgentTaskWorkflowProjectionV1, _ time.Time) (bool, error) {
	if s.executions.execution == nil || s.executions.execution.ExecutionUUID != executionUUID || s.executions.execution.ExecutorUUID != executorUUID || s.executions.execution.ExecutorGrantVersion != grantVersion || s.executions.execution.Status != application.AgentWorkflowRepairExecutionStatusCommitted {
		return false, nil
	}
	if projectionHash(&expected) != s.executions.execution.TargetSHA256 {
		return false, nil
	}
	s.rollbacks++
	s.executions.execution.Status = application.AgentWorkflowRepairExecutionStatusRolledBack
	return true, nil
}

func TestPersistentAgentWorkflowRepairExecutorCommitsAndRollsBackWithFreshGrant(t *testing.T) {
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	current := application.AgentTaskWorkflowProjectionV1{TaskUUID: "TASK-1", WorkflowID: "dipole-agent-task/TASK-1", RunID: "RUN-1", Status: application.AgentTaskWorkflowStatusRunning, Revision: 2}
	target := current
	target.RunID, target.Status, target.Revision = "RUN-2", application.AgentTaskWorkflowStatusCompleted, 3
	execution := &application.AgentWorkflowRepairExecutionV1{
		ExecutionUUID: "repair-execution:" + repeatHex("a"), PlanID: "repair-plan:" + repeatHex("b"), ProposalUUID: "repair:" + repeatHex("c"),
		TaskUUID: "TASK-1", ExecutorUUID: "EXEC-1", ExecutorGrantVersion: 7, ExpectedCurrentSHA256: projectionHash(&current), TargetSHA256: projectionHash(&target), RollbackSHA256: projectionHash(&current), Status: application.AgentWorkflowRepairExecutionStatusPrepared,
	}
	executions := &repairExecutionStoreStubV1{execution: execution}
	policy := &repairExecutorPolicyStub{task: &application.AgentTaskV1{TaskUUID: "TASK-1", Workflow: &current}}
	audit := &repairExecutorAuditStub{grant: &application.AgentWorkflowRepairOperatorGrantV1{UserUUID: "EXEC-1", Version: 7, CanExecute: true, ValidFrom: now.Add(-time.Minute), ExpiresAt: timePtr(now.Add(time.Minute))}}
	transaction := &repairExecutorTransactionStub{executions: executions}
	executor, err := agentapplication.NewPersistentAgentWorkflowRepairExecutorV1WithClock(policy, audit, executions, transaction, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	result, err := executor.Execute(context.Background(), application.AgentWorkflowRepairExecuteRequestV1{ExecutionUUID: execution.ExecutionUUID, ExecutorUUID: execution.ExecutorUUID, Target: target, Rollback: &current})
	if err != nil || result == nil || result.Status != application.AgentWorkflowRepairExecutionStatusCommitted || transaction.commits != 1 {
		t.Fatalf("execute result=%+v commits=%d err=%v", result, transaction.commits, err)
	}
	policy.task.Workflow = &target
	rollback, err := executor.Rollback(context.Background(), application.AgentWorkflowRepairRollbackRequestV1{ExecutionUUID: execution.ExecutionUUID, ExecutorUUID: execution.ExecutorUUID, Rollback: &current})
	if err != nil || rollback == nil || rollback.Status != application.AgentWorkflowRepairExecutionStatusRolledBack || transaction.rollbacks != 1 {
		t.Fatalf("rollback result=%+v rollbacks=%d err=%v", rollback, transaction.rollbacks, err)
	}
}

func TestPersistentAgentWorkflowRepairExecutorRejectsGrantAndFailsPrecondition(t *testing.T) {
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	current := application.AgentTaskWorkflowProjectionV1{TaskUUID: "TASK-1", WorkflowID: "dipole-agent-task/TASK-1", RunID: "RUN-1", Status: application.AgentTaskWorkflowStatusRunning, Revision: 2}
	target := current
	target.Revision = 3
	base := &application.AgentWorkflowRepairExecutionV1{ExecutionUUID: "repair-execution:" + repeatHex("d"), PlanID: "repair-plan:" + repeatHex("e"), ProposalUUID: "repair:" + repeatHex("f"), TaskUUID: "TASK-1", ExecutorUUID: "EXEC-1", ExecutorGrantVersion: 7, ExpectedCurrentSHA256: projectionHash(&current), TargetSHA256: projectionHash(&target), Status: application.AgentWorkflowRepairExecutionStatusPrepared}
	executions := &repairExecutionStoreStubV1{execution: base}
	policy := &repairExecutorPolicyStub{task: &application.AgentTaskV1{TaskUUID: "TASK-1", Workflow: &current}}
	audit := &repairExecutorAuditStub{grant: &application.AgentWorkflowRepairOperatorGrantV1{UserUUID: "EXEC-1", Version: 7, CanExecute: false, ValidFrom: now.Add(-time.Minute), ExpiresAt: timePtr(now.Add(time.Minute))}}
	transaction := &repairExecutorTransactionStub{executions: executions}
	executor, err := agentapplication.NewPersistentAgentWorkflowRepairExecutorV1WithClock(policy, audit, executions, transaction, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executor.Execute(context.Background(), application.AgentWorkflowRepairExecuteRequestV1{ExecutionUUID: base.ExecutionUUID, ExecutorUUID: base.ExecutorUUID, Target: target}); !errors.Is(err, application.ErrAgentWorkflowRepairDenied) {
		t.Fatalf("grant error=%v", err)
	}
	audit.grant.CanExecute = true
	forged := target
	forged.Revision++
	if _, err := executor.Execute(context.Background(), application.AgentWorkflowRepairExecuteRequestV1{ExecutionUUID: base.ExecutionUUID, ExecutorUUID: base.ExecutorUUID, Target: forged}); !errors.Is(err, application.ErrAgentWorkflowRepairPrecondition) {
		t.Fatalf("precondition error=%v", err)
	}
	if executions.execution.Status != application.AgentWorkflowRepairExecutionStatusFailed {
		t.Fatalf("status=%s, want failed", executions.execution.Status)
	}
}

func projectionHash(projection *application.AgentTaskWorkflowProjectionV1) string {
	if projection == nil {
		return ""
	}
	payload, _ := json.Marshal(struct {
		Revision      uint64 `json:"revision"`
		Status        string `json:"status"`
		WorkflowID    string `json:"workflowId"`
		WorkflowRunID string `json:"workflowRunId"`
	}{projection.Revision, string(projection.Status), projection.WorkflowID, projection.RunID})
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

func repeatHex(value string) string {
	result := ""
	for len(result) < 64 {
		result += value
	}
	return result[:64]
}

func timePtr(value time.Time) *time.Time { return &value }
