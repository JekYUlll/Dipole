package model

// SyncProjection is the immutable Inbox locator derived from a created-message event.
type SyncProjection struct {
	EventID         string
	MessageUUID     string
	ConversationKey string
	MessageSeq      uint64
	RecipientUUIDs  []string
}
