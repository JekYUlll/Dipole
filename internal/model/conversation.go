package model

import "time"

type Conversation struct {
	ID                    uint      `json:"id"`
	UserUUID              string    `json:"user_uuid"`
	TargetType            int8      `json:"target_type"`
	TargetUUID            string    `json:"target_uuid"`
	ConversationKey       string    `json:"conversation_key"`
	LastMessageUUID       string    `json:"last_message_uuid"`
	LastMessageType       int8      `json:"last_message_type"`
	LastMessagePreview    string    `json:"last_message_preview"`
	LastMessageAt         time.Time `json:"last_message_at"`
	LastMessageSenderUUID string    `json:"last_message_sender_uuid"`
	UnreadCount           int       `json:"unread_count"`
	Remark                string    `json:"remark"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}
