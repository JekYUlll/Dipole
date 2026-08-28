package agenttimelinerepair

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

type Evidence struct {
	SchemaVersion      string        `json:"schemaVersion"`
	Service            string        `json:"service"`
	DeploymentRevision string        `json:"deploymentRevision"`
	WindowStart        string        `json:"windowStart"`
	WindowEnd          string        `json:"windowEnd"`
	WorkerReady        bool          `json:"workerReady"`
	Operator           string        `json:"operator"`
	RollbackVerified   bool          `json:"rollbackVerified"`
	AlertState         string        `json:"alertState"`
	Outcomes           OutcomeCounts `json:"outcomes"`
}

type OutcomeCounts struct {
	Claimed         uint64 `json:"claimed"`
	Repaired        uint64 `json:"repaired"`
	Retried         uint64 `json:"retried"`
	ProjectionError uint64 `json:"projectionError"`
	CompleteError   uint64 `json:"completeError"`
	ClaimError      uint64 `json:"claimError"`
	Invalid         uint64 `json:"invalid"`
}

type Policy struct {
	MinimumWindowMinutes      uint64 `json:"minimumWindowMinutes"`
	MinimumClaimed            uint64 `json:"minimumClaimed"`
	MaximumRetryRatioBPS      uint64 `json:"maximumRetryRatioBps"`
	MaximumProjectionErrorBPS uint64 `json:"maximumProjectionErrorBps"`
	MaximumCompleteErrorBPS   uint64 `json:"maximumCompleteErrorBps"`
	MaximumClaimErrorBPS      uint64 `json:"maximumClaimErrorBps"`
	MaximumInvalid            uint64 `json:"maximumInvalid"`
	RequireRollbackVerified   bool   `json:"requireRollbackVerified"`
	RequireClearAlerts        bool   `json:"requireClearAlerts"`
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
	WindowMinutes           uint64 `json:"windowMinutes"`
	Claimed                 uint64 `json:"claimed"`
	Repaired                uint64 `json:"repaired"`
	RetryRatioBPS           uint64 `json:"retryRatioBps"`
	ProjectionErrorRatioBPS uint64 `json:"projectionErrorRatioBps"`
	CompleteErrorRatioBPS   uint64 `json:"completeErrorRatioBps"`
	ClaimErrorRatioBPS      uint64 `json:"claimErrorRatioBps"`
	Invalid                 uint64 `json:"invalid"`
}

func ParseEvidence(data []byte) (Evidence, error) {
	var value Evidence
	if err := decodeStrict(data, &value); err != nil {
		return Evidence{}, fmt.Errorf("parse Agent Timeline repair rollout evidence: %w", err)
	}
	return value, validateEvidence(value)
}

func ParsePolicy(data []byte) (Policy, error) {
	var value Policy
	if err := decodeStrict(data, &value); err != nil {
		return Policy{}, fmt.Errorf("parse Agent Timeline repair rollout policy: %w", err)
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
	start, _ := time.Parse(time.RFC3339, evidence.WindowStart)
	end, _ := time.Parse(time.RFC3339, evidence.WindowEnd)
	claimed := evidence.Outcomes.Claimed
	metrics := ReportMetrics{
		WindowMinutes: uint64(end.Sub(start) / time.Minute), Claimed: claimed, Repaired: evidence.Outcomes.Repaired,
		RetryRatioBPS:           ratioBPS(evidence.Outcomes.Retried, claimed),
		ProjectionErrorRatioBPS: ratioBPS(evidence.Outcomes.ProjectionError, claimed),
		CompleteErrorRatioBPS:   ratioBPS(evidence.Outcomes.CompleteError, claimed),
		ClaimErrorRatioBPS:      ratioBPS(evidence.Outcomes.ClaimError, claimed), Invalid: evidence.Outcomes.Invalid,
	}
	reasons := make([]string, 0, 8)
	if metrics.WindowMinutes < policy.MinimumWindowMinutes {
		reasons = append(reasons, "window_below_minimum")
	}
	if claimed < policy.MinimumClaimed {
		reasons = append(reasons, "insufficient_claims")
	}
	if metrics.RetryRatioBPS > policy.MaximumRetryRatioBPS {
		reasons = append(reasons, "retry_ratio_exceeded")
	}
	if metrics.ProjectionErrorRatioBPS > policy.MaximumProjectionErrorBPS {
		reasons = append(reasons, "projection_error_ratio_exceeded")
	}
	if metrics.CompleteErrorRatioBPS > policy.MaximumCompleteErrorBPS {
		reasons = append(reasons, "complete_error_ratio_exceeded")
	}
	if metrics.ClaimErrorRatioBPS > policy.MaximumClaimErrorBPS {
		reasons = append(reasons, "claim_error_ratio_exceeded")
	}
	if metrics.Invalid > policy.MaximumInvalid {
		reasons = append(reasons, "invalid_outcomes_exceeded")
	}
	if !evidence.WorkerReady {
		reasons = append(reasons, "worker_not_ready")
	}
	if evidence.Operator == "" {
		reasons = append(reasons, "operator_missing")
	}
	if policy.RequireRollbackVerified && !evidence.RollbackVerified {
		reasons = append(reasons, "rollback_not_verified")
	}
	if policy.RequireClearAlerts && evidence.AlertState != "clear" {
		reasons = append(reasons, "alerts_not_clear")
	}
	sort.Strings(reasons)
	return Report{
		SchemaVersion: "dipole.agent.timeline-repair-rollout-report.v1", Decision: decision(reasons), Reasons: reasons,
		EvidenceSha256: digest(evidence), PolicySha256: digest(policy), DeploymentRevision: evidence.DeploymentRevision, Metrics: metrics,
	}, nil
}

func validateEvidence(value Evidence) error {
	if value.SchemaVersion != "dipole.agent.timeline-repair-rollout-evidence.v1" || value.Service == "" || value.DeploymentRevision == "" || value.Operator == "" {
		return fmt.Errorf("Agent Timeline repair rollout evidence identity is invalid")
	}
	start, err := time.Parse(time.RFC3339, value.WindowStart)
	if err != nil {
		return fmt.Errorf("repair rollout window start is invalid: %w", err)
	}
	end, err := time.Parse(time.RFC3339, value.WindowEnd)
	if err != nil || !end.After(start) {
		return fmt.Errorf("repair rollout window is invalid")
	}
	counts := value.Outcomes
	if counts.Claimed == 0 || counts.Repaired+counts.Retried > counts.Claimed || counts.ProjectionError > counts.Retried || counts.CompleteError > counts.Claimed || counts.ClaimError > counts.Claimed {
		return fmt.Errorf("repair rollout outcome counts are inconsistent")
	}
	if value.AlertState != "clear" && value.AlertState != "warning" && value.AlertState != "critical" {
		return fmt.Errorf("repair rollout alert state is invalid")
	}
	return nil
}

func validatePolicy(value Policy) error {
	if value.MaximumRetryRatioBPS > 10_000 || value.MaximumProjectionErrorBPS > 10_000 || value.MaximumCompleteErrorBPS > 10_000 || value.MaximumClaimErrorBPS > 10_000 {
		return fmt.Errorf("repair rollout policy ratio is invalid")
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

func decodeStrict(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	return nil
}
