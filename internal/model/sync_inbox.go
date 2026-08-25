package model

import "time"

// UserSyncInbox is the durable user-scoped timeline used for incremental sync.
// SyncSeq is globally allocated by MySQL; for each user it remains monotonic and
// may contain gaps, which is valid for cursor-based reads.
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
