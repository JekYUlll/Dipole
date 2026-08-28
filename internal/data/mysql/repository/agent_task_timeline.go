package repository

import (
	"context"
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
	return appendAgentTaskTimelineEvent(ctx, r.queries, event)
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
	if err != nil || seq <= 0 {
		return 0, fmt.Errorf("read Agent Task Timeline event sequence: %w", err)
	}
	return uint64(seq), nil
}

func timelineEvent(taskUUID, runUUID string, kind application.AgentTaskTimelineEventKindV1, status string) application.AgentTaskTimelineEventV1 {
	return application.AgentTaskTimelineEventV1{
		EventUUID: fmt.Sprintf("timeline-%s-%d", taskUUID, time.Now().UTC().UnixNano()),
		TaskUUID:  taskUUID, RunUUID: runUUID, Kind: kind, Status: status, OccurredAt: time.Now().UTC(),
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
