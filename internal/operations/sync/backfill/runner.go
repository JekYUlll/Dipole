package syncbackfill

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
)

const (
	StatusRunning   = "running"
	StatusFailed    = "failed"
	StatusCompleted = "completed"
	MaxBatchSize    = 10_000
	maxLeaseSeconds = int64(^uint32(0) >> 1)
)

type SourceItem struct {
	SourceID   uint64
	Fanout     bool
	Projection *model.SyncProjection
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
	Apply(*model.SyncProjection) error
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
	Projected       uint64
	Skipped         uint64
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
		return nil, errors.New("Sync replay source is required")
	case checkpoints == nil:
		return nil, errors.New("Sync replay checkpoint store is required")
	case target == nil:
		return nil, errors.New("Sync replay target is required")
	case strings.TrimSpace(cfg.JobName) == "":
		return nil, errors.New("Sync replay job name is required")
	case strings.TrimSpace(cfg.OwnerID) == "":
		return nil, errors.New("Sync replay owner ID is required")
	case cfg.BatchSize <= 0:
		return nil, errors.New("Sync replay batch size must be positive")
	case cfg.BatchSize > MaxBatchSize:
		return nil, fmt.Errorf("Sync replay batch size cannot exceed %d", MaxBatchSize)
	case cfg.LeaseDuration < time.Second:
		return nil, errors.New("Sync replay lease duration must be at least one second")
	case int64(cfg.LeaseDuration/time.Second) > maxLeaseSeconds:
		return nil, errors.New("Sync replay lease duration exceeds MySQL interval range")
	}
	return &Runner{source: source, checkpoints: checkpoints, target: target, config: cfg}, nil
}

func (r *Runner) Run(ctx context.Context) (Result, error) {
	highWatermark, err := r.source.HighWatermark(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("read Sync replay high watermark: %w", err)
	}
	checkpoint, err := r.checkpoints.Acquire(ctx, r.config.JobName, r.config.OwnerID, highWatermark, r.config.LeaseDuration)
	if err != nil {
		return Result{}, fmt.Errorf("acquire Sync replay checkpoint: %w", err)
	}
	result := Result{HighWatermarkID: checkpoint.HighWatermarkID, LastProcessedID: checkpoint.LastProcessedID}
	if checkpoint.Status == StatusCompleted {
		return result, nil
	}
	for result.LastProcessedID < result.HighWatermarkID {
		items, listErr := r.source.ListAfter(ctx, result.LastProcessedID, result.HighWatermarkID, r.config.BatchSize)
		if listErr != nil {
			return result, r.recordFailure(ctx, fmt.Errorf("list Sync replay source: %w", listErr))
		}
		if len(items) == 0 {
			return result, r.recordFailure(ctx, fmt.Errorf("Sync replay source ended before high watermark %d", result.HighWatermarkID))
		}
		lastID := result.LastProcessedID
		var projected, skipped uint64
		for _, item := range items {
			if item.SourceID <= lastID || item.SourceID > result.HighWatermarkID {
				return result, r.recordFailure(ctx, fmt.Errorf("Sync replay source ID %d is outside (%d, %d]", item.SourceID, lastID, result.HighWatermarkID))
			}
			if item.Projection == nil {
				return result, r.recordFailure(ctx, fmt.Errorf("Sync replay source ID %d has no projection", item.SourceID))
			}
			if item.Fanout {
				if applyErr := r.target.Apply(item.Projection); applyErr != nil {
					return result, r.recordFailure(ctx, fmt.Errorf("apply Sync replay message %s: %w", item.Projection.MessageUUID, applyErr))
				}
				projected++
			} else {
				skipped++
			}
			lastID = item.SourceID
		}
		if err := r.checkpoints.Advance(ctx, r.config.JobName, r.config.OwnerID, lastID, r.config.LeaseDuration); err != nil {
			return result, r.recordFailure(ctx, fmt.Errorf("advance Sync replay checkpoint: %w", err))
		}
		result.LastProcessedID = lastID
		result.Processed += uint64(len(items))
		result.Projected += projected
		result.Skipped += skipped
	}
	if err := r.checkpoints.Complete(ctx, r.config.JobName, r.config.OwnerID); err != nil {
		return result, r.recordFailure(ctx, fmt.Errorf("complete Sync replay checkpoint: %w", err))
	}
	return result, nil
}

func (r *Runner) recordFailure(ctx context.Context, cause error) error {
	if err := r.checkpoints.Fail(ctx, r.config.JobName, r.config.OwnerID, cause); err != nil {
		return errors.Join(cause, fmt.Errorf("record Sync replay failure: %w", err))
	}
	return cause
}
