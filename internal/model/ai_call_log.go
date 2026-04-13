package model

import "time"

const (
	AICallStatusPending int8 = iota
	AICallStatusSucceeded
	AICallStatusFailed
)

type AICallLog struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	TriggerMessageUUID  string    `gorm:"column:trigger_message_uuid;size:24;uniqueIndex;not null" json:"trigger_message_uuid"`
	ResponseMessageUUID string    `gorm:"column:response_message_uuid;size:24;index;not null;default:''" json:"response_message_uuid"`
	ConversationKey     string    `gorm:"column:conversation_key;size:64;index;not null" json:"conversation_key"`
	UserUUID            string    `gorm:"column:user_uuid;size:24;index;not null" json:"user_uuid"`
	AssistantUUID       string    `gorm:"column:assistant_uuid;size:24;index;not null" json:"assistant_uuid"`
	Provider            string    `gorm:"size:32;not null;default:''" json:"provider"`
	Model               string    `gorm:"size:64;not null;default:''" json:"model"`
	Status              int8      `gorm:"not null;default:0;index" json:"status"`
	ErrorMessage        string    `gorm:"column:error_message;size:512;not null;default:''" json:"error_message"`
	PromptTokens        int       `gorm:"column:prompt_tokens;not null;default:0" json:"prompt_tokens"`
	CompletionTokens    int       `gorm:"column:completion_tokens;not null;default:0" json:"completion_tokens"`
	TotalTokens         int       `gorm:"column:total_tokens;not null;default:0" json:"total_tokens"`
	LatencyMS           int64     `gorm:"column:latency_ms;not null;default:0" json:"latency_ms"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func (AICallLog) TableName() string {
	return "ai_call_logs"
}
