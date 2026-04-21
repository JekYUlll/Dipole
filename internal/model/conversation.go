package model

import "time"

type Conversation struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	UserUUID           string    `gorm:"column:user_uuid;size:24;not null;index;index:idx_conversation_user_last_message_at,priority:1;uniqueIndex:idx_user_conversation,priority:1" json:"user_uuid"`
	TargetType         int8      `gorm:"column:target_type;not null;default:0" json:"target_type"`
	TargetUUID         string    `gorm:"column:target_uuid;size:24;not null;index" json:"target_uuid"`
	ConversationKey    string    `gorm:"column:conversation_key;size:64;not null;uniqueIndex:idx_user_conversation,priority:2" json:"conversation_key"`
	LastMessageUUID    string    `gorm:"column:last_message_uuid;size:24;not null" json:"last_message_uuid"`
	LastMessageType    int8      `gorm:"column:last_message_type;not null;default:0" json:"last_message_type"`
	LastMessagePreview string    `gorm:"column:last_message_preview;size:255;not null;default:''" json:"last_message_preview"`
	LastMessageAt      time.Time `gorm:"column:last_message_at;not null;index;index:idx_conversation_user_last_message_at,priority:2" json:"last_message_at"`
	UnreadCount        int       `gorm:"column:unread_count;not null;default:0" json:"unread_count"`
	Remark             string    `gorm:"size:50;not null;default:''" json:"remark"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func (Conversation) TableName() string {
	return "conversations"
}
