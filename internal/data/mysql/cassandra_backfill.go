package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	cassandrabackfill "github.com/JekYUlll/Dipole/internal/backfill/cassandra"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/data/mysql/mapper"
)

var (
	ErrCassandraBackfillLeaseHeld  = errors.New("Cassandra backfill lease is held by another owner")
	ErrCassandraBackfillLeaseLost  = errors.New("Cassandra backfill lease was lost")
	ErrCassandraBackfillIncomplete = errors.New("Cassandra backfill job is not complete")
)

type CassandraBackfillSource struct{ queries *generated.Queries }

func NewCassandraBackfillSource(store *Store) (*CassandraBackfillSource, error) {
	if store == nil {
		return nil, errors.New("Cassandra backfill MySQL store is required")
	}
	return &CassandraBackfillSource{queries: store.Queries()}, nil
}

func (s *CassandraBackfillSource) HighWatermark(ctx context.Context) (uint64, error) {
	highWatermark, err := s.queries.GetCassandraBackfillHighWatermark(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return highWatermark, err
}

func (s *CassandraBackfillSource) ListAfter(ctx context.Context, afterID, throughID uint64, limit int) ([]cassandrabackfill.SourceMessage, error) {
	rows, err := s.queries.ListMessagesForCassandraBackfill(ctx, generated.ListMessagesForCassandraBackfillParams{
		AfterID: afterID, ThroughID: throughID, Limit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	result := make([]cassandrabackfill.SourceMessage, 0, len(rows))
	for _, row := range rows {
		result = append(result, cassandrabackfill.SourceMessage{SourceID: row.ID, Message: *mapper.Message(row)})
	}
	return result, nil
}

type CassandraBackfillCheckpointStore struct {
	store *Store
}

func NewCassandraBackfillCheckpointStore(store *Store) (*CassandraBackfillCheckpointStore, error) {
	if store == nil {
		return nil, errors.New("Cassandra backfill checkpoint MySQL store is required")
	}
	return &CassandraBackfillCheckpointStore{store: store}, nil
}

func (s *CassandraBackfillCheckpointStore) Acquire(ctx context.Context, jobName, ownerID string, highWatermark uint64, lease time.Duration) (cassandrabackfill.Checkpoint, error) {
	var checkpoint cassandrabackfill.Checkpoint
	err := s.store.WithinTx(ctx, nil, func(q *generated.Queries) error {
		if err := q.EnsureCassandraBackfillJob(ctx, generated.EnsureCassandraBackfillJobParams{JobName: jobName, SourceHighWatermarkID: highWatermark}); err != nil {
			return fmt.Errorf("ensure Cassandra backfill job: %w", err)
		}
		job, err := q.LockCassandraBackfillJob(ctx, jobName)
		if err != nil {
			return fmt.Errorf("lock Cassandra backfill job: %w", err)
		}
		checkpoint = cassandrabackfill.Checkpoint{
			HighWatermarkID: job.SourceHighWatermarkID,
			LastProcessedID: job.LastProcessedID,
			Status:          job.Status,
		}
		if job.Status == cassandrabackfill.StatusCompleted {
			return nil
		}
		if job.OwnerID != "" && job.OwnerID != ownerID && job.LeaseExpiresAt.Valid && job.LeaseExpiresAt.Time.After(job.DatabaseNow) {
			return fmt.Errorf("%w: job=%s owner=%s", ErrCassandraBackfillLeaseHeld, jobName, job.OwnerID)
		}
		if err := q.ClaimCassandraBackfillJob(ctx, generated.ClaimCassandraBackfillJobParams{
			OwnerID: ownerID, LeaseSeconds: int32(lease / time.Second), JobName: jobName,
		}); err != nil {
			return fmt.Errorf("claim Cassandra backfill job: %w", err)
		}
		checkpoint.Status = cassandrabackfill.StatusRunning
		return nil
	})
	return checkpoint, err
}

func (s *CassandraBackfillCheckpointStore) Advance(ctx context.Context, jobName, ownerID string, sourceID uint64, lease time.Duration) error {
	result, err := s.store.Queries().AdvanceCassandraBackfillJob(ctx, generated.AdvanceCassandraBackfillJobParams{
		LastProcessedID: sourceID, LeaseSeconds: int32(lease / time.Second), JobName: jobName, OwnerID: ownerID,
	})
	return requireBackfillOwnership(result, err, "advance")
}

func (s *CassandraBackfillCheckpointStore) Fail(ctx context.Context, jobName, ownerID string, cause error) error {
	message := ""
	if cause != nil {
		message = cause.Error()
	}
	if len(message) > 4096 {
		message = message[:4096]
	}
	result, err := s.store.Queries().FailCassandraBackfillJob(ctx, generated.FailCassandraBackfillJobParams{
		LastError: message, JobName: jobName, OwnerID: ownerID,
	})
	return requireBackfillOwnership(result, err, "fail")
}

func (s *CassandraBackfillCheckpointStore) Complete(ctx context.Context, jobName, ownerID string) error {
	result, err := s.store.Queries().CompleteCassandraBackfillJob(ctx, generated.CompleteCassandraBackfillJobParams{JobName: jobName, OwnerID: ownerID})
	return requireBackfillOwnership(result, err, "complete")
}

func (s *CassandraBackfillCheckpointStore) CompletedHighWatermark(ctx context.Context, jobName string) (uint64, error) {
	job, err := s.store.Queries().GetCassandraBackfillJob(ctx, strings.TrimSpace(jobName))
	if err != nil {
		return 0, fmt.Errorf("read Cassandra backfill job: %w", err)
	}
	if job.Status != cassandrabackfill.StatusCompleted || job.LastProcessedID != job.SourceHighWatermarkID {
		return 0, fmt.Errorf("%w: job=%s status=%s checkpoint=%d high_watermark=%d",
			ErrCassandraBackfillIncomplete, job.JobName, job.Status, job.LastProcessedID, job.SourceHighWatermarkID)
	}
	return job.SourceHighWatermarkID, nil
}

func requireBackfillOwnership(result sql.Result, err error, operation string) error {
	if err != nil {
		return fmt.Errorf("%s Cassandra backfill job: %w", operation, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read %s Cassandra backfill result: %w", operation, err)
	}
	if rows != 1 {
		return fmt.Errorf("%w during %s", ErrCassandraBackfillLeaseLost, strings.TrimSpace(operation))
	}
	return nil
}
