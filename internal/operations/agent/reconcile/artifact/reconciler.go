package artifactreconcile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	SchemaVersionV1  = "dipole.agent.artifact.reconcile.v1"
	MinimumSafeAgeV1 = 24 * time.Hour
)

type ObjectEvidenceV1 struct {
	Key          string
	SizeBytes    int64
	LastModified time.Time
	ETag         string
}

type ObjectSourceV1 interface {
	Walk(context.Context, string, func(ObjectEvidenceV1) error) error
}

type MetadataStoreV1 interface {
	ExistsByObjectKey(context.Context, string, string) (bool, error)
}

type ConfigV1 struct {
	Bucket      string
	Prefix      string
	MinimumAge  time.Duration
	MaxExamples int
}

type ExampleV1 struct {
	Kind               string    `json:"kind"`
	ObjectKey          string    `json:"object_key"`
	ContentSHA256      string    `json:"content_sha256,omitempty"`
	SizeBytes          int64     `json:"size_bytes"`
	ETag               string    `json:"etag"`
	LastModified       time.Time `json:"last_modified"`
	EligibleAfter      time.Time `json:"eligible_after"`
	EligibleForCleanup bool      `json:"eligible_for_cleanup"`
}

type ReportV1 struct {
	SchemaVersion        string      `json:"schema_version"`
	Mode                 string      `json:"mode"`
	DeleteAuthorized     bool        `json:"delete_authorized"`
	Bucket               string      `json:"bucket"`
	Prefix               string      `json:"prefix"`
	MinimumAgeSeconds    int64       `json:"minimum_age_seconds"`
	StartedAt            time.Time   `json:"started_at"`
	CompletedAt          time.Time   `json:"completed_at"`
	ScannedCount         uint64      `json:"scanned_count"`
	ReferencedCount      uint64      `json:"referenced_count"`
	YoungCount           uint64      `json:"young_count"`
	OrphanCandidateCount uint64      `json:"orphan_candidate_count"`
	UnsafeCount          uint64      `json:"unsafe_count"`
	Consistent           bool        `json:"consistent"`
	Examples             []ExampleV1 `json:"examples,omitempty"`
	EvidenceSHA256       string      `json:"evidence_sha256"`
}

type ReconcilerV1 struct {
	source   ObjectSourceV1
	metadata MetadataStoreV1
	config   ConfigV1
	now      func() time.Time
}

func New(source ObjectSourceV1, metadata MetadataStoreV1, cfg ConfigV1) (*ReconcilerV1, error) {
	cfg.Bucket = strings.TrimSpace(cfg.Bucket)
	cfg.Prefix = strings.TrimSpace(cfg.Prefix)
	switch {
	case source == nil:
		return nil, errors.New("Artifact reconcile object source is required")
	case metadata == nil:
		return nil, errors.New("Artifact reconcile metadata store is required")
	case cfg.Bucket == "":
		return nil, errors.New("Artifact reconcile bucket is required")
	case cfg.Prefix != "agent-artifacts/v1/":
		return nil, errors.New("Artifact reconcile prefix must remain version bound")
	case cfg.MinimumAge < MinimumSafeAgeV1:
		return nil, fmt.Errorf("Artifact reconcile minimum age must be at least %s", MinimumSafeAgeV1)
	case cfg.MaxExamples <= 0 || cfg.MaxExamples > 1000:
		return nil, errors.New("Artifact reconcile examples must be within 1..1000")
	}
	return &ReconcilerV1{source: source, metadata: metadata, config: cfg, now: time.Now}, nil
}

func (r *ReconcilerV1) Run(ctx context.Context) (ReportV1, error) {
	started := r.now().UTC()
	report := ReportV1{
		SchemaVersion: SchemaVersionV1, Mode: "dry_run", DeleteAuthorized: false,
		Bucket: r.config.Bucket, Prefix: r.config.Prefix,
		MinimumAgeSeconds: int64(r.config.MinimumAge / time.Second), StartedAt: started,
	}
	examples := make([]ExampleV1, 0)
	err := r.source.Walk(ctx, r.config.Prefix, func(object ObjectEvidenceV1) error {
		report.ScannedCount++
		contentHash, valid := artifactContentHash(r.config.Prefix, object)
		if !valid {
			report.UnsafeCount++
			examples = append(examples, evidenceExample("invalid_object_key", object, r.config.MinimumAge, "", false))
			return nil
		}
		if object.LastModified.Add(r.config.MinimumAge).After(started) {
			report.YoungCount++
			return nil
		}
		exists, err := r.metadata.ExistsByObjectKey(ctx, r.config.Bucket, object.Key)
		if err != nil {
			return fmt.Errorf("lookup Artifact metadata for %s: %w", object.Key, err)
		}
		if exists {
			report.ReferencedCount++
			return nil
		}
		report.OrphanCandidateCount++
		examples = append(examples, evidenceExample("orphan_candidate", object, r.config.MinimumAge, contentHash, true))
		return nil
	})
	if err != nil {
		return report, fmt.Errorf("walk Agent Artifact objects: %w", err)
	}
	sort.Slice(examples, func(i, j int) bool {
		if examples[i].Kind != examples[j].Kind {
			return examples[i].Kind > examples[j].Kind
		}
		return examples[i].ObjectKey < examples[j].ObjectKey
	})
	if len(examples) > r.config.MaxExamples {
		examples = examples[:r.config.MaxExamples]
	}
	report.Examples = examples
	report.CompletedAt = r.now().UTC()
	report.Consistent = report.OrphanCandidateCount == 0 && report.UnsafeCount == 0
	report.EvidenceSHA256 = report.evidenceHash()
	return report, nil
}

func (r ReportV1) VerifyEvidence() error {
	if r.SchemaVersion != SchemaVersionV1 || r.Mode != "dry_run" || r.DeleteAuthorized || r.EvidenceSHA256 == "" {
		return errors.New("invalid Artifact reconcile report contract")
	}
	if r.evidenceHash() != r.EvidenceSHA256 {
		return errors.New("Artifact reconcile evidence hash mismatch")
	}
	return nil
}

func (r ReportV1) evidenceHash() string {
	r.EvidenceSHA256 = ""
	body, _ := json.Marshal(r)
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:])
}

func artifactContentHash(prefix string, object ObjectEvidenceV1) (string, bool) {
	if object.SizeBytes <= 0 || object.LastModified.IsZero() || !strings.HasPrefix(object.Key, prefix) {
		return "", false
	}
	parts := strings.Split(strings.TrimPrefix(object.Key, prefix), "/")
	if len(parts) != 4 || parts[0] == "" || parts[1] == "" || !isSHA256(parts[2]) || !isSHA256(parts[3]) {
		return "", false
	}
	return strings.ToLower(parts[3]), true
}

func isSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func evidenceExample(kind string, object ObjectEvidenceV1, minimumAge time.Duration, contentHash string, eligible bool) ExampleV1 {
	return ExampleV1{
		Kind: kind, ObjectKey: object.Key, ContentSHA256: contentHash,
		SizeBytes: object.SizeBytes, ETag: strings.Trim(strings.TrimSpace(object.ETag), `"`),
		LastModified: object.LastModified.UTC(), EligibleAfter: object.LastModified.Add(minimumAge).UTC(),
		EligibleForCleanup: eligible,
	}
}
