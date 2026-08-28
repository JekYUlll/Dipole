package agenttimelinerepair

import "testing"

func TestEvaluateEligibleRepairWindow(t *testing.T) {
	evidence := Evidence{SchemaVersion: "dipole.agent.timeline-repair-rollout-evidence.v1", Service: "repair", DeploymentRevision: "rev-1", WindowStart: "2026-08-29T00:00:00Z", WindowEnd: "2026-08-29T02:00:00Z", WorkerReady: true, Operator: "ops-1", RollbackVerified: true, AlertState: "clear", Outcomes: OutcomeCounts{Claimed: 100, Repaired: 98, Retried: 2, ProjectionError: 1}}
	policy := Policy{MinimumWindowMinutes: 60, MinimumClaimed: 50, MaximumRetryRatioBPS: 500, MaximumProjectionErrorBPS: 200, MaximumCompleteErrorBPS: 0, MaximumClaimErrorBPS: 0, MaximumInvalid: 0, RequireRollbackVerified: true, RequireClearAlerts: true}
	report, err := Evaluate(evidence, policy)
	if err != nil || report.Decision != "eligible" {
		t.Fatalf("report=%+v err=%v", report, err)
	}
}

func TestEvaluateBlocksUnsafeRepairWindow(t *testing.T) {
	evidence := Evidence{SchemaVersion: "dipole.agent.timeline-repair-rollout-evidence.v1", Service: "repair", DeploymentRevision: "rev-1", WindowStart: "2026-08-29T00:00:00Z", WindowEnd: "2026-08-29T00:10:00Z", WorkerReady: false, Operator: "ops-1", AlertState: "critical", Outcomes: OutcomeCounts{Claimed: 10, Retried: 4, ProjectionError: 4, Invalid: 1}}
	policy := Policy{MinimumWindowMinutes: 60, MinimumClaimed: 50, MaximumRetryRatioBPS: 500, MaximumProjectionErrorBPS: 200, MaximumInvalid: 0, RequireRollbackVerified: true, RequireClearAlerts: true}
	report, err := Evaluate(evidence, policy)
	if err != nil || report.Decision != "blocked" {
		t.Fatalf("report=%+v err=%v", report, err)
	}
	if len(report.Reasons) < 5 {
		t.Fatalf("reasons=%v", report.Reasons)
	}
}

func TestParseRejectsUnknownFields(t *testing.T) {
	if _, err := ParseEvidence([]byte(`{"schemaVersion":"dipole.agent.timeline-repair-rollout-evidence.v1","service":"repair","deploymentRevision":"rev","windowStart":"2026-08-29T00:00:00Z","windowEnd":"2026-08-29T01:00:00Z","workerReady":true,"operator":"ops","rollbackVerified":true,"alertState":"clear","outcomes":{"claimed":1,"repaired":1},"extra":true}`)); err == nil {
		t.Fatal("expected unknown field rejection")
	}
}
