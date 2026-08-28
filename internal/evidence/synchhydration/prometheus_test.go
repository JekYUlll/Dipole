package synchhydration

import (
	"testing"
)

func TestEvidenceFromPrometheusAggregatesRoutesAndConservativeP95(t *testing.T) {
	data := []byte(`# HELP dipole_sync_hydration_route_total Sync message hydration requests by selected route outcome.
# TYPE dipole_sync_hydration_route_total counter
dipole_sync_hydration_route_total{outcome="hit"} 95
dipole_sync_hydration_route_total{outcome="fallback"} 3
dipole_sync_hydration_route_total{outcome="cancelled"} 1
dipole_sync_hydration_route_total{outcome="error"} 1
# HELP dipole_sync_hydration_route_duration_seconds Sync message hydration request duration by route outcome.
# TYPE dipole_sync_hydration_route_duration_seconds histogram
dipole_sync_hydration_route_duration_seconds_bucket{outcome="hit",le="0.001"} 50
dipole_sync_hydration_route_duration_seconds_bucket{outcome="hit",le="0.005"} 95
dipole_sync_hydration_route_duration_seconds_bucket{outcome="hit",le="+Inf"} 95
dipole_sync_hydration_route_duration_seconds_sum{outcome="hit"} 0.2
dipole_sync_hydration_route_duration_seconds_count{outcome="hit"} 95
`)
	evidence, err := EvidenceFromPrometheus(data, PrometheusSnapshotMetadata{Service: "sync-service", DeploymentRevision: "sync@1", Mode: "primary", WindowStart: "2026-08-29T00:00:00Z", WindowEnd: "2026-08-29T01:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Requests != (Counts{Total: 100, CassandraHit: 95, MySQLFallback: 3, Error: 2}) {
		t.Fatalf("counts = %+v", evidence.Requests)
	}
	if evidence.Latency.CassandraP95Micros != 5000 {
		t.Fatalf("p95 = %d, want 5000", evidence.Latency.CassandraP95Micros)
	}
}

func TestEvidenceFromPrometheusRejectsMissingHitHistogram(t *testing.T) {
	data := []byte(`# TYPE dipole_sync_hydration_route_total counter
dipole_sync_hydration_route_total{outcome="hit"} 1
`)
	_, err := EvidenceFromPrometheus(data, PrometheusSnapshotMetadata{Service: "sync", DeploymentRevision: "r1", Mode: "primary", WindowStart: "2026-08-29T00:00:00Z", WindowEnd: "2026-08-29T01:00:00Z"})
	if err == nil {
		t.Fatal("expected missing histogram error")
	}
}
