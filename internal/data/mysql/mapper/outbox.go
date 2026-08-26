package mapper

import (
	"database/sql"

	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/model"
)

func OutboxCreateParams(event *model.OutboxEvent) generated.CreateOutboxEventParams {
	return generated.CreateOutboxEventParams{
		AggregateType: event.AggregateType, AggregateID: event.AggregateID,
		EventType: event.EventType, Topic: event.Topic, MessageKey: event.MessageKey,
		Value: event.Value, HeadersJson: sql.NullString{String: string(event.HeadersJSON), Valid: len(event.HeadersJSON) > 0},
		Status: event.Status, RetryCount: int64(event.RetryCount),
		LastError:   sql.NullString{String: event.LastError, Valid: event.LastError != ""},
		NextRetryAt: nullableTime(event.NextRetryAt), LockedAt: nullableTime(event.LockedAt),
		PublishedAt: nullableTime(event.PublishedAt),
	}
}

func OutboxEvent(row generated.OutboxEvent) *model.OutboxEvent {
	return &model.OutboxEvent{
		ID:            uint(row.ID),
		AggregateType: row.AggregateType,
		AggregateID:   row.AggregateID,
		EventType:     row.EventType,
		Topic:         row.Topic,
		MessageKey:    row.MessageKey,
		Value:         append([]byte(nil), row.Value...),
		HeadersJSON:   []byte(row.HeadersJson.String),
		Status:        row.Status,
		RetryCount:    int(row.RetryCount),
		LastError:     row.LastError.String,
		NextRetryAt:   nullableTimePointer(row.NextRetryAt),
		LockedAt:      nullableTimePointer(row.LockedAt),
		PublishedAt:   nullableTimePointer(row.PublishedAt),
		CreatedAt:     row.CreatedAt.Time,
		UpdatedAt:     row.UpdatedAt.Time,
	}
}

func OutboxEvents(rows []generated.OutboxEvent) []*model.OutboxEvent {
	events := make([]*model.OutboxEvent, 0, len(rows))
	for _, row := range rows {
		events = append(events, OutboxEvent(row))
	}
	return events
}
