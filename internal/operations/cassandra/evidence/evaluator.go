package cassandrareadrollout

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

type Evidence struct {
	SchemaVersion            string       `json:"schemaVersion"`
	Service                  string       `json:"service"`
	DeploymentRevision       string       `json:"deploymentRevision"`
	WindowStart              string       `json:"windowStart"`
	WindowEnd                string       `json:"windowEnd"`
	ConfiguredReadPercentage int          `json:"configuredReadPercentage"`
	Requests                 RouteCounts  `json:"requests"`
	Latency                  RouteLatency `json:"latency"`
}

type RouteCounts struct {
	Total                uint64 `json:"total"`
	Cassandra            uint64 `json:"cassandra"`
	MySQL                uint64 `json:"mysql"`
	Fallback             uint64 `json:"fallback"`
	VerificationSamples  uint64 `json:"verificationSamples"`
	VerificationMismatch uint64 `json:"verificationMismatch"`
	VerificationErrors   uint64 `json:"verificationErrors"`
}

type RouteLatency struct {
	CassandraP95Micros uint64 `json:"cassandraP95Micros"`
	MySQLP95Micros     uint64 `json:"mysqlP95Micros"`
}

type Policy struct {
	MinimumTotalRequests             uint64 `json:"minimumTotalRequests"`
	MinimumCassandraRequests         uint64 `json:"minimumCassandraRequests"`
	MinimumObservedShareBPS          uint64 `json:"minimumObservedShareBps"`
	MaximumFallbackBPS               uint64 `json:"maximumFallbackBps"`
	MaximumVerificationMismatchBPS   uint64 `json:"maximumVerificationMismatchBps"`
	MaximumVerificationErrorBPS      uint64 `json:"maximumVerificationErrorBps"`
	MaximumCassandraP95LatencyMicros uint64 `json:"maximumCassandraP95LatencyMicros"`
}

type Report struct {
	SchemaVersion      string        `json:"schemaVersion"`
	Decision           string        `json:"decision"`
	Reasons            []string      `json:"reasons"`
	EvidenceSha256     string        `json:"evidenceSha256"`
	PolicySha256       string        `json:"policySha256"`
	DeploymentRevision string        `json:"deploymentRevision"`
	Metrics            ReportMetrics `json:"metrics"`
}

type ReportMetrics struct {
	TotalRequests                uint64 `json:"totalRequests"`
	CassandraRequests            uint64 `json:"cassandraRequests"`
	ObservedCassandraShareBPS    uint64 `json:"observedCassandraShareBps"`
	FallbackRatioBPS             uint64 `json:"fallbackRatioBps"`
	VerificationMismatchRatioBPS uint64 `json:"verificationMismatchRatioBps"`
	VerificationErrorRatioBPS    uint64 `json:"verificationErrorRatioBps"`
	CassandraP95LatencyMicros    uint64 `json:"cassandraP95LatencyMicros"`
}

func ParseEvidence(data []byte) (Evidence, error) {
	var value Evidence
	if err := json.Unmarshal(data, &value); err != nil {
		return Evidence{}, fmt.Errorf("parse Cassandra read rollout evidence: %w", err)
	}
	if err := validateEvidence(value); err != nil {
		return Evidence{}, err
	}
	return value, nil
}

func ParsePolicy(data []byte) (Policy, error) {
	var value Policy
	if err := json.Unmarshal(data, &value); err != nil {
		return Policy{}, fmt.Errorf("parse Cassandra read rollout policy: %w", err)
	}
	return value, validatePolicy(value)
}

func Evaluate(evidence Evidence, policy Policy) (Report, error) {
	if err := validateEvidence(evidence); err != nil {
		return Report{}, err
	}
	if err := validatePolicy(policy); err != nil {
		return Report{}, err
	}
	reasons := make([]string, 0, 5)
	if evidence.Requests.Total < policy.MinimumTotalRequests || evidence.Requests.Cassandra < policy.MinimumCassandraRequests {
		reasons = append(reasons, "insufficient_samples")
	}
	metrics := ReportMetrics{
		TotalRequests: evidence.Requests.Total, CassandraRequests: evidence.Requests.Cassandra,
		ObservedCassandraShareBPS:    ratioBPS(evidence.Requests.Cassandra, evidence.Requests.Total),
		FallbackRatioBPS:             ratioBPS(evidence.Requests.Fallback, evidence.Requests.Total),
		VerificationMismatchRatioBPS: ratioBPS(evidence.Requests.VerificationMismatch, evidence.Requests.VerificationSamples),
		VerificationErrorRatioBPS:    ratioBPS(evidence.Requests.VerificationErrors, evidence.Requests.VerificationSamples),
		CassandraP95LatencyMicros:    evidence.Latency.CassandraP95Micros,
	}
	if metrics.ObservedCassandraShareBPS < policy.MinimumObservedShareBPS {
		reasons = append(reasons, "observed_share_below_minimum")
	}
	if metrics.FallbackRatioBPS > policy.MaximumFallbackBPS {
		reasons = append(reasons, "fallback_ratio_exceeded")
	}
	if metrics.VerificationMismatchRatioBPS > policy.MaximumVerificationMismatchBPS {
		reasons = append(reasons, "verification_mismatch_exceeded")
	}
	if metrics.VerificationErrorRatioBPS > policy.MaximumVerificationErrorBPS {
		reasons = append(reasons, "verification_error_exceeded")
	}
	if metrics.CassandraP95LatencyMicros > policy.MaximumCassandraP95LatencyMicros {
		reasons = append(reasons, "cassandra_p95_latency_exceeded")
	}
	sort.Strings(reasons)
	return Report{
		SchemaVersion: "dipole.cassandra-read-rollout-report.v1", Decision: decision(reasons), Reasons: reasons,
		EvidenceSha256: digest(evidence), PolicySha256: digest(policy), DeploymentRevision: evidence.DeploymentRevision, Metrics: metrics,
	}, nil
}

func validateEvidence(value Evidence) error {
	if value.SchemaVersion != "dipole.cassandra-read-rollout-evidence.v1" || value.Service == "" || value.DeploymentRevision == "" {
		return fmt.Errorf("Cassandra read rollout evidence identity is invalid")
	}
	start, err := time.Parse(time.RFC3339, value.WindowStart)
	if err != nil {
		return fmt.Errorf("Cassandra read rollout window start is invalid: %w", err)
	}
	end, err := time.Parse(time.RFC3339, value.WindowEnd)
	if err != nil || !end.After(start) {
		return fmt.Errorf("Cassandra read rollout window is invalid")
	}
	if value.ConfiguredReadPercentage < 0 || value.ConfiguredReadPercentage > 100 {
		return fmt.Errorf("configured Cassandra read percentage is invalid")
	}
	counts := value.Requests
	if counts.Total == 0 || counts.Cassandra+counts.MySQL != counts.Total || counts.Fallback > counts.Cassandra || counts.VerificationMismatch+counts.VerificationErrors > counts.VerificationSamples || counts.VerificationSamples > counts.Total {
		return fmt.Errorf("Cassandra read rollout route counts are inconsistent")
	}
	return nil
}

func validatePolicy(value Policy) error {
	if value.MinimumObservedShareBPS > 10_000 || value.MaximumFallbackBPS > 10_000 || value.MaximumVerificationMismatchBPS > 10_000 || value.MaximumVerificationErrorBPS > 10_000 || value.MaximumCassandraP95LatencyMicros == 0 {
		return fmt.Errorf("Cassandra read rollout policy is invalid")
	}
	return nil
}

func ratioBPS(numerator, denominator uint64) uint64 {
	if denominator == 0 {
		return 0
	}
	return numerator * 10_000 / denominator
}
func decision(reasons []string) string {
	if len(reasons) == 0 {
		return "eligible"
	}
	return "blocked"
}
func digest(value any) string {
	data, _ := json.Marshal(value)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
