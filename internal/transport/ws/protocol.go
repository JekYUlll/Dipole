package ws

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	TypeConnected           = "connected"
	TypeError               = "error"
	TypePing                = "ping"
	TypePong                = "pong"
	TypeChatSend            = "chat.send"
	TypeChatSendFile        = "chat.send_file"
	TypeChatSent            = "chat.sent"
	TypeChatMessage         = "chat.message"
	TypeChatRead            = "chat.read"
	TypeGroupMessageNotify  = "group.message.notify"
	TypeSessionKicked       = "session.kicked"
	TypeGroupCreated        = "group.created"
	TypeGroupUpdated        = "group.updated"
	TypeGroupMembersAdded   = "group.members_added"
	TypeGroupMembersRemoved = "group.members_removed"
	TypeGroupDismissed      = "group.dismissed"
)

const (
	ErrorBadRequest        = "bad_request"
	ErrorUnsupportedType   = "unsupported_type"
	ErrorTargetUnavailable = "target_unavailable"
	ErrorPermissionDenied  = "permission_denied"
	ErrorRateLimited       = "rate_limited"
	ErrorInternal          = "internal"
)

type InboundEnvelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

type OutboundEvent struct {
	Type string `json:"type"`
	Data any    `json:"data,omitempty"`
}

type ConnectedEventData struct {
	UserUUID        string `json:"user_uuid"`
	ConnectionCount int    `json:"connection_count"`
	OnlineUserCount int    `json:"online_user_count"`
}

type PongData struct {
	ServerTime time.Time `json:"server_time"`
}

type ErrorEventData struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	RequestType string `json:"request_type,omitempty"`
}

type SendTextMessageInput struct {
	TargetUUID string `json:"target_uuid"`
	Content    string `json:"content"`
}

type SendFileMessageInput struct {
	TargetUUID string `json:"target_uuid"`
	FileID     string `json:"file_id"`
}

type FilePayload struct {
	FileID        string     `json:"file_id"`
	FileName      string     `json:"file_name"`
	FileSize      int64      `json:"file_size"`
	DownloadPath  string     `json:"download_path"`
	ContentPath   string     `json:"content_path"`
	ContentType   string     `json:"content_type"`
	FileExpiresAt *time.Time `json:"file_expires_at,omitempty"`
}

type ChatMessageData struct {
	MessageID   string       `json:"message_id"`
	FromUUID    string       `json:"from_uuid"`
	TargetUUID  string       `json:"target_uuid"`
	TargetType  int8         `json:"target_type"`
	MessageType int8         `json:"message_type"`
	Content     string       `json:"content"`
	File        *FilePayload `json:"file,omitempty"`
	SentAt      time.Time    `json:"sent_at"`
}

type ChatSentData struct {
	ChatMessageData
	Delivered bool `json:"delivered"`
}

type ChatReadData struct {
	ReaderUUID          string    `json:"reader_uuid"`
	TargetUUID          string    `json:"target_uuid"`
	TargetType          int8      `json:"target_type"`
	ConversationKey     string    `json:"conversation_key"`
	LastReadMessageUUID string    `json:"last_read_message_uuid"`
	ReadAt              time.Time `json:"read_at"`
}

type GroupMessageNotifyData struct {
	GroupUUID          string    `json:"group_uuid"`
	LatestMessageID    string    `json:"latest_message_id"`
	MessageType        int8      `json:"message_type"`
	Preview            string    `json:"preview"`
	RecentMessageCount int       `json:"recent_message_count"`
	SentAt             time.Time `json:"sent_at"`
}

type SessionKickedData struct {
	ConnectionID string    `json:"connection_id"`
	Reason       string    `json:"reason"`
	OccurredAt   time.Time `json:"occurred_at"`
}

type GroupUpdatedEventData struct {
	GroupUUID    string    `json:"group_uuid"`
	Name         string    `json:"name"`
	Notice       string    `json:"notice"`
	Avatar       string    `json:"avatar"`
	OperatorUUID string    `json:"operator_uuid"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type GroupCreatedEventData struct {
	GroupUUID    string    `json:"group_uuid"`
	Name         string    `json:"name"`
	Notice       string    `json:"notice"`
	Avatar       string    `json:"avatar"`
	MemberUUIDs  []string  `json:"member_uuids"`
	OperatorUUID string    `json:"operator_uuid"`
	OccurredAt   time.Time `json:"occurred_at"`
}

type GroupMembersChangedEventData struct {
	GroupUUID    string    `json:"group_uuid"`
	MemberUUIDs  []string  `json:"member_uuids"`
	OperatorUUID string    `json:"operator_uuid"`
	OccurredAt   time.Time `json:"occurred_at"`
}

type GroupDismissedEventData struct {
	GroupUUID    string    `json:"group_uuid"`
	GroupName    string    `json:"group_name"`
	OperatorUUID string    `json:"operator_uuid"`
	OccurredAt   time.Time `json:"occurred_at"`
}

func EncodeCommand(eventType string, data any) ([]byte, error) {
	payload, err := json.Marshal(OutboundEvent{
		Type: eventType,
		Data: data,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal websocket command: %w", err)
	}

	return payload, nil
}
