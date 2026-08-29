package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/compat/service"
	searchbackfill "github.com/JekYUlll/Dipole/internal/operations/search/backfill"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
)

var (
	ErrSearchBackfillLeaseHeld      = errors.New("Search backfill lease is held by another owner")
	ErrSearchBackfillLeaseLost      = errors.New("Search backfill lease was lost")
	ErrSearchBackfillIncomplete     = errors.New("Search backfill job is not complete")
	ErrSearchBackfillTargetMismatch = errors.New("Search backfill target index does not match job")
	ErrSearchBackfillSourceMismatch = errors.New("Search backfill source does not match job")
)

type SearchBackfillSource struct{ queries *generated.Queries }

func NewSearchBackfillSource(store *Store) (*SearchBackfillSource, error) {
	if store == nil {
		return nil, errors.New("Search backfill MySQL store is required")
	}
	return &SearchBackfillSource{queries: store.Queries()}, nil
}

func (s *SearchBackfillSource) HighWatermark(ctx context.Context) (uint64, error) {
	highWatermark, err := s.queries.GetSearchBackfillHighWatermark(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return highWatermark, err
}

func (s *SearchBackfillSource) Descriptor(_ context.Context, highWatermark uint64) (searchbackfill.SourceDescriptor, error) {
	return searchbackfill.SourceDescriptor{
		Kind:       searchbackfill.SourceKindMySQLOutbox,
		SnapshotID: fmt.Sprintf("mysql-outbox:%d", highWatermark),
	}, nil
}

func (s *SearchBackfillSource) ListAfter(ctx context.Context, afterID, throughID uint64, limit int) ([]searchbackfill.SourceMutation, error) {
	rows, err := s.queries.ListLatestSearchMutationsForBackfill(ctx, generated.ListLatestSearchMutationsForBackfillParams{
		AfterID: afterID, ThroughID: throughID, Limit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	result := make([]searchbackfill.SourceMutation, 0, len(rows))
	for _, row := range rows {
		envelope, err := platformKafka.DecodeEnvelope(row.Value)
		if err != nil {
			return nil, fmt.Errorf("decode Search backfill outbox event %d: %w", row.ID, err)
		}
		if envelope.EventType != row.EventType {
			return nil, fmt.Errorf("Search backfill outbox event %d type mismatch: row=%s envelope=%s", row.ID, row.EventType, envelope.EventType)
		}
		payload, err := service.DecodeMessageEventPayload(envelope.EventType, envelope.Payload)
		if err != nil {
			return nil, fmt.Errorf("decode Search backfill payload %d: %w", row.ID, err)
		}
		mutation, err := service.MessageSearchMutation(envelope.EventType, payload)
		if err != nil {
			return nil, fmt.Errorf("map Search backfill mutation %d: %w", row.ID, err)
		}
		if strings.TrimSpace(row.MessageKey) != mutation.MessageUUID {
			return nil, fmt.Errorf("Search backfill outbox event %d key mismatch: row=%s payload=%s", row.ID, row.MessageKey, mutation.MessageUUID)
		}
		result = append(result, searchbackfill.SourceMutation{SourceID: row.ID, Mutation: mutation})
	}
	return result, nil
}

type SearchBackfillCheckpointStore struct {
	store       *Store
	targetIndex string
}

func NewSearchBackfillCheckpointStore(store *Store, targetIndex string) (*SearchBackfillCheckpointStore, error) {
	if store == nil {
		return nil, errors.New("Search backfill checkpoint MySQL store is required")
	}
	targetIndex = strings.TrimSpace(targetIndex)
	if targetIndex == "" {
		return nil, errors.New("Search backfill target index is required")
	}
	return &SearchBackfillCheckpointStore{store: store, targetIndex: targetIndex}, nil
}

func (s *SearchBackfillCheckpointStore) Acquire(ctx context.Context, jobName, ownerID string, source searchbackfill.SourceDescriptor, highWatermark uint64, lease time.Duration) (searchbackfill.Checkpoint, error) {
	var checkpoint searchbackfill.Checkpoint
	err := s.store.WithinTx(ctx, nil, func(q *generated.Queries) error {
		if err := q.EnsureSearchBackfillJob(ctx, generated.EnsureSearchBackfillJobParams{
			JobName: jobName, TargetIndex: s.targetIndex,
			SourceKind: strings.TrimSpace(source.Kind), SourceSnapshotID: strings.TrimSpace(source.SnapshotID),
			SourceSha256: strings.ToLower(strings.TrimSpace(source.SHA256)), SourceHighWatermarkID: highWatermark,
		}); err != nil {
			return fmt.Errorf("ensure Search backfill job: %w", err)
		}
		job, err := q.LockSearchBackfillJob(ctx, jobName)
		if err != nil {
			return fmt.Errorf("lock Search backfill job: %w", err)
		}
		if job.TargetIndex != s.targetIndex {
			return fmt.Errorf("%w: job=%s expected=%s actual=%s", ErrSearchBackfillTargetMismatch, jobName, s.targetIndex, job.TargetIndex)
		}
		if job.SourceKind != strings.TrimSpace(source.Kind) ||
			job.SourceSnapshotID != strings.TrimSpace(source.SnapshotID) ||
			job.SourceSha256 != strings.ToLower(strings.TrimSpace(source.SHA256)) {
			return fmt.Errorf("%w: job=%s", ErrSearchBackfillSourceMismatch, jobName)
		}
		checkpoint = searchbackfill.Checkpoint{HighWatermarkID: job.SourceHighWatermarkID, LastProcessedID: job.LastProcessedID, Status: job.Status}
		if job.Status == searchbackfill.StatusCompleted {
			return nil
		}
		if job.OwnerID != "" && job.OwnerID != ownerID && job.LeaseExpiresAt.Valid && job.LeaseExpiresAt.Time.After(job.DatabaseNow) {
			return fmt.Errorf("%w: job=%s owner=%s", ErrSearchBackfillLeaseHeld, jobName, job.OwnerID)
		}
		if err := q.ClaimSearchBackfillJob(ctx, generated.ClaimSearchBackfillJobParams{
			OwnerID: ownerID, LeaseSeconds: int32(lease / time.Second), JobName: jobName,
		}); err != nil {
			return fmt.Errorf("claim Search backfill job: %w", err)
		}
		checkpoint.Status = searchbackfill.StatusRunning
		return nil
	})
	return checkpoint, err
}

func (s *SearchBackfillCheckpointStore) Advance(ctx context.Context, jobName, ownerID string, sourceID uint64, lease time.Duration) error {
	result, err := s.store.Queries().AdvanceSearchBackfillJob(ctx, generated.AdvanceSearchBackfillJobParams{
		LastProcessedID: sourceID, LeaseSeconds: int32(lease / time.Second), JobName: jobName, OwnerID: ownerID,
	})
	return requireSearchBackfillOwnership(result, err, "advance")
}

func (s *SearchBackfillCheckpointStore) Fail(ctx context.Context, jobName, ownerID string, cause error) error {
	message := ""
	if cause != nil {
		message = cause.Error()
	}
	if len(message) > 4096 {
		message = message[:4096]
	}
	result, err := s.store.Queries().FailSearchBackfillJob(ctx, generated.FailSearchBackfillJobParams{
		LastError: message, JobName: jobName, OwnerID: ownerID,
	})
	return requireSearchBackfillOwnership(result, err, "fail")
}

func (s *SearchBackfillCheckpointStore) Complete(ctx context.Context, jobName, ownerID string) error {
	result, err := s.store.Queries().CompleteSearchBackfillJob(ctx, generated.CompleteSearchBackfillJobParams{JobName: jobName, OwnerID: ownerID})
	return requireSearchBackfillOwnership(result, err, "complete")
}

func (s *SearchBackfillCheckpointStore) CompletedHighWatermark(ctx context.Context, jobName string) (uint64, error) {
	return s.completedHighWatermark(ctx, jobName, nil)
}

func (s *SearchBackfillCheckpointStore) CompletedHighWatermarkForSource(ctx context.Context, jobName string, source searchbackfill.SourceDescriptor) (uint64, error) {
	return s.completedHighWatermark(ctx, jobName, &source)
}

func (s *SearchBackfillCheckpointStore) completedHighWatermark(ctx context.Context, jobName string, source *searchbackfill.SourceDescriptor) (uint64, error) {
	job, err := s.store.Queries().GetSearchBackfillJob(ctx, strings.TrimSpace(jobName))
	if err != nil {
		return 0, fmt.Errorf("read Search backfill job: %w", err)
	}
	if job.TargetIndex != s.targetIndex {
		return 0, fmt.Errorf("%w: job=%s expected=%s actual=%s", ErrSearchBackfillTargetMismatch, jobName, s.targetIndex, job.TargetIndex)
	}
	if source != nil && (job.SourceKind != strings.TrimSpace(source.Kind) ||
		job.SourceSnapshotID != strings.TrimSpace(source.SnapshotID) ||
		job.SourceSha256 != strings.ToLower(strings.TrimSpace(source.SHA256))) {
		return 0, fmt.Errorf("%w: job=%s", ErrSearchBackfillSourceMismatch, jobName)
	}
	if job.Status != searchbackfill.StatusCompleted || job.LastProcessedID != job.SourceHighWatermarkID {
		return 0, fmt.Errorf("%w: job=%s status=%s checkpoint=%d high_watermark=%d", ErrSearchBackfillIncomplete, job.JobName, job.Status, job.LastProcessedID, job.SourceHighWatermarkID)
	}
	return job.SourceHighWatermarkID, nil
}

func requireSearchBackfillOwnership(result sql.Result, err error, operation string) error {
	if err != nil {
		return fmt.Errorf("%s Search backfill job: %w", operation, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read %s Search backfill result: %w", operation, err)
	}
	if rows != 1 {
		return fmt.Errorf("%w during %s", ErrSearchBackfillLeaseLost, strings.TrimSpace(operation))
	}
	return nil
}
