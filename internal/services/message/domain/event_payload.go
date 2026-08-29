package messagedomain

import "time"

// MessageEventPayload is the versioned payload shared by Message producers
// and event consumers.
type MessageEventPayload struct {
	MutationType    MessageMutationType `json:"mutation_type,omitempty"`
	Revision        uint64              `json:"revision,omitempty"`
	ActorUUID       string              `json:"actor_uuid,omitempty"`
	MessageID       string              `json:"message_id"`
	ClientMessageID string              `json:"client_message_id,omitempty"`
	ConversationKey string              `json:"conversation_key"`
	MessageSeq      uint64              `json:"message_seq,omitempty"`
	SenderUUID      string              `json:"sender_uuid"`
	TargetUUID      string              `json:"target_uuid"`
	TargetType      int8                `json:"target_type"`
	MessageType     int8                `json:"message_type"`
	Content         string              `json:"content"`
	FileID          string              `json:"file_id,omitempty"`
	FileName        string              `json:"file_name,omitempty"`
	FileSize        int64               `json:"file_size,omitempty"`
	FileURL         string              `json:"file_url,omitempty"`
	FileContentType string              `json:"file_content_type,omitempty"`
	FileExpiresAt   *time.Time          `json:"file_expires_at,omitempty"`
	SentAt          time.Time           `json:"sent_at"`
	RecipientUUIDs  []string            `json:"recipient_uuids,omitempty"`
	SyncFanout      *bool               `json:"sync_fanout,omitempty"`
}
