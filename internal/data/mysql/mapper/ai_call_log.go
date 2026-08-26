package mapper

import (
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/model"
)

func AICallLogInsertParams(log *model.AICallLog) generated.InsertAICallLogParams {
	if log == nil {
		return generated.InsertAICallLogParams{}
	}
	return generated.InsertAICallLogParams{
		TriggerMessageUuid:  log.TriggerMessageUUID,
		ResponseMessageUuid: log.ResponseMessageUUID,
		ConversationKey:     log.ConversationKey,
		UserUuid:            log.UserUUID,
		AssistantUuid:       log.AssistantUUID,
		Provider:            log.Provider,
		Model:               log.Model,
		Status:              log.Status,
		ErrorMessage:        log.ErrorMessage,
		PromptTokens:        int64(log.PromptTokens),
		CompletionTokens:    int64(log.CompletionTokens),
		TotalTokens:         int64(log.TotalTokens),
		LatencyMs:           log.LatencyMS,
	}
}
