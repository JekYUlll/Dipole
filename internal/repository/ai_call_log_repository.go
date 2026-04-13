package repository

import (
	"fmt"

	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/store"
	"gorm.io/gorm/clause"
)

type AICallLogRepository struct{}

func NewAICallLogRepository() *AICallLogRepository {
	return &AICallLogRepository{}
}

func (r *AICallLogRepository) Begin(log *model.AICallLog) (bool, error) {
	if log == nil {
		return false, nil
	}

	result := store.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "trigger_message_uuid"}},
		DoNothing: true,
	}).Create(log)
	if result.Error != nil {
		return false, fmt.Errorf("create ai call log: %w", result.Error)
	}

	return result.RowsAffected > 0, nil
}

func (r *AICallLogRepository) MarkSucceeded(triggerMessageUUID, responseMessageUUID string, promptTokens, completionTokens, totalTokens int, latencyMS int64) error {
	if err := store.DB.Model(&model.AICallLog{}).
		Where("trigger_message_uuid = ?", triggerMessageUUID).
		Updates(map[string]any{
			"status":                model.AICallStatusSucceeded,
			"response_message_uuid": responseMessageUUID,
			"prompt_tokens":         promptTokens,
			"completion_tokens":     completionTokens,
			"total_tokens":          totalTokens,
			"latency_ms":            latencyMS,
			"error_message":         "",
		}).Error; err != nil {
		return fmt.Errorf("mark ai call log succeeded: %w", err)
	}

	return nil
}

func (r *AICallLogRepository) MarkFailed(triggerMessageUUID, errorMessage string, latencyMS int64) error {
	if err := store.DB.Model(&model.AICallLog{}).
		Where("trigger_message_uuid = ?", triggerMessageUUID).
		Updates(map[string]any{
			"status":        model.AICallStatusFailed,
			"error_message": errorMessage,
			"latency_ms":    latencyMS,
		}).Error; err != nil {
		return fmt.Errorf("mark ai call log failed: %w", err)
	}

	return nil
}
