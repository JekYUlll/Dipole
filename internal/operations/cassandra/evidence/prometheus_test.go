package cassandrareadrollout

import (
	"strings"
	"testing"
)

func TestEvidenceFromPrometheusWindow(t *testing.T) {
	evidence, err := EvidenceFromPrometheusWindow([]byte(cassandraReadMetricsStart), []byte(cassandraReadMetricsEnd), PrometheusSnapshotMetadata{
		Service: "message-service", DeploymentRevision: "message@2026-08-31", ConfiguredReadPercentage: 50,
		WindowStart: "2026-08-31T00:00:00Z", WindowEnd: "2026-08-31T00:05:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Requests != (RouteCounts{Total: 11, Cassandra: 5, MySQL: 6, Fallback: 2, VerificationSamples: 5, VerificationMismatch: 1, VerificationErrors: 1}) {
		t.Fatalf("unexpected route counts: %+v", evidence.Requests)
	}
	if evidence.Latency.CassandraP95Micros != 5_000 || evidence.Latency.MySQLP95Micros != 20_000 {
		t.Fatalf("unexpected p95 latency: %+v", evidence.Latency)
	}
}

func TestEvidenceFromPrometheusWindowRejectsCounterRegression(t *testing.T) {
	end := strings.Replace(cassandraReadMetricsEnd, "route=\"cassandra\",fallback_reason=\"\"} 15", "route=\"cassandra\",fallback_reason=\"\"} 9", 1)
	_, err := EvidenceFromPrometheusWindow([]byte(cassandraReadMetricsStart), []byte(end), PrometheusSnapshotMetadata{
		Service: "message-service", DeploymentRevision: "message@2026-08-31", ConfiguredReadPercentage: 50,
		WindowStart: "2026-08-31T00:00:00Z", WindowEnd: "2026-08-31T00:05:00Z",
	})
	if err == nil || !strings.Contains(err.Error(), "moved backwards") {
		t.Fatalf("expected counter regression error, got %v", err)
	}
}

func TestEvidenceFromPrometheusWindowAcceptsNewRouteHistogram(t *testing.T) {
	start := strings.Replace(cassandraReadMetricsStart, "dipole_message_read_route_total{route=\"mysql_fallback\",fallback_reason=\"cassandra_error\"} 3\n", "", 1)
	start = strings.Replace(start, "dipole_message_read_route_duration_seconds_bucket{route=\"mysql_fallback\",le=\"0.01\"} 0\ndipole_message_read_route_duration_seconds_bucket{route=\"mysql_fallback\",le=\"0.02\"} 3\ndipole_message_read_route_duration_seconds_bucket{route=\"mysql_fallback\",le=\"+Inf\"} 3\ndipole_message_read_route_duration_seconds_sum{route=\"mysql_fallback\"} 0.045\ndipole_message_read_route_duration_seconds_count{route=\"mysql_fallback\"} 3\n", "", 1)
	_, err := EvidenceFromPrometheusWindow([]byte(start), []byte(cassandraReadMetricsEnd), PrometheusSnapshotMetadata{
		Service: "message-service", DeploymentRevision: "message@2026-08-31", ConfiguredReadPercentage: 50,
		WindowStart: "2026-08-31T00:00:00Z", WindowEnd: "2026-08-31T00:05:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
}

const cassandraReadMetricsStart = `# TYPE dipole_message_read_route_total counter
dipole_message_read_route_total{route="cassandra",fallback_reason=""} 10
dipole_message_read_route_total{route="mysql",fallback_reason=""} 20
dipole_message_read_route_total{route="mysql_fallback",fallback_reason="cassandra_error"} 3
# TYPE dipole_message_read_verification_total counter
dipole_message_read_verification_total{operation="after_seq",outcome="match"} 4
dipole_message_read_verification_total{operation="after_seq",outcome="mismatch"} 1
dipole_message_read_verification_total{operation="before_seq",outcome="mysql_error"} 2
# TYPE dipole_message_read_route_duration_seconds histogram
dipole_message_read_route_duration_seconds_bucket{route="cassandra",le="0.005"} 10
dipole_message_read_route_duration_seconds_bucket{route="cassandra",le="0.01"} 10
dipole_message_read_route_duration_seconds_bucket{route="cassandra",le="+Inf"} 10
dipole_message_read_route_duration_seconds_sum{route="cassandra"} 0.03
dipole_message_read_route_duration_seconds_count{route="cassandra"} 10
dipole_message_read_route_duration_seconds_bucket{route="mysql",le="0.01"} 20
dipole_message_read_route_duration_seconds_bucket{route="mysql",le="0.02"} 20
dipole_message_read_route_duration_seconds_bucket{route="mysql",le="+Inf"} 20
dipole_message_read_route_duration_seconds_sum{route="mysql"} 0.1
dipole_message_read_route_duration_seconds_count{route="mysql"} 20
dipole_message_read_route_duration_seconds_bucket{route="mysql_fallback",le="0.01"} 0
dipole_message_read_route_duration_seconds_bucket{route="mysql_fallback",le="0.02"} 3
dipole_message_read_route_duration_seconds_bucket{route="mysql_fallback",le="+Inf"} 3
dipole_message_read_route_duration_seconds_sum{route="mysql_fallback"} 0.045
dipole_message_read_route_duration_seconds_count{route="mysql_fallback"} 3
`

const cassandraReadMetricsEnd = `# TYPE dipole_message_read_route_total counter
dipole_message_read_route_total{route="cassandra",fallback_reason=""} 15
dipole_message_read_route_total{route="mysql",fallback_reason=""} 24
dipole_message_read_route_total{route="mysql_fallback",fallback_reason="cassandra_error"} 5
# TYPE dipole_message_read_verification_total counter
dipole_message_read_verification_total{operation="after_seq",outcome="match"} 7
dipole_message_read_verification_total{operation="after_seq",outcome="mismatch"} 2
dipole_message_read_verification_total{operation="before_seq",outcome="mysql_error"} 3
# TYPE dipole_message_read_route_duration_seconds histogram
dipole_message_read_route_duration_seconds_bucket{route="cassandra",le="0.005"} 15
dipole_message_read_route_duration_seconds_bucket{route="cassandra",le="0.01"} 15
dipole_message_read_route_duration_seconds_bucket{route="cassandra",le="+Inf"} 15
dipole_message_read_route_duration_seconds_sum{route="cassandra"} 0.045
dipole_message_read_route_duration_seconds_count{route="cassandra"} 15
dipole_message_read_route_duration_seconds_bucket{route="mysql",le="0.01"} 24
dipole_message_read_route_duration_seconds_bucket{route="mysql",le="0.02"} 24
dipole_message_read_route_duration_seconds_bucket{route="mysql",le="+Inf"} 24
dipole_message_read_route_duration_seconds_sum{route="mysql"} 0.12
dipole_message_read_route_duration_seconds_count{route="mysql"} 24
dipole_message_read_route_duration_seconds_bucket{route="mysql_fallback",le="0.01"} 0
dipole_message_read_route_duration_seconds_bucket{route="mysql_fallback",le="0.02"} 5
dipole_message_read_route_duration_seconds_bucket{route="mysql_fallback",le="+Inf"} 5
dipole_message_read_route_duration_seconds_sum{route="mysql_fallback"} 0.075
dipole_message_read_route_duration_seconds_count{route="mysql_fallback"} 5
`
