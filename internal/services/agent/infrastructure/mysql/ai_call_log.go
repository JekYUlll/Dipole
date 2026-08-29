package agentmysql

import (
	"context"
	"errors"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/data/mysql/mapper"
	"github.com/JekYUlll/Dipole/internal/model"
)

var _ application.AICallLogStore = (*AICallLogRepository)(nil)

type AICallLogRepository struct {
	queries generated.Querier
}

func NewAICallLogRepository(queries generated.Querier) (*AICallLogRepository, error) {
	if queries == nil {
		return nil, errors.New("AI call log queries are required")
	}
	return &AICallLogRepository{queries: queries}, nil
}

func (r *AICallLogRepository) Begin(log *model.AICallLog) (bool, error) {
	if log == nil {
		return false, nil
	}
	rows, err := r.queries.InsertAICallLog(context.Background(), mapper.AICallLogInsertParams(log))
	if err != nil {
		return false, fmt.Errorf("create AI call log with sqlc: %w", err)
	}
	return rows > 0, nil
}

func (r *AICallLogRepository) MarkSucceeded(triggerMessageUUID, responseMessageUUID string, promptTokens, completionTokens, totalTokens int, latencyMS int64) error {
	err := r.queries.MarkAICallLogSucceeded(context.Background(), generated.MarkAICallLogSucceededParams{
		Status:              model.AICallStatusSucceeded,
		ResponseMessageUuid: responseMessageUUID,
		PromptTokens:        int64(promptTokens),
		CompletionTokens:    int64(completionTokens),
		TotalTokens:         int64(totalTokens),
		LatencyMs:           latencyMS,
		TriggerMessageUuid:  triggerMessageUUID,
	})
	if err != nil {
		return fmt.Errorf("mark AI call log succeeded with sqlc: %w", err)
	}
	return nil
}

func (r *AICallLogRepository) MarkFailed(triggerMessageUUID, errorMessage string, latencyMS int64) error {
	err := r.queries.MarkAICallLogFailed(context.Background(), generated.MarkAICallLogFailedParams{
		Status:             model.AICallStatusFailed,
		ErrorMessage:       errorMessage,
		LatencyMs:          latencyMS,
		TriggerMessageUuid: triggerMessageUUID,
	})
	if err != nil {
		return fmt.Errorf("mark AI call log failed with sqlc: %w", err)
	}
	return nil
}
