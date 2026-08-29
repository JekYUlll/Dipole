package cassandrareadrollout

import "testing"

func TestEvaluateEligibleReadRollout(t *testing.T) {
	evidence := validEvidence()
	policy := Policy{MinimumTotalRequests: 100, MinimumCassandraRequests: 40, MinimumObservedShareBPS: 4_000, MaximumFallbackBPS: 500, MaximumVerificationMismatchBPS: 100, MaximumVerificationErrorBPS: 100, MaximumCassandraP95LatencyMicros: 20_000}
	report, err := Evaluate(evidence, policy)
	if err != nil {
		t.Fatal(err)
	}
	if report.Decision != "eligible" || len(report.Reasons) != 0 || report.Metrics.ObservedCassandraShareBPS != 5_000 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestEvaluateBlocksLowSamplesAndFallback(t *testing.T) {
	evidence := validEvidence()
	evidence.Requests.Total = 10
	evidence.Requests.Cassandra = 10
	evidence.Requests.MySQL = 0
	evidence.Requests.Fallback = 2
	evidence.Requests.VerificationSamples = 0
	report, err := Evaluate(evidence, Policy{MinimumTotalRequests: 100, MinimumCassandraRequests: 40, MinimumObservedShareBPS: 4_000, MaximumFallbackBPS: 500, MaximumVerificationMismatchBPS: 100, MaximumVerificationErrorBPS: 100, MaximumCassandraP95LatencyMicros: 20_000})
	if err != nil {
		t.Fatal(err)
	}
	if report.Decision != "blocked" || !hasReason(report.Reasons, "insufficient_samples") || !hasReason(report.Reasons, "fallback_ratio_exceeded") {
		t.Fatalf("unexpected reasons: %+v", report.Reasons)
	}
}

func TestEvaluateRejectsInconsistentOrUnsafeEvidence(t *testing.T) {
	evidence := validEvidence()
	evidence.Requests.Cassandra = 101
	if _, err := Evaluate(evidence, Policy{}); err == nil {
		t.Fatal("expected route count validation error")
	}
	evidence = validEvidence()
	evidence.DeploymentRevision = ""
	if _, err := Evaluate(evidence, Policy{}); err == nil {
		t.Fatal("expected deployment revision validation error")
	}
}

func validEvidence() Evidence {
	return Evidence{
		SchemaVersion: "dipole.cassandra-read-rollout-evidence.v1", Service: "message-service", DeploymentRevision: "message@2026-08-29",
		WindowStart: "2026-08-29T00:00:00Z", WindowEnd: "2026-08-29T01:00:00Z", ConfiguredReadPercentage: 50,
		Requests: RouteCounts{Total: 100, Cassandra: 50, MySQL: 50, Fallback: 0, VerificationSamples: 20, VerificationMismatch: 0, VerificationErrors: 0},
		Latency:  RouteLatency{CassandraP95Micros: 5_000, MySQLP95Micros: 8_000},
	}
}

func hasReason(reasons []string, expected string) bool {
	for _, reason := range reasons {
		if reason == expected {
			return true
		}
	}
	return false
}
