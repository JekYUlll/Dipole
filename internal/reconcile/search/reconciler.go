package searchreconcile

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	searchbackfill "github.com/JekYUlll/Dipole/internal/backfill/search"
	"github.com/JekYUlll/Dipole/internal/model"
)

type Source interface {
	ListAfter(context.Context, uint64, uint64, int) ([]searchbackfill.SourceMutation, error)
}

type Target interface {
	Lookup(context.Context, string) (model.MessageSearchState, bool, error)
	Count(context.Context) (uint64, error)
}

type Config struct {
	JobName         string
	HighWatermarkID uint64
	BatchSize       int
	MaxExamples     int
}

type MismatchExample struct {
	Kind         string `json:"kind"`
	MessageUUID  string `json:"message_uuid,omitempty"`
	ExpectedHash string `json:"expected_hash,omitempty"`
	ActualHash   string `json:"actual_hash,omitempty"`
}

type Report struct {
	JobName             string            `json:"job_name"`
	SourceHighWatermark uint64            `json:"source_high_watermark_id"`
	StartedAt           time.Time         `json:"started_at"`
	CompletedAt         time.Time         `json:"completed_at"`
	SourceCount         uint64            `json:"source_count"`
	TargetCount         uint64            `json:"target_count"`
	TargetFoundCount    uint64            `json:"target_found_count"`
	HashMatchedCount    uint64            `json:"hash_matched_count"`
	MissingCount        uint64            `json:"missing_count"`
	HashMismatchCount   uint64            `json:"hash_mismatch_count"`
	ExtraCount          uint64            `json:"extra_count"`
	Consistent          bool              `json:"consistent"`
	Examples            []MismatchExample `json:"examples,omitempty"`
}

type Reconciler struct {
	source Source
	target Target
	config Config
	now    func() time.Time
}

func New(source Source, target Target, cfg Config) (*Reconciler, error) {
	switch {
	case source == nil:
		return nil, errors.New("Search reconciliation source is required")
	case target == nil:
		return nil, errors.New("Search reconciliation target is required")
	case strings.TrimSpace(cfg.JobName) == "":
		return nil, errors.New("Search reconciliation job name is required")
	case cfg.BatchSize <= 0:
		return nil, errors.New("Search reconciliation batch size must be positive")
	case cfg.BatchSize > searchbackfill.MaxBatchSize:
		return nil, fmt.Errorf("Search reconciliation batch size cannot exceed %d", searchbackfill.MaxBatchSize)
	case cfg.MaxExamples < 0:
		return nil, errors.New("Search reconciliation max examples cannot be negative")
	}
	return &Reconciler{source: source, target: target, config: cfg, now: time.Now}, nil
}

func (r *Reconciler) Run(ctx context.Context) (Report, error) {
	report := Report{JobName: r.config.JobName, SourceHighWatermark: r.config.HighWatermarkID, StartedAt: r.now().UTC()}
	var afterID uint64
	for afterID < r.config.HighWatermarkID {
		items, err := r.source.ListAfter(ctx, afterID, r.config.HighWatermarkID, r.config.BatchSize)
		if err != nil {
			return report, fmt.Errorf("list Search reconciliation source: %w", err)
		}
		if len(items) == 0 {
			return report, fmt.Errorf("Search reconciliation source ended before high watermark %d", r.config.HighWatermarkID)
		}
		for _, item := range items {
			if item.SourceID <= afterID || item.SourceID > r.config.HighWatermarkID || item.Mutation == nil {
				return report, fmt.Errorf("Search reconciliation source ID %d is invalid after %d", item.SourceID, afterID)
			}
			if err := r.compare(ctx, &report, item.Mutation); err != nil {
				return report, err
			}
			afterID = item.SourceID
		}
	}
	targetCount, err := r.target.Count(ctx)
	if err != nil {
		return report, fmt.Errorf("count Search reconciliation target: %w", err)
	}
	report.TargetCount = targetCount
	if targetCount > report.TargetFoundCount {
		report.ExtraCount = targetCount - report.TargetFoundCount
	}
	report.CompletedAt = r.now().UTC()
	report.Consistent = report.SourceCount == report.TargetCount && report.TargetFoundCount == report.SourceCount &&
		report.HashMatchedCount == report.SourceCount && report.MissingCount == 0 && report.HashMismatchCount == 0 && report.ExtraCount == 0
	return report, nil
}

func (r *Reconciler) compare(ctx context.Context, report *Report, mutation *model.MessageSearchMutation) error {
	expected, err := mutation.State()
	if err != nil {
		return fmt.Errorf("normalize Search reconciliation source %s: %w", mutation.MessageUUID, err)
	}
	report.SourceCount++
	actual, found, err := r.target.Lookup(ctx, expected.MessageUUID)
	if err != nil {
		return fmt.Errorf("read Search reconciliation target %s: %w", expected.MessageUUID, err)
	}
	if !found {
		report.MissingCount++
		r.addExample(report, MismatchExample{Kind: "missing", MessageUUID: expected.MessageUUID, ExpectedHash: expected.PayloadHash})
		return nil
	}
	report.TargetFoundCount++
	if actual.Revision == expected.Revision && actual.Searchable == expected.Searchable && actual.PayloadHash == expected.PayloadHash {
		report.HashMatchedCount++
		return nil
	}
	report.HashMismatchCount++
	r.addExample(report, MismatchExample{Kind: "state_mismatch", MessageUUID: expected.MessageUUID, ExpectedHash: expected.PayloadHash, ActualHash: actual.PayloadHash})
	return nil
}

func (r *Reconciler) addExample(report *Report, example MismatchExample) {
	if len(report.Examples) < r.config.MaxExamples {
		report.Examples = append(report.Examples, example)
	}
}
