package ws

import (
	"encoding/json"
	"errors"
	"fmt"

	"go.uber.org/zap"

	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/service"
)

type inboundHandler interface {
	Handle(client *Client, payload []byte)
}

type directMessageService interface {
	SendDirectMessage(senderUUID, targetUUID, content string) (*model.Message, error)
}

type conversationUpdater interface {
	UpdateDirectConversations(message *model.Message) error
}

type Dispatcher struct {
	hub                 *Hub
	messageService      directMessageService
	conversationUpdater conversationUpdater
}

func NewDispatcher(hub *Hub, messageService directMessageService, conversationUpdater conversationUpdater) *Dispatcher {
	return &Dispatcher{
		hub:                 hub,
		messageService:      messageService,
		conversationUpdater: conversationUpdater,
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

	message, err := d.messageService.SendDirectMessage(client.sessionUser.UUID, input.TargetUUID, input.Content)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrMessageTargetRequired):
			_ = client.SendError(ErrorBadRequest, "target_uuid is required", TypeChatSend)
		case errors.Is(err, service.ErrMessageContentRequired):
			_ = client.SendError(ErrorBadRequest, "content is required", TypeChatSend)
		case errors.Is(err, service.ErrMessageContentTooLong):
			_ = client.SendError(ErrorBadRequest, "content is too long", TypeChatSend)
		case errors.Is(err, service.ErrMessageFriendRequired):
			_ = client.SendError(ErrorPermissionDenied, "direct message requires friendship", TypeChatSend)
		case errors.Is(err, service.ErrMessageTargetUnavailable):
			_ = client.SendError(ErrorTargetUnavailable, "target user is unavailable", TypeChatSend)
		default:
			client.log.Warn("persist websocket direct message failed", zap.Error(err))
			_ = client.SendError(ErrorInternal, "message send failed", TypeChatSend)
		}
		return
	}

	if d.conversationUpdater != nil {
		if err := d.conversationUpdater.UpdateDirectConversations(message); err != nil {
			client.log.Warn("update direct conversations failed", zap.Error(err))
		}
	}

	eventData := newChatMessageData(message)
	deliveredCount := d.hub.SendEventToUser(message.TargetUUID, TypeChatMessage, eventData)
	if message.TargetUUID == client.sessionUser.UUID {
		deliveredCount = max(deliveredCount-1, 0)
	}

	ack := ChatSentData{
		ChatMessageData: eventData,
		Delivered:       deliveredCount > 0,
	}
	if err := client.SendEvent(TypeChatSent, ack); err != nil && !errors.Is(err, ErrClientClosed) {
		client.log.Warn("send websocket chat ack failed", zap.Error(err))
	}
}

func newChatMessageData(message *model.Message) ChatMessageData {
	return ChatMessageData{
		MessageID:  message.UUID,
		FromUUID:   message.SenderUUID,
		TargetUUID: message.TargetUUID,
		Content:    message.Content,
		SentAt:     message.SentAt,
	}
}
