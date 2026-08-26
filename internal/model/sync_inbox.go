package model

import "time"

// UserSyncState serializes inbox writes for one user.
type UserSyncState struct {
	UserUUID  string    `json:"user_uuid"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UserSyncInbox is the durable user-scoped timeline used for incremental sync.
type UserSyncInbox struct {
	SyncSeq         uint64    `json:"sync_seq"`
	UserUUID        string    `json:"user_uuid"`
	MessageUUID     string    `json:"message_uuid"`
	ConversationKey string    `json:"conversation_key"`
	MessageSeq      uint64    `json:"message_seq"`
	CreatedAt       time.Time `json:"created_at"`
}

type SyncMessage struct {
	SyncSeq         uint64   `json:"sync_seq"`
	ConversationKey string   `json:"conversation_key"`
	MessageUUID     string   `json:"message_uuid"`
	MessageSeq      uint64   `json:"message_seq"`
	Message         *Message `json:"message"`
}
