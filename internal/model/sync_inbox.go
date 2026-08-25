package model

import "time"

// UserSyncState serializes inbox writes for one user. The row lock is held by
// the surrounding message transaction until its inbox rows become visible.
type UserSyncState struct {
	UserUUID  string    `gorm:"column:user_uuid;size:24;primaryKey" json:"user_uuid"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (UserSyncState) TableName() string {
	return "user_sync_states"
}

// UserSyncInbox is the durable user-scoped timeline used for incremental sync.
// SyncSeq is globally allocated by MySQL. UserSyncState row locks serialize
// allocations for each user, so gaps remain valid without commit-order skips.
type UserSyncInbox struct {
	SyncSeq         uint64    `gorm:"column:sync_seq;primaryKey;autoIncrement;index:idx_sync_inbox_user_seq,priority:2" json:"sync_seq"`
	UserUUID        string    `gorm:"column:user_uuid;size:24;not null;index:idx_sync_inbox_user_seq,priority:1;uniqueIndex:idx_sync_inbox_user_message,priority:1" json:"user_uuid"`
	MessageUUID     string    `gorm:"column:message_uuid;size:24;not null;index;uniqueIndex:idx_sync_inbox_user_message,priority:2" json:"message_uuid"`
	ConversationKey string    `gorm:"column:conversation_key;size:64;not null;index" json:"conversation_key"`
	CreatedAt       time.Time `json:"created_at"`
}

func (UserSyncInbox) TableName() string {
	return "user_sync_inbox"
}

type SyncMessage struct {
	SyncSeq         uint64   `json:"sync_seq"`
	ConversationKey string   `json:"conversation_key"`
	Message         *Message `json:"message"`
}
