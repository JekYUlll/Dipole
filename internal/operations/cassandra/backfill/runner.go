package cassandrabackfill

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
	cassandradata "github.com/JekYUlll/Dipole/internal/platform/cassandra"
)

const (
	StatusRunning            = "running"
	StatusFailed             = "failed"
	StatusCompleted          = "completed"
	MaxBatchSize             = 10_000
	maxLeaseSeconds          = int64(^uint32(0) >> 1)
	SourceKindMySQLMessages  = "mysql_messages"
	SourceKindMessageArchive = "message_archive"
)

type SourceDescriptor struct {
	Kind       string
	SnapshotID string
	SHA256     string
}

type SourceMessage struct {
	SourceID uint64
	Message  model.Message
}

type Source interface {
	HighWatermark(context.Context) (uint64, error)
	Descriptor(context.Context, uint64) (SourceDescriptor, error)
	ListAfter(context.Context, uint64, uint64, int) ([]SourceMessage, error)
}

type Checkpoint struct {
	HighWatermarkID uint64
	LastProcessedID uint64
	Status          string
}

type CheckpointStore interface {
	Acquire(context.Context, string, string, SourceDescriptor, uint64, time.Duration) (Checkpoint, error)
	Advance(context.Context, string, string, uint64, time.Duration) error
	Fail(context.Context, string, string, error) error
	Complete(context.Context, string, string) error
}

type TimelineAppender interface {
	Append(context.Context, cassandradata.TimelineProjection) (cassandradata.AppendResult, error)
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
	Inserted        uint64
	Duplicates      uint64
}

type Runner struct {
	source      Source
	checkpoints CheckpointStore
	timeline    TimelineAppender
	config      Config
}

func NewRunner(source Source, checkpoints CheckpointStore, timeline TimelineAppender, cfg Config) (*Runner, error) {
	switch {
	case source == nil:
		return nil, errors.New("Cassandra backfill source is required")
	case checkpoints == nil:
		return nil, errors.New("Cassandra backfill checkpoint store is required")
	case timeline == nil:
		return nil, errors.New("Cassandra backfill timeline appender is required")
	case strings.TrimSpace(cfg.JobName) == "":
		return nil, errors.New("Cassandra backfill job name is required")
	case strings.TrimSpace(cfg.OwnerID) == "":
		return nil, errors.New("Cassandra backfill owner ID is required")
	case cfg.BatchSize <= 0:
		return nil, errors.New("Cassandra backfill batch size must be positive")
	case cfg.BatchSize > MaxBatchSize:
		return nil, fmt.Errorf("Cassandra backfill batch size cannot exceed %d", MaxBatchSize)
	case cfg.LeaseDuration < time.Second:
		return nil, errors.New("Cassandra backfill lease duration must be at least one second")
	case int64(cfg.LeaseDuration/time.Second) > maxLeaseSeconds:
		return nil, errors.New("Cassandra backfill lease duration exceeds MySQL interval range")
	}
	return &Runner{source: source, checkpoints: checkpoints, timeline: timeline, config: cfg}, nil
}

func (r *Runner) Run(ctx context.Context) (Result, error) {
	highWatermark, err := r.source.HighWatermark(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("read Cassandra backfill high watermark: %w", err)
	}
	descriptor, err := r.source.Descriptor(ctx, highWatermark)
	if err != nil {
		return Result{}, fmt.Errorf("describe Cassandra backfill source: %w", err)
	}
	if strings.TrimSpace(descriptor.Kind) == "" || strings.TrimSpace(descriptor.SnapshotID) == "" {
		return Result{}, errors.New("Cassandra backfill source descriptor is incomplete")
	}
	checkpoint, err := r.checkpoints.Acquire(ctx, r.config.JobName, r.config.OwnerID, descriptor, highWatermark, r.config.LeaseDuration)
	if err != nil {
		return Result{}, fmt.Errorf("acquire Cassandra backfill checkpoint: %w", err)
	}
	result := Result{HighWatermarkID: checkpoint.HighWatermarkID, LastProcessedID: checkpoint.LastProcessedID}
	if checkpoint.Status == StatusCompleted {
		return result, nil
	}

	for result.LastProcessedID < result.HighWatermarkID {
		messages, listErr := r.source.ListAfter(ctx, result.LastProcessedID, result.HighWatermarkID, r.config.BatchSize)
		if listErr != nil {
			return result, r.recordFailure(ctx, fmt.Errorf("list Cassandra backfill source: %w", listErr))
		}
		if len(messages) == 0 {
			return result, r.recordFailure(ctx, fmt.Errorf("Cassandra backfill source ended before high watermark %d", result.HighWatermarkID))
		}

		lastID := result.LastProcessedID
		var batchInserted, batchDuplicates uint64
		for _, sourceMessage := range messages {
			if sourceMessage.SourceID <= lastID || sourceMessage.SourceID > result.HighWatermarkID {
				return result, r.recordFailure(ctx, fmt.Errorf("Cassandra backfill source ID %d is outside (%d, %d]", sourceMessage.SourceID, lastID, result.HighWatermarkID))
			}
			appendResult, appendErr := r.timeline.Append(ctx, ProjectionForMessage(sourceMessage.Message))
			if appendErr != nil {
				return result, r.recordFailure(ctx, fmt.Errorf("append Cassandra backfill message %s: %w", sourceMessage.Message.UUID, appendErr))
			}
			if appendResult.Inserted {
				batchInserted++
			}
			if appendResult.Duplicate {
				batchDuplicates++
			}
			lastID = sourceMessage.SourceID
		}
		if err := r.checkpoints.Advance(ctx, r.config.JobName, r.config.OwnerID, lastID, r.config.LeaseDuration); err != nil {
			return result, r.recordFailure(ctx, fmt.Errorf("advance Cassandra backfill checkpoint: %w", err))
		}
		result.LastProcessedID = lastID
		result.Processed += uint64(len(messages))
		result.Inserted += batchInserted
		result.Duplicates += batchDuplicates
	}

	if err := r.checkpoints.Complete(ctx, r.config.JobName, r.config.OwnerID); err != nil {
		return result, r.recordFailure(ctx, fmt.Errorf("complete Cassandra backfill checkpoint: %w", err))
	}
	return result, nil
}

func (r *Runner) recordFailure(ctx context.Context, cause error) error {
	if err := r.checkpoints.Fail(ctx, r.config.JobName, r.config.OwnerID, cause); err != nil {
		return errors.Join(cause, fmt.Errorf("record Cassandra backfill failure: %w", err))
	}
	return cause
}

func ProjectionForMessage(message model.Message) cassandradata.TimelineProjection {
	return cassandradata.TimelineProjection{
		EventID: "backfill:" + message.UUID, EventVersion: "v1",
		ConversationKey: message.ConversationKey, MessageSeq: message.Seq,
		MessageUUID: message.UUID, ClientMessageID: message.ClientMessageID,
		SenderUUID: message.SenderUUID, TargetType: message.TargetType, TargetUUID: message.TargetUUID,
		MessageType: message.MessageType, Content: message.Content,
		FileID: message.FileID, FileName: message.FileName, FileSize: message.FileSize,
		FileURL: message.FileURL, FileContentType: message.FileContentType,
		FileExpiresAt: message.FileExpiresAt, SentAt: message.SentAt,
	}
}
