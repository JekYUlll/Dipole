package ws

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/JekYUlll/Dipole/internal/model"
)

type inboundHandler interface {
	Handle(client *Client, payload []byte)
}

type Dispatcher struct {
	hub        *Hub
	userFinder userFinder
}

func NewDispatcher(hub *Hub, userFinder userFinder) *Dispatcher {
	return &Dispatcher{
		hub:        hub,
		userFinder: userFinder,
	}
}

func (d *Dispatcher) Handle(client *Client, payload []byte) {
	var envelope InboundEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		_ = client.SendError(ErrorBadRequest, "message payload is invalid", "")
		return
	}

	switch envelope.Type {
	case TypeChatSend:
		d.handleChatSend(client, envelope.Data)
	default:
		_ = client.SendError(
			ErrorUnsupportedType,
			fmt.Sprintf("unsupported message type: %s", envelope.Type),
			envelope.Type,
		)
	}
}

func (d *Dispatcher) handleChatSend(client *Client, raw json.RawMessage) {
	var input SendTextMessageInput
	if err := json.Unmarshal(raw, &input); err != nil {
		_ = client.SendError(ErrorBadRequest, "chat.send payload is invalid", TypeChatSend)
		return
	}

	targetUUID := strings.TrimSpace(input.TargetUUID)
	content := strings.TrimSpace(input.Content)
	if targetUUID == "" {
		_ = client.SendError(ErrorBadRequest, "target_uuid is required", TypeChatSend)
		return
	}
	if content == "" {
		_ = client.SendError(ErrorBadRequest, "content is required", TypeChatSend)
		return
	}
	if len([]rune(content)) > 1000 {
		_ = client.SendError(ErrorBadRequest, "content is too long", TypeChatSend)
		return
	}

	targetUser, err := d.userFinder.GetByUUID(targetUUID)
	if err != nil {
		client.log.Warn("lookup websocket target user failed",
			zap.String("target_uuid", targetUUID),
			zap.Error(err),
		)
		_ = client.SendError(ErrorInternal, "target user lookup failed", TypeChatSend)
		return
	}
	if targetUser == nil || targetUser.Status == model.UserStatusDisabled {
		_ = client.SendError(ErrorTargetUnavailable, "target user is unavailable", TypeChatSend)
		return
	}

	message := ChatMessageData{
		MessageID:  generateMessageID(),
		FromUUID:   client.user.UUID,
		TargetUUID: targetUUID,
		Content:    content,
		SentAt:     time.Now().UTC(),
	}

	deliveredCount := d.hub.SendEventToUser(targetUUID, TypeChatMessage, message)
	if targetUUID == client.user.UUID {
		deliveredCount = max(deliveredCount-1, 0)
	}

	ack := ChatSentData{
		ChatMessageData: message,
		Delivered:       deliveredCount > 0,
	}
	if err := client.SendEvent(TypeChatSent, ack); err != nil && !errors.Is(err, ErrClientClosed) {
		client.log.Warn("send websocket chat ack failed", zap.Error(err))
	}
}
