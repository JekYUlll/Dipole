package synchhydration

import "testing"

func TestEvaluateEligiblePrimaryHydration(t *testing.T) {
	e := Evidence{SchemaVersion: "dipole.sync-cassandra-hydration-evidence.v1", Service: "sync-service", DeploymentRevision: "sync@1", Mode: "primary", WindowStart: "2026-08-29T00:00:00Z", WindowEnd: "2026-08-29T01:00:00Z", Requests: Counts{Total: 100, CassandraHit: 98, MySQLFallback: 2}, Latency: Latency{CassandraP95Micros: 4000}}
	r, err := Evaluate(e, Policy{MinimumTotalRequests: 100, MinimumCassandraHits: 90, MaximumFallbackBPS: 500, MaximumMissingBPS: 0, MaximumConflictBPS: 0, MaximumErrorBPS: 100, MaximumP95LatencyMicros: 10000})
	if err != nil || r.Decision != "eligible" || r.Metrics.CassandraHitBPS != 9800 {
		t.Fatalf("report=%+v err=%v", r, err)
	}
}

func TestEvaluateBlocksHydrationFailureRatios(t *testing.T) {
	e := validEvidence()
	e.Requests = Counts{Total: 100, CassandraHit: 80, MySQLFallback: 10, Missing: 5, Conflict: 3, Error: 2}
	r, err := Evaluate(e, Policy{MinimumTotalRequests: 100, MinimumCassandraHits: 90, MaximumFallbackBPS: 500, MaximumMissingBPS: 0, MaximumConflictBPS: 0, MaximumErrorBPS: 100, MaximumP95LatencyMicros: 10000})
	if err != nil || r.Decision != "blocked" || !has(r.Reasons, "cassandra_hit_below_minimum") || !has(r.Reasons, "missing_exceeded") {
		t.Fatalf("report=%+v err=%v", r, err)
	}
}

func TestEvaluateRejectsInvalidCounts(t *testing.T) {
	e := validEvidence()
	e.Requests.Total = 1
	e.Requests.CassandraHit = 2
	if _, err := Evaluate(e, Policy{}); err == nil {
		t.Fatal("expected invalid counts")
	}
}

func validEvidence() Evidence {
	return Evidence{SchemaVersion: "dipole.sync-cassandra-hydration-evidence.v1", Service: "sync-service", DeploymentRevision: "sync@1", Mode: "shadow", WindowStart: "2026-08-29T00:00:00Z", WindowEnd: "2026-08-29T01:00:00Z", Requests: Counts{Total: 100, CassandraHit: 98}, Latency: Latency{CassandraP95Micros: 4000}}
}
func has(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
