package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/store"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OutboxRepository struct{}

func NewOutboxRepository() *OutboxRepository {
	return &OutboxRepository{}
}

func (r *OutboxRepository) Enqueue(tx *gorm.DB, event *model.OutboxEvent) error {
	if event == nil {
		return nil
	}

	db := tx
	if db == nil {
		db = store.DB
	}
	if db == nil {
		return fmt.Errorf("enqueue outbox event: mysql not initialized")
	}

	if event.Status == "" {
		event.Status = model.OutboxStatusPending
	}

	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(event).Error; err != nil {
		return fmt.Errorf("enqueue outbox event: %w", err)
	}

	return nil
}

func (r *OutboxRepository) ClaimPendingBatch(limit int, now time.Time, lease time.Duration) ([]*model.OutboxEvent, error) {
	if store.DB == nil {
		return nil, fmt.Errorf("claim outbox batch: mysql not initialized")
	}
	if limit <= 0 {
		return []*model.OutboxEvent{}, nil
	}

	var events []*model.OutboxEvent
	claimBefore := now.UTC().Add(-lease)

	err := store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where(
				"("+
					"(status = ? AND (next_retry_at IS NULL OR next_retry_at <= ?))"+
					" OR "+
					"(status = ? AND locked_at IS NOT NULL AND locked_at <= ?)"+
					")",
				model.OutboxStatusPending,
				now.UTC(),
				model.OutboxStatusProcessing,
				claimBefore,
			).
			Order("id ASC").
			Limit(limit).
			Find(&events).Error; err != nil {
			return fmt.Errorf("select pending outbox events: %w", err)
		}
		if len(events) == 0 {
			return nil
		}

		ids := make([]uint, 0, len(events))
		for _, event := range events {
			if event == nil {
				continue
			}
			ids = append(ids, event.ID)
		}
		if len(ids) == 0 {
			events = []*model.OutboxEvent{}
			return nil
		}

		if err := tx.Model(&model.OutboxEvent{}).
			Where("id IN ?", ids).
			Updates(map[string]any{
				"status":    model.OutboxStatusProcessing,
				"locked_at": now.UTC(),
			}).Error; err != nil {
			return fmt.Errorf("mark outbox events processing: %w", err)
		}

		for _, event := range events {
			if event == nil {
				continue
			}
			event.Status = model.OutboxStatusProcessing
			lockedAt := now.UTC()
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
	if store.DB == nil {
		return fmt.Errorf("mark outbox published: mysql not initialized")
	}

	return store.DB.Model(&model.OutboxEvent{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":       model.OutboxStatusPublished,
			"published_at": publishedAt.UTC(),
			"locked_at":    nil,
			"last_error":   "",
		}).Error
}

func (r *OutboxRepository) MarkRetry(id uint, retryCount int, nextRetryAt time.Time, lastErr error) error {
	if store.DB == nil {
		return fmt.Errorf("mark outbox retry: mysql not initialized")
	}

	lastError := ""
	if lastErr != nil {
		lastError = lastErr.Error()
		if len(lastError) > 512 {
			lastError = lastError[:512]
		}
	}

	return store.DB.Model(&model.OutboxEvent{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":        model.OutboxStatusPending,
			"retry_count":   retryCount,
			"next_retry_at": nextRetryAt.UTC(),
			"locked_at":     nil,
			"last_error":    lastError,
		}).Error
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

func IsDuplicateOutboxError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}

	return false
}
