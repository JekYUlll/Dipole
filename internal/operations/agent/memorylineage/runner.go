package memorylineage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	StatusRunning   = "running"
	StatusFailed    = "failed"
	StatusCompleted = "completed"
	MaxBatchSize    = 10_000
	maxLeaseSeconds = int64(^uint32(0) >> 1)
)

var (
	ErrLeaseHeld = errors.New("Memory lineage backfill lease is held by another owner")
	ErrLeaseLost = errors.New("Memory lineage backfill lease was lost")
)

type Reference struct {
	MemoryUUID     string
	TaskUUID       string
	Representation string
}

type SourceItem struct {
	SourceID   uint64
	References []Reference
}

type Source interface {
	HighWatermark(context.Context) (uint64, error)
	ListAfter(context.Context, uint64, uint64, int) ([]SourceItem, error)
}

type Checkpoint struct {
	HighWatermarkID uint64
	LastProcessedID uint64
	Status          string
}

type CheckpointStore interface {
	Acquire(context.Context, string, string, uint64, time.Duration) (Checkpoint, error)
	Advance(context.Context, string, string, uint64, time.Duration) error
	Fail(context.Context, string, string, error) error
	Complete(context.Context, string, string) error
}

type Target interface {
	Apply(context.Context, Reference) (inserted bool, duplicate bool, err error)
}

type Config struct {
	JobName       string
	OwnerID       string
	BatchSize     int
	LeaseDuration time.Duration
}

type Result struct {
	HighWatermarkID uint64
	LastProcessedID uint64
	Processed       uint64
	References      uint64
	Inserted        uint64
	Duplicates      uint64
}

type Runner struct {
	source      Source
	checkpoints CheckpointStore
	target      Target
	config      Config
}

func NewRunner(source Source, checkpoints CheckpointStore, target Target, cfg Config) (*Runner, error) {
	switch {
	case source == nil:
		return nil, errors.New("Memory lineage backfill source is required")
	case checkpoints == nil:
		return nil, errors.New("Memory lineage backfill checkpoint store is required")
	case target == nil:
		return nil, errors.New("Memory lineage backfill target is required")
	case strings.TrimSpace(cfg.JobName) == "":
		return nil, errors.New("Memory lineage backfill job name is required")
	case strings.TrimSpace(cfg.OwnerID) == "":
		return nil, errors.New("Memory lineage backfill owner ID is required")
	case cfg.BatchSize <= 0 || cfg.BatchSize > MaxBatchSize:
		return nil, fmt.Errorf("Memory lineage backfill batch size must be between 1 and %d", MaxBatchSize)
	case cfg.LeaseDuration < time.Second:
		return nil, errors.New("Memory lineage backfill lease duration must be at least one second")
	case int64(cfg.LeaseDuration/time.Second) > maxLeaseSeconds:
		return nil, errors.New("Memory lineage backfill lease duration exceeds MySQL interval range")
	}
	return &Runner{source: source, checkpoints: checkpoints, target: target, config: cfg}, nil
}

func (r *Runner) Run(ctx context.Context) (Result, error) {
	highWatermark, err := r.source.HighWatermark(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("read Memory lineage backfill high watermark: %w", err)
	}
	checkpoint, err := r.checkpoints.Acquire(ctx, r.config.JobName, r.config.OwnerID, highWatermark, r.config.LeaseDuration)
	if err != nil {
		return Result{}, fmt.Errorf("acquire Memory lineage backfill checkpoint: %w", err)
	}
	result := Result{HighWatermarkID: checkpoint.HighWatermarkID, LastProcessedID: checkpoint.LastProcessedID}
	if result.HighWatermarkID != highWatermark {
		return result, r.recordFailure(ctx, fmt.Errorf("Memory lineage backfill high watermark drift: source=%d checkpoint=%d", highWatermark, result.HighWatermarkID))
	}
	if checkpoint.Status == StatusCompleted {
		return result, nil
	}

	for result.LastProcessedID < result.HighWatermarkID {
		items, listErr := r.source.ListAfter(ctx, result.LastProcessedID, result.HighWatermarkID, r.config.BatchSize)
		if listErr != nil {
			return result, r.recordFailure(ctx, fmt.Errorf("list Memory lineage backfill source: %w", listErr))
		}
		if len(items) == 0 {
			return result, r.recordFailure(ctx, fmt.Errorf("Memory lineage backfill source ended before high watermark %d", result.HighWatermarkID))
		}

		lastID := result.LastProcessedID
		var batchReferences, batchInserted, batchDuplicates uint64
		for _, item := range items {
			if item.SourceID <= lastID || item.SourceID > result.HighWatermarkID {
				return result, r.recordFailure(ctx, fmt.Errorf("Memory lineage backfill source ID %d is outside (%d, %d]", item.SourceID, lastID, result.HighWatermarkID))
			}
			if len(item.References) == 0 {
				return result, r.recordFailure(ctx, fmt.Errorf("Memory lineage backfill source ID %d has no references", item.SourceID))
			}
			for _, reference := range item.References {
				if err := validateReference(reference); err != nil {
					return result, r.recordFailure(ctx, fmt.Errorf("Memory lineage backfill source ID %d: %w", item.SourceID, err))
				}
				inserted, duplicate, applyErr := r.target.Apply(ctx, reference)
				if applyErr != nil {
					return result, r.recordFailure(ctx, fmt.Errorf("apply Memory lineage %s/%s: %w", reference.MemoryUUID, reference.TaskUUID, applyErr))
				}
				if inserted {
					batchInserted++
				}
				if duplicate {
					batchDuplicates++
				}
				batchReferences++
			}
			lastID = item.SourceID
		}
		if err := r.checkpoints.Advance(ctx, r.config.JobName, r.config.OwnerID, lastID, r.config.LeaseDuration); err != nil {
			return result, r.recordFailure(ctx, fmt.Errorf("advance Memory lineage backfill checkpoint: %w", err))
		}
		result.LastProcessedID = lastID
		result.Processed += uint64(len(items))
		result.References += batchReferences
		result.Inserted += batchInserted
		result.Duplicates += batchDuplicates
	}
	if err := r.checkpoints.Complete(ctx, r.config.JobName, r.config.OwnerID); err != nil {
		return result, r.recordFailure(ctx, fmt.Errorf("complete Memory lineage backfill checkpoint: %w", err))
	}
	return result, nil
}

func (r *Runner) recordFailure(ctx context.Context, cause error) error {
	if err := r.checkpoints.Fail(ctx, r.config.JobName, r.config.OwnerID, cause); err != nil {
		return errors.Join(cause, fmt.Errorf("record Memory lineage backfill failure: %w", err))
	}
	return cause
}

func validateReference(reference Reference) error {
	if strings.TrimSpace(reference.MemoryUUID) == "" || strings.TrimSpace(reference.TaskUUID) == "" {
		return errors.New("lineage reference identity is required")
	}
	if reference.Representation != "full" && reference.Representation != "compact" {
		return fmt.Errorf("lineage representation %q is invalid", reference.Representation)
	}
	return nil
}
