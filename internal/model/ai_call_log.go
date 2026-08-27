package model

import "time"

const (
	AICallStatusPending int8 = iota
	AICallStatusSucceeded
	AICallStatusFailed
)

type AICallLog struct {
	ID                  uint      `json:"id"`
	TriggerMessageUUID  string    `json:"trigger_message_uuid"`
	ResponseMessageUUID string    `json:"response_message_uuid"`
	ConversationKey     string    `json:"conversation_key"`
	UserUUID            string    `json:"user_uuid"`
	AssistantUUID       string    `json:"assistant_uuid"`
	Provider            string    `json:"provider"`
	Model               string    `json:"model"`
	Status              int8      `json:"status"`
	ErrorMessage        string    `json:"error_message"`
	PromptTokens        int       `json:"prompt_tokens"`
	CompletionTokens    int       `json:"completion_tokens"`
	TotalTokens         int       `json:"total_tokens"`
	LatencyMS           int64     `json:"latency_ms"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}
