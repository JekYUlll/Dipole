package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	memorylineage "github.com/JekYUlll/Dipole/internal/operations/agent/memorylineage"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
)

var (
	ErrMemoryLineageBackfillLeaseHeld      = errors.New("Memory lineage backfill lease is held by another owner")
	ErrMemoryLineageBackfillLeaseLost      = errors.New("Memory lineage backfill lease was lost")
	ErrMemoryLineageBackfillSourceMismatch = errors.New("Memory lineage backfill source does not match job")
)

type memoryLineageManifest struct {
	Selected []memoryLineageManifestItem `json:"selected"`
}

type memoryLineageManifestItem struct {
	ID             string `json:"id"`
	Representation string `json:"representation"`
}

func decodeMemoryLineageManifest(data []byte) (memoryLineageManifest, error) {
	var manifest memoryLineageManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return memoryLineageManifest{}, fmt.Errorf("decode Context manifest: %w", err)
	}
	return manifest, nil
}

type MemoryLineageBackfillSource struct{ queries *generated.Queries }

func NewMemoryLineageBackfillSource(store *Store) (*MemoryLineageBackfillSource, error) {
	if store == nil {
		return nil, errors.New("Memory lineage backfill MySQL store is required")
	}
	return &MemoryLineageBackfillSource{queries: store.Queries()}, nil
}

func (s *MemoryLineageBackfillSource) HighWatermark(ctx context.Context) (uint64, error) {
	highWatermark, err := s.queries.GetAgentMemoryLineageBackfillHighWatermark(ctx)
	if err != nil {
		return 0, err
	}
	if highWatermark < 0 {
		return 0, errors.New("Memory lineage backfill high watermark is negative")
	}
	return uint64(highWatermark), nil
}

func (s *MemoryLineageBackfillSource) ListAfter(ctx context.Context, afterID, throughID uint64, limit int) ([]memorylineage.SourceItem, error) {
	rows, err := s.queries.ListAgentMemoryLineageBackfill(ctx, generated.ListAgentMemoryLineageBackfillParams{
		AfterID: afterID, ThroughID: throughID, Limit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	items := make([]memorylineage.SourceItem, 0, len(rows))
	for _, row := range rows {
		manifest, err := decodeMemoryLineageManifest(row.ContextManifestJson)
		if err != nil {
			return nil, fmt.Errorf("decode Context manifest for plan %d: %w", row.PlanID, err)
		}
		references := make([]memorylineage.Reference, 0, len(manifest.Selected))
		for _, selected := range manifest.Selected {
			memoryUUID := ""
			if strings.HasPrefix(selected.ID, "memory:") {
				candidate := strings.TrimPrefix(selected.ID, "memory:")
				if candidate != "" {
					resolved, lookupErr := s.queries.GetAgentMemoryBackfillReference(ctx, generated.GetAgentMemoryBackfillReferenceParams{
						MemoryUuid: candidate, TenantID: row.TenantID, PrincipalUuid: row.PrincipalUuid,
					})
					if lookupErr == nil {
						memoryUUID = resolved
					} else if !errors.Is(lookupErr, sql.ErrNoRows) {
						return nil, fmt.Errorf("resolve Memory reference for plan %d: %w", row.PlanID, lookupErr)
					}
				}
			}
			references = append(references, memorylineage.Reference{
				MemoryUUID: memoryUUID, TaskUUID: row.TaskUuid, Representation: selected.Representation,
			})
		}
		items = append(items, memorylineage.SourceItem{SourceID: row.PlanID, References: references})
	}
	return items, nil
}

type MemoryLineageBackfillTarget struct{ queries *generated.Queries }

func NewMemoryLineageBackfillTarget(store *Store) (*MemoryLineageBackfillTarget, error) {
	if store == nil {
		return nil, errors.New("Memory lineage backfill target MySQL store is required")
	}
	return &MemoryLineageBackfillTarget{queries: store.Queries()}, nil
}

func (t *MemoryLineageBackfillTarget) Apply(ctx context.Context, reference memorylineage.Reference) (bool, bool, error) {
	result, err := t.queries.UpsertAgentMemoryLineageBackfill(ctx, generated.UpsertAgentMemoryLineageBackfillParams{
		MemoryUuid: reference.MemoryUUID, TaskUuid: reference.TaskUUID, Representation: reference.Representation,
	})
	if err != nil {
		return false, false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, false, fmt.Errorf("read Memory lineage backfill target result: %w", err)
	}
	return rows == 1, rows == 0, nil
}

type MemoryLineageBackfillCheckpointStore struct{ store *Store }

func NewMemoryLineageBackfillCheckpointStore(store *Store) (*MemoryLineageBackfillCheckpointStore, error) {
	if store == nil {
		return nil, errors.New("Memory lineage backfill checkpoint MySQL store is required")
	}
	return &MemoryLineageBackfillCheckpointStore{store: store}, nil
}

func (s *MemoryLineageBackfillCheckpointStore) Acquire(ctx context.Context, jobName, ownerID string, highWatermark uint64, lease time.Duration) (memorylineage.Checkpoint, error) {
	var checkpoint memorylineage.Checkpoint
	err := s.store.WithinTx(ctx, nil, func(q *generated.Queries) error {
		if err := q.EnsureAgentMemoryLineageBackfillJob(ctx, generated.EnsureAgentMemoryLineageBackfillJobParams{JobName: jobName, SourceHighWatermarkID: highWatermark}); err != nil {
			return fmt.Errorf("ensure Memory lineage backfill job: %w", err)
		}
		job, err := q.LockAgentMemoryLineageBackfillJob(ctx, jobName)
		if err != nil {
			return fmt.Errorf("lock Memory lineage backfill job: %w", err)
		}
		if job.SourceHighWatermarkID != highWatermark {
			return fmt.Errorf("%w: job=%s source=%d current=%d", ErrMemoryLineageBackfillSourceMismatch, jobName, job.SourceHighWatermarkID, highWatermark)
		}
		checkpoint = memorylineage.Checkpoint{HighWatermarkID: job.SourceHighWatermarkID, LastProcessedID: job.LastProcessedID, Status: job.Status}
		if job.Status == memorylineage.StatusCompleted {
			return nil
		}
		if job.OwnerID != "" && job.OwnerID != ownerID && job.LeaseExpiresAt.Valid && job.LeaseExpiresAt.Time.After(job.DatabaseNow) {
			return fmt.Errorf("%w: job=%s owner=%s", ErrMemoryLineageBackfillLeaseHeld, jobName, job.OwnerID)
		}
		if err := q.ClaimAgentMemoryLineageBackfillJob(ctx, generated.ClaimAgentMemoryLineageBackfillJobParams{OwnerID: ownerID, LeaseSeconds: int32(lease / time.Second), JobName: jobName}); err != nil {
			return fmt.Errorf("claim Memory lineage backfill job: %w", err)
		}
		checkpoint.Status = memorylineage.StatusRunning
		return nil
	})
	return checkpoint, err
}

func (s *MemoryLineageBackfillCheckpointStore) Advance(ctx context.Context, jobName, ownerID string, sourceID uint64, lease time.Duration) error {
	result, err := s.store.Queries().AdvanceAgentMemoryLineageBackfillJob(ctx, generated.AdvanceAgentMemoryLineageBackfillJobParams{LastProcessedID: sourceID, LeaseSeconds: int32(lease / time.Second), JobName: jobName, OwnerID: ownerID})
	return requireMemoryLineageBackfillOwnership(result, err, "advance")
}

func (s *MemoryLineageBackfillCheckpointStore) Fail(ctx context.Context, jobName, ownerID string, cause error) error {
	message := ""
	if cause != nil {
		message = cause.Error()
	}
	if len(message) > 4096 {
		message = message[:4096]
	}
	result, err := s.store.Queries().FailAgentMemoryLineageBackfillJob(ctx, generated.FailAgentMemoryLineageBackfillJobParams{LastError: message, JobName: jobName, OwnerID: ownerID})
	return requireMemoryLineageBackfillOwnership(result, err, "fail")
}

func (s *MemoryLineageBackfillCheckpointStore) Complete(ctx context.Context, jobName, ownerID string) error {
	result, err := s.store.Queries().CompleteAgentMemoryLineageBackfillJob(ctx, generated.CompleteAgentMemoryLineageBackfillJobParams{JobName: jobName, OwnerID: ownerID})
	return requireMemoryLineageBackfillOwnership(result, err, "complete")
}

func requireMemoryLineageBackfillOwnership(result sql.Result, err error, operation string) error {
	if err != nil {
		return fmt.Errorf("%s Memory lineage backfill job: %w", operation, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read %s Memory lineage backfill result: %w", operation, err)
	}
	if rows != 1 {
		return fmt.Errorf("%w during %s", ErrMemoryLineageBackfillLeaseLost, strings.TrimSpace(operation))
	}
	return nil
}
