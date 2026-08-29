package agentmysql

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
)

var _ application.AgentTaskTimelineStoreV1 = (*AgentPolicyRepository)(nil)

func (r *AgentPolicyRepository) AppendAgentTaskTimelineEvent(ctx context.Context, event application.AgentTaskTimelineEventV1) (uint64, error) {
	if err := event.Validate(); err != nil {
		return 0, fmt.Errorf("validate Agent Task Timeline event: %w", err)
	}
	seq, err := appendAgentTaskTimelineEvent(ctx, r.queries, event)
	if err == nil {
		return seq, nil
	}
	// Keep a durable intent when the projection database rejects the write.
	// The caller still observes the original failure; a repair worker can replay it later.
	repairCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if repairErr := r.EnqueueAgentTaskTimelineRepair(repairCtx, event, err); repairErr != nil {
		return 0, fmt.Errorf("append Agent Task Timeline event: %w (enqueue repair: %v)", err, repairErr)
	}
	return 0, err
}

func appendAgentTaskTimelineEvent(ctx context.Context, queries generated.Querier, event application.AgentTaskTimelineEventV1) (uint64, error) {
	result, err := queries.InsertAgentTaskTimelineEvent(ctx, generated.InsertAgentTaskTimelineEventParams{
		EventUuid: event.EventUUID, TaskUuid: event.TaskUUID, RunUuid: nullableString(event.RunUUID),
		EventKind: string(event.Kind), Status: event.Status, CapabilityID: nullableString(event.CapabilityID),
		ApprovalUuid: nullableString(event.ApprovalUUID), OccurredAt: event.OccurredAt,
	})
	if err != nil {
		return 0, fmt.Errorf("append Agent Task Timeline event: %w", err)
	}
	seq, err := result.LastInsertId()
	if err == nil && seq > 0 {
		return uint64(seq), nil
	}
	row, lookupErr := queries.GetAgentTaskTimelineEventByUUID(ctx, event.EventUUID)
	if lookupErr != nil {
		return 0, fmt.Errorf("read Agent Task Timeline event sequence: %w", errors.Join(err, lookupErr))
	}
	return row.EventSeq, nil
}

var _ application.AgentTaskTimelineRepairStoreV1 = (*AgentPolicyRepository)(nil)

func (r *AgentPolicyRepository) EnqueueAgentTaskTimelineRepair(ctx context.Context, event application.AgentTaskTimelineEventV1, cause error) error {
	if err := event.Validate(); err != nil {
		return fmt.Errorf("validate Agent Task Timeline repair: %w", err)
	}
	lastError := "timeline append failed"
	if cause != nil {
		lastError = cause.Error()
	}
	if len(lastError) > 512 {
		lastError = lastError[:512]
	}
	_, err := r.queries.EnqueueAgentTaskTimelineRepair(ctx, generated.EnqueueAgentTaskTimelineRepairParams{
		EventUuid: event.EventUUID, TaskUuid: event.TaskUUID, RunUuid: nullableString(event.RunUUID),
		EventKind: string(event.Kind), Status: event.Status, CapabilityID: nullableString(event.CapabilityID),
		ApprovalUuid: nullableString(event.ApprovalUUID), OccurredAt: event.OccurredAt, LastError: sql.NullString{String: lastError, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("enqueue Agent Task Timeline repair: %w", err)
	}
	return nil
}

func (r *AgentPolicyRepository) ClaimAgentTaskTimelineRepairs(limit int, now time.Time, lease time.Duration) ([]application.AgentTaskTimelineRepairV1, error) {
	if limit <= 0 {
		return []application.AgentTaskTimelineRepairV1{}, nil
	}
	now = now.UTC()
	lockedBefore := now.Add(-lease)
	ctx := context.Background()
	var result []application.AgentTaskTimelineRepairV1
	err := r.withTransaction(ctx, func(q generated.Querier) error {
		rows, err := q.SelectClaimableAgentTaskTimelineRepairs(ctx, generated.SelectClaimableAgentTaskTimelineRepairsParams{
			LockedAt: nullableTime(&lockedBefore), Limit: int32(limit),
		})
		if err != nil {
			return fmt.Errorf("claim Agent Task Timeline repairs: %w", err)
		}
		if len(rows) == 0 {
			return nil
		}
		uuids := make([]string, 0, len(rows))
		for _, row := range rows {
			uuids = append(uuids, row.EventUuid)
		}
		if _, err := q.MarkAgentTaskTimelineRepairsProcessing(ctx, generated.MarkAgentTaskTimelineRepairsProcessingParams{LockedAt: nullableTime(&now), EventUuids: uuids}); err != nil {
			return fmt.Errorf("mark Agent Task Timeline repairs processing: %w", err)
		}
		for _, row := range rows {
			result = append(result, application.AgentTaskTimelineRepairV1{EventUUID: row.EventUuid, TaskUUID: row.TaskUuid, RunUUID: row.RunUuid.String, Kind: application.AgentTaskTimelineEventKindV1(row.EventKind), Status: row.Status, CapabilityID: row.CapabilityID.String, ApprovalUUID: row.ApprovalUuid.String, OccurredAt: row.OccurredAt, RepairStatus: "processing", RetryCount: uint32(row.RetryCount), LastError: row.LastError.String, NextRetryAt: timePointer(row.NextRetryAt), LockedAt: timePointer(row.LockedAt)})
		}
		return nil
	})
	return result, err
}

func (r *AgentPolicyRepository) MarkAgentTaskTimelineRepairCompleted(eventUUID string) error {
	_, err := r.queries.MarkAgentTaskTimelineRepairCompleted(context.Background(), eventUUID)
	return err
}

func (r *AgentPolicyRepository) MarkAgentTaskTimelineRepairRetry(eventUUID string, retryCount uint32, next time.Time, cause error) error {
	lastError := "timeline repair failed"
	if cause != nil {
		lastError = cause.Error()
	}
	if len(lastError) > 512 {
		lastError = lastError[:512]
	}
	_, err := r.queries.MarkAgentTaskTimelineRepairRetry(context.Background(), generated.MarkAgentTaskTimelineRepairRetryParams{RetryCount: retryCount, NextRetryAt: nullableTime(&next), LastError: sql.NullString{String: lastError, Valid: true}, EventUuid: eventUUID})
	return err
}

func (r *AgentPolicyRepository) withTransaction(ctx context.Context, fn func(generated.Querier) error) error {
	if r.store == nil {
		return fn(r.queries)
	}
	return r.store.WithinTx(ctx, nil, func(q *generated.Queries) error { return fn(q) })
}

func timelineEvent(taskUUID, runUUID string, kind application.AgentTaskTimelineEventKindV1, status string) application.AgentTaskTimelineEventV1 {
	occurredAt := time.Now().UTC()
	identity := fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%d", taskUUID, runUUID, kind, status, occurredAt.UnixNano())
	digest := sha256.Sum256([]byte(identity))
	return application.AgentTaskTimelineEventV1{
		EventUUID: hex.EncodeToString(digest[:]),
		TaskUUID:  taskUUID, RunUUID: runUUID, Kind: kind, Status: status, OccurredAt: occurredAt,
	}
}

func (r *AgentPolicyRepository) ListAgentTaskTimelineEvents(ctx context.Context, taskUUID string, afterSeq uint64, limit int) ([]application.AgentTaskTimelineEventV1, error) {
	if strings.TrimSpace(taskUUID) == "" || limit < 1 || limit > 100 {
		return nil, fmt.Errorf("%w: invalid Agent Task Timeline page", application.ErrAgentPolicyInvalid)
	}
	rows, err := r.queries.ListAgentTaskTimelineEvents(ctx, generated.ListAgentTaskTimelineEventsParams{
		TaskUuid: strings.TrimSpace(taskUUID), EventSeq: afterSeq, Limit: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list Agent Task Timeline events: %w", err)
	}
	result := make([]application.AgentTaskTimelineEventV1, 0, len(rows))
	for _, row := range rows {
		result = append(result, application.AgentTaskTimelineEventV1{
			EventSeq: uint64(row.EventSeq), EventUUID: row.EventUuid, TaskUUID: row.TaskUuid,
			RunUUID: row.RunUuid.String, Kind: application.AgentTaskTimelineEventKindV1(row.EventKind),
			Status: row.Status, CapabilityID: row.CapabilityID.String, ApprovalUUID: row.ApprovalUuid.String,
			OccurredAt: row.OccurredAt,
		})
	}
	return result, nil
}
