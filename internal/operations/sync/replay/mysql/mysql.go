package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/compat/service"
	"github.com/JekYUlll/Dipole/internal/model"
	syncbackfill "github.com/JekYUlll/Dipole/internal/operations/sync/backfill"
	platformkafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	platformmysql "github.com/JekYUlll/Dipole/internal/platform/mysql"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
)

var (
	ErrSyncReplayLeaseHeld  = errors.New("Sync replay lease is held by another owner")
	ErrSyncReplayLeaseLost  = errors.New("Sync replay lease was lost")
	ErrSyncReplayIncomplete = errors.New("Sync replay job is not complete")
)

type SyncReplaySource struct{ queries *generated.Queries }

func NewSyncReplaySource(store *platformmysql.Store) (*SyncReplaySource, error) {
	if store == nil {
		return nil, errors.New("Sync replay MySQL store is required")
	}
	return &SyncReplaySource{queries: store.Queries()}, nil
}

func (s *SyncReplaySource) HighWatermark(ctx context.Context) (uint64, error) {
	highWatermark, err := s.queries.GetSyncReplayHighWatermark(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return highWatermark, err
}

func (s *SyncReplaySource) ListAfter(ctx context.Context, afterID, throughID uint64, limit int) ([]syncbackfill.SourceItem, error) {
	rows, err := s.queries.ListSyncReplayEvents(ctx, generated.ListSyncReplayEventsParams{
		AfterID: afterID, ThroughID: throughID, Limit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	result := make([]syncbackfill.SourceItem, 0, len(rows))
	for _, row := range rows {
		envelope, err := platformkafka.DecodeEnvelope(row.Value)
		if err != nil {
			return nil, fmt.Errorf("decode Sync replay outbox event %d: %w", row.ID, err)
		}
		if envelope.EventType != row.EventType {
			return nil, fmt.Errorf("Sync replay outbox event %d type mismatch: row=%s envelope=%s", row.ID, row.EventType, envelope.EventType)
		}
		payload, err := service.DecodeMessageEventPayload(envelope.EventType, envelope.Payload)
		if err != nil {
			return nil, fmt.Errorf("decode Sync replay payload %d: %w", row.ID, err)
		}
		projection, fanout, err := service.MessageSyncProjection(envelope.EventID, envelope.EventType, payload)
		if err != nil {
			return nil, fmt.Errorf("map Sync replay event %d: %w", row.ID, err)
		}
		if strings.TrimSpace(row.MessageKey) != projection.MessageUUID {
			return nil, fmt.Errorf("Sync replay outbox event %d key mismatch: row=%s payload=%s", row.ID, row.MessageKey, projection.MessageUUID)
		}
		result = append(result, syncbackfill.SourceItem{SourceID: row.ID, Fanout: fanout, Projection: projection})
	}
	return result, nil
}

type SyncReplayCheckpointStore struct{ store *platformmysql.Store }

func NewSyncReplayCheckpointStore(store *platformmysql.Store) (*SyncReplayCheckpointStore, error) {
	if store == nil {
		return nil, errors.New("Sync replay checkpoint MySQL store is required")
	}
	return &SyncReplayCheckpointStore{store: store}, nil
}

func (s *SyncReplayCheckpointStore) Acquire(ctx context.Context, jobName, ownerID string, highWatermark uint64, lease time.Duration) (syncbackfill.Checkpoint, error) {
	var checkpoint syncbackfill.Checkpoint
	err := s.store.WithinTx(ctx, nil, func(q *generated.Queries) error {
		if err := q.EnsureSyncReplayJob(ctx, generated.EnsureSyncReplayJobParams{JobName: jobName, SourceHighWatermarkID: highWatermark}); err != nil {
			return fmt.Errorf("ensure Sync replay job: %w", err)
		}
		job, err := q.LockSyncReplayJob(ctx, jobName)
		if err != nil {
			return fmt.Errorf("lock Sync replay job: %w", err)
		}
		checkpoint = syncbackfill.Checkpoint{
			HighWatermarkID: job.SourceHighWatermarkID, LastProcessedID: job.LastProcessedID, Status: job.Status,
		}
		if job.Status == syncbackfill.StatusCompleted {
			return nil
		}
		if job.OwnerID != "" && job.OwnerID != ownerID && job.LeaseExpiresAt.Valid && job.LeaseExpiresAt.Time.After(job.DatabaseNow) {
			return fmt.Errorf("%w: job=%s owner=%s", ErrSyncReplayLeaseHeld, jobName, job.OwnerID)
		}
		if err := q.ClaimSyncReplayJob(ctx, generated.ClaimSyncReplayJobParams{
			OwnerID: ownerID, LeaseSeconds: int32(lease / time.Second), JobName: jobName,
		}); err != nil {
			return fmt.Errorf("claim Sync replay job: %w", err)
		}
		checkpoint.Status = syncbackfill.StatusRunning
		return nil
	})
	return checkpoint, err
}

func (s *SyncReplayCheckpointStore) Advance(ctx context.Context, jobName, ownerID string, sourceID uint64, lease time.Duration) error {
	result, err := s.store.Queries().AdvanceSyncReplayJob(ctx, generated.AdvanceSyncReplayJobParams{
		LastProcessedID: sourceID, LeaseSeconds: int32(lease / time.Second), JobName: jobName, OwnerID: ownerID,
	})
	return requireSyncReplayOwnership(result, err, "advance")
}

func (s *SyncReplayCheckpointStore) Fail(ctx context.Context, jobName, ownerID string, cause error) error {
	message := ""
	if cause != nil {
		message = cause.Error()
	}
	if len(message) > 4096 {
		message = message[:4096]
	}
	result, err := s.store.Queries().FailSyncReplayJob(ctx, generated.FailSyncReplayJobParams{
		LastError: message, JobName: jobName, OwnerID: ownerID,
	})
	return requireSyncReplayOwnership(result, err, "fail")
}

func (s *SyncReplayCheckpointStore) Complete(ctx context.Context, jobName, ownerID string) error {
	result, err := s.store.Queries().CompleteSyncReplayJob(ctx, generated.CompleteSyncReplayJobParams{JobName: jobName, OwnerID: ownerID})
	return requireSyncReplayOwnership(result, err, "complete")
}

func (s *SyncReplayCheckpointStore) CompletedHighWatermark(ctx context.Context, jobName string) (uint64, error) {
	job, err := s.store.Queries().GetSyncReplayJob(ctx, strings.TrimSpace(jobName))
	if err != nil {
		return 0, fmt.Errorf("read Sync replay job: %w", err)
	}
	if job.Status != syncbackfill.StatusCompleted || job.LastProcessedID != job.SourceHighWatermarkID {
		return 0, fmt.Errorf("%w: job=%s status=%s checkpoint=%d high_watermark=%d",
			ErrSyncReplayIncomplete, job.JobName, job.Status, job.LastProcessedID, job.SourceHighWatermarkID)
	}
	return job.SourceHighWatermarkID, nil
}

func requireSyncReplayOwnership(result sql.Result, err error, operation string) error {
	if err != nil {
		return fmt.Errorf("%s Sync replay job: %w", operation, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read %s Sync replay result: %w", operation, err)
	}
	if rows != 1 {
		return fmt.Errorf("%w during %s", ErrSyncReplayLeaseLost, strings.TrimSpace(operation))
	}
	return nil
}

type SyncInboxReconcileTarget struct{ queries *generated.Queries }

func NewSyncInboxReconcileTarget(store *platformmysql.Store) (*SyncInboxReconcileTarget, error) {
	if store == nil {
		return nil, errors.New("Sync reconciliation MySQL store is required")
	}
	return &SyncInboxReconcileTarget{queries: store.Queries()}, nil
}

func (t *SyncInboxReconcileTarget) ListByMessageUUID(ctx context.Context, messageUUID string) ([]model.SyncInboxLocator, error) {
	rows, err := t.queries.ListSyncInboxLocatorsByMessageUUID(ctx, strings.TrimSpace(messageUUID))
	if err != nil {
		return nil, err
	}
	result := make([]model.SyncInboxLocator, 0, len(rows))
	for _, row := range rows {
		result = append(result, model.SyncInboxLocator{
			UserUUID: row.UserUuid, MessageUUID: row.MessageUuid,
			ConversationKey: row.ConversationKey, MessageSeq: row.MessageSeq,
		})
	}
	return result, nil
}
