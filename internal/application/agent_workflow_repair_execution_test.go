package application

import "testing"

func TestAgentWorkflowRepairExecutionRequiresPreparedBoundEvidence(t *testing.T) {
	execution := AgentWorkflowRepairExecutionV1{
		ExecutionUUID: "repair-execution:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		PlanID: "repair-plan:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ProposalUUID: "repair:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		TaskUUID: "TASK-1", ExecutorUUID: "EXEC-1", ExecutorGrantVersion: 1,
		ExpectedCurrentSHA256: "c" + string(make([]byte, 63)), TargetSHA256: "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
		Status: AgentWorkflowRepairExecutionStatusPrepared,
	}
	if err := execution.Validate(); err == nil {
		t.Fatal("invalid optional hash must be rejected")
	}
	execution.ExpectedCurrentSHA256 = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	if err := execution.Validate(); err != nil {
		t.Fatal(err)
	}
	execution.Status = "executing"
	if err := execution.Validate(); err == nil {
		t.Fatal("execution state transition must remain unavailable")
	}
}
