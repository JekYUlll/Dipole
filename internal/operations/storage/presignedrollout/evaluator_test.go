package presignedrollout

import (
	"math"
	"testing"
)

func TestEvaluateEligibleEvidence(t *testing.T) {
	policy := validPolicy()
	evidence := validEvidence(policy)
	report, err := Evaluate(evidence, policy)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if report.Decision != "eligible" || len(report.Reasons) != 0 {
		t.Fatalf("report = %+v, want eligible", report)
	}
	if report.Metrics.RelayFallbackRatioBPS != 100 || report.Metrics.FailureRatioBPS != 100 {
		t.Fatalf("ratios = %+v", report.Metrics)
	}
}

func TestEvaluateBlocksUnsafeDefaultPromotion(t *testing.T) {
	evidence := validEvidence(validPolicy())
	evidence.WindowEnd = "2026-08-30T00:30:00Z"
	evidence.RollbackVerified = false
	evidence.AlertState = "warning"
	evidence.Reviewer = evidence.Operator
	evidence.Outcomes = OutcomeCounts{Attempted: 100, DirectCompleted: 50, RelayFallback: 20, Failed: 20, Expired: 10, Retried: 20, ChecksumMismatch: 1}
	evidence.Latency.DirectP95Millis = 30_001
	report, err := Evaluate(evidence, validPolicy())
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	want := map[string]bool{
		"alerts_not_clear": true, "checksum_mismatch_ratio_exceeded": true, "direct_p95_latency_exceeded": true,
		"expired_ratio_exceeded": true, "failure_ratio_exceeded": true, "independent_review_missing": true,
		"insufficient_direct_samples": true, "relay_fallback_ratio_exceeded": true, "rollback_not_verified": true,
		"window_below_minimum": true,
	}
	if report.Decision != "blocked" || len(report.Reasons) != len(want) {
		t.Fatalf("report = %+v", report)
	}
	for _, reason := range report.Reasons {
		if !want[reason] {
			t.Fatalf("unexpected reason %q", reason)
		}
	}
}

func TestParseEvidenceRejectsUnknownAndInconsistentTerminalCounts(t *testing.T) {
	if _, err := ParseEvidence([]byte(`{"schemaVersion":"dipole.multipart-presigned-rollout-evidence.v1","service":"gateway","deploymentRevision":"rev","policySha256":"0123456789012345678901234567890123456789012345678901234567890123","windowStart":"2026-08-30T00:00:00Z","windowEnd":"2026-08-31T00:00:00Z","operator":"ops","reviewer":"review","rollbackVerified":true,"alertState":"clear","outcomes":{"attempted":1,"directCompleted":1,"relayFallback":0,"failed":0,"expired":0,"aborted":0,"retried":0,"checksumMismatch":0},"latency":{"directP95Millis":1},"extra":true}`)); err == nil {
		t.Fatal("ParseEvidence() accepted unknown field")
	}
	evidence := validEvidence(validPolicy())
	evidence.Outcomes.Attempted++
	if _, err := Evaluate(evidence, validPolicy()); err == nil {
		t.Fatal("Evaluate() accepted inconsistent terminal counts")
	}
}

func TestEvaluateBlocksPolicyHashDrift(t *testing.T) {
	policy := validPolicy()
	evidence := validEvidence(policy)
	evidence.PolicySHA256 = "0123456789012345678901234567890123456789012345678901234567890123"
	report, err := Evaluate(evidence, policy)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if report.Decision != "blocked" || len(report.Reasons) != 1 || report.Reasons[0] != "policy_hash_mismatch" {
		t.Fatalf("report = %+v", report)
	}
}

func TestEvaluateRejectsTerminalOutcomeOverflow(t *testing.T) {
	policy := validPolicy()
	evidence := validEvidence(policy)
	evidence.Outcomes = OutcomeCounts{Attempted: math.MaxUint64, DirectCompleted: math.MaxUint64, RelayFallback: 1}
	if _, err := Evaluate(evidence, policy); err == nil {
		t.Fatal("Evaluate() accepted an overflowing terminal outcome count")
	}
}

func validEvidence(policy Policy) Evidence {
	return Evidence{
		SchemaVersion: "dipole.multipart-presigned-rollout-evidence.v1", Service: "gateway", DeploymentRevision: "gateway@rev-1",
		PolicySHA256: PolicySHA256(policy),
		WindowStart:  "2026-08-30T00:00:00Z", WindowEnd: "2026-08-31T00:00:00Z", Operator: "ops-a", Reviewer: "ops-b", RollbackVerified: true, AlertState: "clear",
		Outcomes: OutcomeCounts{Attempted: 100, DirectCompleted: 98, RelayFallback: 1, Failed: 1, Retried: 3}, Latency: Latency{DirectP95Millis: 5_000},
	}
}

func validPolicy() Policy {
	return Policy{MinimumWindowMinutes: 1_440, MinimumAttempts: 100, MinimumDirectCompleted: 90, MaximumRelayFallbackBPS: 100, MaximumFailureBPS: 100, MaximumExpiredBPS: 0, MaximumChecksumMismatchBPS: 0, MaximumDirectP95Millis: 30_000, RequireRollbackVerified: true, RequireClearAlerts: true, RequireIndependentReviewer: true}
}
