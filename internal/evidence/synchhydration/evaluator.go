package synchhydration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

type Evidence struct {
	SchemaVersion      string  `json:"schemaVersion"`
	Service            string  `json:"service"`
	DeploymentRevision string  `json:"deploymentRevision"`
	Mode               string  `json:"mode"`
	WindowStart        string  `json:"windowStart"`
	WindowEnd          string  `json:"windowEnd"`
	Requests           Counts  `json:"requests"`
	Latency            Latency `json:"latency"`
}
type Counts struct {
	Total         uint64 `json:"total"`
	CassandraHit  uint64 `json:"cassandraHit"`
	MySQLFallback uint64 `json:"mysqlFallback"`
	Missing       uint64 `json:"missing"`
	Conflict      uint64 `json:"conflict"`
	Error         uint64 `json:"error"`
}
type Latency struct {
	CassandraP95Micros uint64 `json:"cassandraP95Micros"`
}
type Policy struct {
	MinimumTotalRequests    uint64 `json:"minimumTotalRequests"`
	MinimumCassandraHits    uint64 `json:"minimumCassandraHits"`
	MaximumFallbackBPS      uint64 `json:"maximumFallbackBps"`
	MaximumMissingBPS       uint64 `json:"maximumMissingBps"`
	MaximumConflictBPS      uint64 `json:"maximumConflictBps"`
	MaximumErrorBPS         uint64 `json:"maximumErrorBps"`
	MaximumP95LatencyMicros uint64 `json:"maximumP95LatencyMicros"`
}
type Report struct {
	SchemaVersion      string   `json:"schemaVersion"`
	Decision           string   `json:"decision"`
	Reasons            []string `json:"reasons"`
	EvidenceSha256     string   `json:"evidenceSha256"`
	PolicySha256       string   `json:"policySha256"`
	DeploymentRevision string   `json:"deploymentRevision"`
	Mode               string   `json:"mode"`
	Metrics            Metrics  `json:"metrics"`
}
type Metrics struct {
	TotalRequests      uint64 `json:"totalRequests"`
	CassandraHitBPS    uint64 `json:"cassandraHitBps"`
	FallbackBPS        uint64 `json:"fallbackBps"`
	MissingBPS         uint64 `json:"missingBps"`
	ConflictBPS        uint64 `json:"conflictBps"`
	ErrorBPS           uint64 `json:"errorBps"`
	CassandraP95Micros uint64 `json:"cassandraP95Micros"`
}

func ParseEvidence(data []byte) (Evidence, error) {
	var v Evidence
	if err := json.Unmarshal(data, &v); err != nil {
		return v, fmt.Errorf("parse Sync hydration evidence: %w", err)
	}
	return v, validateEvidence(v)
}
func ParsePolicy(data []byte) (Policy, error) {
	var v Policy
	if err := json.Unmarshal(data, &v); err != nil {
		return v, fmt.Errorf("parse Sync hydration policy: %w", err)
	}
	return v, validatePolicy(v)
}
func Evaluate(e Evidence, p Policy) (Report, error) {
	if err := validateEvidence(e); err != nil {
		return Report{}, err
	}
	if err := validatePolicy(p); err != nil {
		return Report{}, err
	}
	c := e.Requests
	m := Metrics{TotalRequests: c.Total, CassandraHitBPS: ratio(c.CassandraHit, c.Total), FallbackBPS: ratio(c.MySQLFallback, c.Total), MissingBPS: ratio(c.Missing, c.Total), ConflictBPS: ratio(c.Conflict, c.Total), ErrorBPS: ratio(c.Error, c.Total), CassandraP95Micros: e.Latency.CassandraP95Micros}
	reasons := []string{}
	if c.Total < p.MinimumTotalRequests {
		reasons = append(reasons, "insufficient_samples")
	}
	if c.CassandraHit < p.MinimumCassandraHits {
		reasons = append(reasons, "cassandra_hit_below_minimum")
	}
	if m.FallbackBPS > p.MaximumFallbackBPS {
		reasons = append(reasons, "fallback_exceeded")
	}
	if m.MissingBPS > p.MaximumMissingBPS {
		reasons = append(reasons, "missing_exceeded")
	}
	if m.ConflictBPS > p.MaximumConflictBPS {
		reasons = append(reasons, "conflict_exceeded")
	}
	if m.ErrorBPS > p.MaximumErrorBPS {
		reasons = append(reasons, "error_exceeded")
	}
	if m.CassandraP95Micros > p.MaximumP95LatencyMicros {
		reasons = append(reasons, "p95_latency_exceeded")
	}
	sort.Strings(reasons)
	return Report{SchemaVersion: "dipole.sync-cassandra-hydration-report.v1", Decision: map[bool]string{true: "eligible", false: "blocked"}[len(reasons) == 0], Reasons: reasons, EvidenceSha256: digest(e), PolicySha256: digest(p), DeploymentRevision: e.DeploymentRevision, Mode: e.Mode, Metrics: m}, nil
}
func validateEvidence(e Evidence) error {
	if e.SchemaVersion != "dipole.sync-cassandra-hydration-evidence.v1" || e.Service == "" || e.DeploymentRevision == "" || (e.Mode != "shadow" && e.Mode != "primary") {
		return fmt.Errorf("Sync hydration evidence identity is invalid")
	}
	start, err := time.Parse(time.RFC3339, e.WindowStart)
	if err != nil {
		return fmt.Errorf("Sync hydration window start is invalid")
	}
	end, err := time.Parse(time.RFC3339, e.WindowEnd)
	if err != nil || !end.After(start) {
		return fmt.Errorf("Sync hydration window is invalid")
	}
	c := e.Requests
	if c.Total == 0 || c.CassandraHit+c.MySQLFallback+c.Missing+c.Conflict+c.Error > c.Total {
		return fmt.Errorf("Sync hydration counts are inconsistent")
	}
	return nil
}
func validatePolicy(p Policy) error {
	if p.MaximumFallbackBPS > 10000 || p.MaximumMissingBPS > 10000 || p.MaximumConflictBPS > 10000 || p.MaximumErrorBPS > 10000 || p.MaximumP95LatencyMicros == 0 {
		return fmt.Errorf("Sync hydration policy is invalid")
	}
	return nil
}
func ratio(n, d uint64) uint64 {
	if d == 0 {
		return 0
	}
	return n * 10000 / d
}
func digest(v any) string {
	b, _ := json.Marshal(v)
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
