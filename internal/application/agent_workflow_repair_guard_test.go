package application

import (
	"strings"
	"testing"
	"time"
)

func TestAgentWorkflowRepairPreconditionBindsGrantAndProjectionHashes(t *testing.T) {
	now := time.Date(2026, 8, 29, 1, 0, 0, 0, time.UTC)
	current := AgentTaskWorkflowProjectionV1{TaskUUID: "TASK-1", WorkflowID: "dipole-agent-task/TASK-1", RunID: "RUN-1", Status: AgentTaskWorkflowStatusRunning, Revision: 2}
	if got := workflowProjectionSHA256(&current); got != "2570831452dbdc2dead6ef203e00b0ea3885d2d03fb8189d2101d10dd5d5b8b3" {
		t.Fatalf("cross-language projection hash = %s", got)
	}
	target := current
	target.Status, target.Revision = AgentTaskWorkflowStatusFailed, 3
	execution := AgentWorkflowRepairExecutionV1{ExecutionUUID: "repair-execution:" + strings.Repeat("a", 64), PlanID: "repair-plan:" + strings.Repeat("b", 64), ProposalUUID: "repair:" + strings.Repeat("c", 64), TaskUUID: "TASK-1", ExecutorUUID: "EXEC-1", ExecutorGrantVersion: 7, ExpectedCurrentSHA256: workflowProjectionSHA256(&current), TargetSHA256: workflowProjectionSHA256(&target), Status: AgentWorkflowRepairExecutionStatusPrepared}
	grant := AgentWorkflowRepairOperatorGrantV1{UserUUID: "EXEC-1", Version: 7, CanExecute: true, ValidFrom: now.Add(-time.Minute), ExpiresAt: timePointerForGuard(now.Add(time.Minute))}
	if err := (AgentWorkflowRepairPreconditionV1{Execution: execution, Grant: grant, Current: &current, Target: target, At: now}).Validate(); err != nil {
		t.Fatal(err)
	}
	target.Revision++
	if err := (AgentWorkflowRepairPreconditionV1{Execution: execution, Grant: grant, Current: &current, Target: target, At: now}).Validate(); err == nil {
		t.Fatal("expected target hash mismatch")
	}
}

func TestAgentWorkflowRepairPreconditionRejectsStaleOrUnauthorizedState(t *testing.T) {
	now := time.Date(2026, 8, 29, 1, 0, 0, 0, time.UTC)
	target := AgentTaskWorkflowProjectionV1{TaskUUID: "TASK-1", WorkflowID: "dipole-agent-task/TASK-1", RunID: "RUN-1", Status: AgentTaskWorkflowStatusFailed, Revision: 3}
	execution := AgentWorkflowRepairExecutionV1{ExecutionUUID: "repair-execution:" + strings.Repeat("a", 64), PlanID: "repair-plan:" + strings.Repeat("b", 64), ProposalUUID: "repair:" + strings.Repeat("c", 64), TaskUUID: "TASK-1", ExecutorUUID: "EXEC-1", ExecutorGrantVersion: 7, TargetSHA256: workflowProjectionSHA256(&target), Status: AgentWorkflowRepairExecutionStatusPrepared}
	grant := AgentWorkflowRepairOperatorGrantV1{UserUUID: "EXEC-1", Version: 7, CanExecute: false, ValidFrom: now.Add(-time.Minute), ExpiresAt: timePointerForGuard(now.Add(time.Minute))}
	if err := (AgentWorkflowRepairPreconditionV1{Execution: execution, Grant: grant, Target: target, At: now}).Validate(); err == nil {
		t.Fatal("expected execution permission denial")
	}
}

func timePointerForGuard(value time.Time) *time.Time { return &value }
