package ws

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	TypeConnected      = "connected"
	TypeError          = "error"
	TypeChatSend       = "chat.send"
	TypeChatSent       = "chat.sent"
	TypeChatMessage    = "chat.message"
	TypeGroupDismissed = "group.dismissed"
)

const (
	ErrorBadRequest        = "bad_request"
	ErrorUnsupportedType   = "unsupported_type"
	ErrorTargetUnavailable = "target_unavailable"
	ErrorPermissionDenied  = "permission_denied"
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

type ErrorEventData struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	RequestType string `json:"request_type,omitempty"`
}

type SendTextMessageInput struct {
	TargetUUID string `json:"target_uuid"`
	Content    string `json:"content"`
}

type ChatMessageData struct {
	MessageID  string    `json:"message_id"`
	FromUUID   string    `json:"from_uuid"`
	TargetUUID string    `json:"target_uuid"`
	TargetType int8      `json:"target_type"`
	Content    string    `json:"content"`
	SentAt     time.Time `json:"sent_at"`
}

type ChatSentData struct {
	ChatMessageData
	Delivered bool `json:"delivered"`
}

type GroupDismissedEventData struct {
	GroupUUID string `json:"group_uuid"`
	GroupName string `json:"group_name"`
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
