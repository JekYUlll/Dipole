package messagemysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	mysqlData "github.com/JekYUlll/Dipole/internal/data/mysql"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/data/mysql/mapper"
	"github.com/JekYUlll/Dipole/internal/model"
)

var _ application.OutboxRelayStore = (*OutboxRepository)(nil)

type OutboxRepository struct {
	store mysqlData.TransactionStore
}

func NewOutboxRepository(store mysqlData.TransactionStore) (*OutboxRepository, error) {
	if store == nil {
		return nil, errors.New("outbox transaction store is required")
	}
	return &OutboxRepository{store: store}, nil
}

func (r *OutboxRepository) ClaimPendingBatch(limit int, now time.Time, lease time.Duration) ([]*model.OutboxEvent, error) {
	if limit <= 0 {
		return []*model.OutboxEvent{}, nil
	}
	now = now.UTC()
	ctx := context.Background()
	events := []*model.OutboxEvent{}
	err := r.store.WithinTx(ctx, nil, func(queries *generated.Queries) error {
		rows, err := queries.SelectClaimableOutboxEvents(ctx, generated.SelectClaimableOutboxEventsParams{
			PendingStatus:    model.OutboxStatusPending,
			Now:              outboxTime(now),
			ProcessingStatus: model.OutboxStatusProcessing,
			ClaimBefore:      outboxTime(now.Add(-lease)),
			Limit:            int32(limit),
		})
		if err != nil {
			return fmt.Errorf("select pending outbox events with sqlc: %w", err)
		}
		events = mapper.OutboxEvents(rows)
		if len(events) == 0 {
			return nil
		}
		ids := make([]uint64, 0, len(events))
		for _, event := range events {
			ids = append(ids, uint64(event.ID))
		}
		if _, err := queries.MarkOutboxEventsProcessing(ctx, generated.MarkOutboxEventsProcessingParams{
			Status:   model.OutboxStatusProcessing,
			LockedAt: outboxTime(now),
			Ids:      ids,
		}); err != nil {
			return fmt.Errorf("mark outbox events processing with sqlc: %w", err)
		}
		for _, event := range events {
			event.Status = model.OutboxStatusProcessing
			lockedAt := now
			event.LockedAt = &lockedAt
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (r *OutboxRepository) MarkPublished(id uint, publishedAt time.Time) error {
	_, err := r.store.Queries().MarkOutboxPublished(context.Background(), generated.MarkOutboxPublishedParams{
		Status:      model.OutboxStatusPublished,
		PublishedAt: outboxTime(publishedAt.UTC()),
		ID:          uint64(id),
	})
	return err
}

func (r *OutboxRepository) MarkRetry(id uint, retryCount int, nextRetryAt time.Time, lastErr error) error {
	lastError := ""
	if lastErr != nil {
		lastError = lastErr.Error()
		if len(lastError) > 512 {
			lastError = lastError[:512]
		}
	}
	_, err := r.store.Queries().MarkOutboxRetry(context.Background(), generated.MarkOutboxRetryParams{
		Status:      model.OutboxStatusPending,
		RetryCount:  int64(retryCount),
		NextRetryAt: outboxTime(nextRetryAt.UTC()),
		LastError:   sql.NullString{String: lastError, Valid: true},
		ID:          uint64(id),
	})
	return err
}

func (r *OutboxRepository) DecodeHeaders(event *model.OutboxEvent) (map[string]string, error) {
	if event == nil || len(event.HeadersJSON) == 0 {
		return nil, nil
	}
	var headers map[string]string
	if err := json.Unmarshal(event.HeadersJSON, &headers); err != nil {
		return nil, fmt.Errorf("decode outbox headers: %w", err)
	}
	return headers, nil
}

func outboxTime(value time.Time) sql.NullTime {
	return sql.NullTime{Time: value, Valid: true}
}
