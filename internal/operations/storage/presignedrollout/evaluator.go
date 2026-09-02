// Package presignedrollout evaluates evidence before Multipart direct upload becomes the default.
package presignedrollout

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/bits"
	"sort"
	"time"
)

type Evidence struct {
	SchemaVersion      string        `json:"schemaVersion"`
	Service            string        `json:"service"`
	DeploymentRevision string        `json:"deploymentRevision"`
	PolicySHA256       string        `json:"policySha256"`
	WindowStart        string        `json:"windowStart"`
	WindowEnd          string        `json:"windowEnd"`
	Operator           string        `json:"operator"`
	Reviewer           string        `json:"reviewer"`
	RollbackVerified   bool          `json:"rollbackVerified"`
	AlertState         string        `json:"alertState"`
	Outcomes           OutcomeCounts `json:"outcomes"`
	Latency            Latency       `json:"latency"`
}

type OutcomeCounts struct {
	Attempted        uint64 `json:"attempted"`
	DirectCompleted  uint64 `json:"directCompleted"`
	RelayFallback    uint64 `json:"relayFallback"`
	Failed           uint64 `json:"failed"`
	Expired          uint64 `json:"expired"`
	Aborted          uint64 `json:"aborted"`
	Retried          uint64 `json:"retried"`
	ChecksumMismatch uint64 `json:"checksumMismatch"`
}

type Latency struct {
	DirectP95Millis uint64 `json:"directP95Millis"`
}

type Policy struct {
	MinimumWindowMinutes       uint64 `json:"minimumWindowMinutes"`
	MinimumAttempts            uint64 `json:"minimumAttempts"`
	MinimumDirectCompleted     uint64 `json:"minimumDirectCompleted"`
	MaximumRelayFallbackBPS    uint64 `json:"maximumRelayFallbackBps"`
	MaximumFailureBPS          uint64 `json:"maximumFailureBps"`
	MaximumExpiredBPS          uint64 `json:"maximumExpiredBps"`
	MaximumChecksumMismatchBPS uint64 `json:"maximumChecksumMismatchBps"`
	MaximumDirectP95Millis     uint64 `json:"maximumDirectP95Millis"`
	RequireRollbackVerified    bool   `json:"requireRollbackVerified"`
	RequireClearAlerts         bool   `json:"requireClearAlerts"`
	RequireIndependentReviewer bool   `json:"requireIndependentReviewer"`
}

type Report struct {
	SchemaVersion      string        `json:"schemaVersion"`
	Decision           string        `json:"decision"`
	Reasons            []string      `json:"reasons"`
	EvidenceSHA256     string        `json:"evidenceSha256"`
	PolicySHA256       string        `json:"policySha256"`
	DeploymentRevision string        `json:"deploymentRevision"`
	Metrics            ReportMetrics `json:"metrics"`
}

type ReportMetrics struct {
	WindowMinutes            uint64 `json:"windowMinutes"`
	Attempted                uint64 `json:"attempted"`
	DirectCompleted          uint64 `json:"directCompleted"`
	RelayFallbackRatioBPS    uint64 `json:"relayFallbackRatioBps"`
	FailureRatioBPS          uint64 `json:"failureRatioBps"`
	ExpiredRatioBPS          uint64 `json:"expiredRatioBps"`
	ChecksumMismatchRatioBPS uint64 `json:"checksumMismatchRatioBps"`
	DirectP95Millis          uint64 `json:"directP95Millis"`
}

func ParseEvidence(data []byte) (Evidence, error) {
	var value Evidence
	if err := decodeStrict(data, &value); err != nil {
		return Evidence{}, fmt.Errorf("parse Multipart presigned rollout evidence: %w", err)
	}
	return value, validateEvidence(value)
}

func ParsePolicy(data []byte) (Policy, error) {
	var value Policy
	if err := decodeStrict(data, &value); err != nil {
		return Policy{}, fmt.Errorf("parse Multipart presigned rollout policy: %w", err)
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
	metrics := ReportMetrics{
		WindowMinutes:            uint64(end.Sub(start) / time.Minute),
		Attempted:                evidence.Outcomes.Attempted,
		DirectCompleted:          evidence.Outcomes.DirectCompleted,
		RelayFallbackRatioBPS:    ratioBPS(evidence.Outcomes.RelayFallback, evidence.Outcomes.Attempted),
		FailureRatioBPS:          ratioBPS(evidence.Outcomes.Failed, evidence.Outcomes.Attempted),
		ExpiredRatioBPS:          ratioBPS(evidence.Outcomes.Expired, evidence.Outcomes.Attempted),
		ChecksumMismatchRatioBPS: ratioBPS(evidence.Outcomes.ChecksumMismatch, evidence.Outcomes.Attempted),
		DirectP95Millis:          evidence.Latency.DirectP95Millis,
	}
	reasons := make([]string, 0, 10)
	if metrics.WindowMinutes < policy.MinimumWindowMinutes {
		reasons = append(reasons, "window_below_minimum")
	}
	if metrics.Attempted < policy.MinimumAttempts || metrics.DirectCompleted < policy.MinimumDirectCompleted {
		reasons = append(reasons, "insufficient_direct_samples")
	}
	if metrics.RelayFallbackRatioBPS > policy.MaximumRelayFallbackBPS {
		reasons = append(reasons, "relay_fallback_ratio_exceeded")
	}
	if metrics.FailureRatioBPS > policy.MaximumFailureBPS {
		reasons = append(reasons, "failure_ratio_exceeded")
	}
	if metrics.ExpiredRatioBPS > policy.MaximumExpiredBPS {
		reasons = append(reasons, "expired_ratio_exceeded")
	}
	if metrics.ChecksumMismatchRatioBPS > policy.MaximumChecksumMismatchBPS {
		reasons = append(reasons, "checksum_mismatch_ratio_exceeded")
	}
	if metrics.DirectP95Millis > policy.MaximumDirectP95Millis {
		reasons = append(reasons, "direct_p95_latency_exceeded")
	}
	if evidence.PolicySHA256 != PolicySHA256(policy) {
		reasons = append(reasons, "policy_hash_mismatch")
	}
	if policy.RequireRollbackVerified && !evidence.RollbackVerified {
		reasons = append(reasons, "rollback_not_verified")
	}
	if policy.RequireClearAlerts && evidence.AlertState != "clear" {
		reasons = append(reasons, "alerts_not_clear")
	}
	if policy.RequireIndependentReviewer && evidence.Operator == evidence.Reviewer {
		reasons = append(reasons, "independent_review_missing")
	}
	sort.Strings(reasons)
	return Report{
		SchemaVersion: "dipole.multipart-presigned-rollout-report.v1", Decision: decision(reasons), Reasons: reasons,
		EvidenceSHA256: digest(evidence), PolicySHA256: digest(policy), DeploymentRevision: evidence.DeploymentRevision, Metrics: metrics,
	}, nil
}

func validateEvidence(value Evidence) error {
	if value.SchemaVersion != "dipole.multipart-presigned-rollout-evidence.v1" || value.Service == "" || value.DeploymentRevision == "" || len(value.PolicySHA256) != 64 || value.Operator == "" || value.Reviewer == "" {
		return fmt.Errorf("Multipart presigned rollout evidence identity is invalid")
	}
	if _, err := hex.DecodeString(value.PolicySHA256); err != nil {
		return fmt.Errorf("Multipart presigned rollout policy hash is invalid")
	}
	start, err := time.Parse(time.RFC3339, value.WindowStart)
	if err != nil {
		return fmt.Errorf("Multipart presigned rollout window start is invalid: %w", err)
	}
	end, err := time.Parse(time.RFC3339, value.WindowEnd)
	if err != nil || !end.After(start) {
		return fmt.Errorf("Multipart presigned rollout window is invalid")
	}
	counts := value.Outcomes
	terminal, overflow := terminalOutcomeCount(counts)
	if counts.Attempted == 0 || overflow || terminal != counts.Attempted || counts.ChecksumMismatch > counts.Failed || value.AlertState != "clear" && value.AlertState != "warning" && value.AlertState != "critical" {
		return fmt.Errorf("Multipart presigned rollout outcomes are inconsistent")
	}
	return nil
}

func terminalOutcomeCount(counts OutcomeCounts) (uint64, bool) {
	total, carry := bits.Add64(counts.DirectCompleted, counts.RelayFallback, 0)
	total, next := bits.Add64(total, counts.Failed, 0)
	carry |= next
	total, next = bits.Add64(total, counts.Expired, 0)
	carry |= next
	total, next = bits.Add64(total, counts.Aborted, 0)
	return total, carry != 0 || next != 0
}

func validatePolicy(value Policy) error {
	if value.MinimumWindowMinutes == 0 || value.MinimumAttempts == 0 || value.MinimumDirectCompleted == 0 || value.MinimumDirectCompleted > value.MinimumAttempts || value.MaximumDirectP95Millis == 0 || value.MaximumRelayFallbackBPS > 10_000 || value.MaximumFailureBPS > 10_000 || value.MaximumExpiredBPS > 10_000 || value.MaximumChecksumMismatchBPS > 10_000 {
		return fmt.Errorf("Multipart presigned rollout policy is invalid")
	}
	return nil
}

func ratioBPS(numerator, denominator uint64) uint64 {
	if denominator == 0 {
		return 0
	}
	quotient, remainder := numerator/denominator, numerator%denominator
	high, low := bits.Mul64(remainder, 10_000)
	fraction, _ := bits.Div64(high, low, denominator)
	return quotient*10_000 + fraction
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

// PolicySHA256 returns the canonical hash that an evidence record must bind to.
func PolicySHA256(policy Policy) string {
	return digest(policy)
}

func decodeStrict(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}
