package ws

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/service"
)

type inboundHandler interface {
	Handle(client *Client, payload []byte)
}

type directMessageService interface {
	SendDirectMessage(senderUUID, targetUUID, content string) (*model.Message, error)
	SendGroupMessage(senderUUID, groupUUID, content string) (*model.Message, []string, error)
}

type conversationUpdater interface {
	UpdateDirectConversations(message *model.Message) error
	UpdateGroupConversations(message *model.Message) error
}

type Dispatcher struct {
	hub                 *Hub
	messageService      directMessageService
	conversationUpdater conversationUpdater
	syncDispatch        bool
}

func NewDispatcher(hub *Hub, messageService directMessageService, conversationUpdater conversationUpdater, syncDispatch bool) *Dispatcher {
	return &Dispatcher{
		hub:                 hub,
		messageService:      messageService,
		conversationUpdater: conversationUpdater,
		syncDispatch:        syncDispatch,
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
	if strings.HasPrefix(targetUUID, "G") {
		d.handleGroupChatSend(client, targetUUID, input.Content)
		return
	}

	message, err := d.messageService.SendDirectMessage(client.sessionUser.UUID, targetUUID, input.Content)
	if err != nil {
		d.handleChatSendError(client, err, "target user is unavailable")
		return
	}

	if d.conversationUpdater != nil {
		if err := d.conversationUpdater.UpdateDirectConversations(message); err != nil {
			client.log.Warn("update direct conversations failed", zap.Error(err))
		}
	}

	eventData := newChatMessageData(message)
	deliveredCount := 0
	if d.syncDispatch {
		deliveredCount = d.hub.SendEventToUser(message.TargetUUID, TypeChatMessage, eventData)
		if message.TargetUUID == client.sessionUser.UUID {
			deliveredCount = max(deliveredCount-1, 0)
		}
	}
	ack := ChatSentData{
		ChatMessageData: eventData,
		Delivered:       deliveredCount > 0,
	}
	if err := client.SendEvent(TypeChatSent, ack); err != nil && !errors.Is(err, ErrClientClosed) {
		client.log.Warn("send websocket chat ack failed", zap.Error(err))
	}
}

func (d *Dispatcher) handleGroupChatSend(client *Client, groupUUID, content string) {
	message, recipients, err := d.messageService.SendGroupMessage(client.sessionUser.UUID, groupUUID, content)
	if err != nil {
		d.handleChatSendError(client, err, "target group is unavailable")
		return
	}

	if d.conversationUpdater != nil {
		if err := d.conversationUpdater.UpdateGroupConversations(message); err != nil {
			client.log.Warn("update group conversations failed", zap.Error(err))
		}
	}

	eventData := newChatMessageData(message)
	deliveredCount := 0
	if d.syncDispatch {
		for _, recipientUUID := range recipients {
			if recipientUUID == client.sessionUser.UUID {
				continue
			}
			deliveredCount += d.hub.SendEventToUser(recipientUUID, TypeChatMessage, eventData)
		}
	}

	ack := ChatSentData{
		ChatMessageData: eventData,
		Delivered:       deliveredCount > 0,
	}
	if err := client.SendEvent(TypeChatSent, ack); err != nil && !errors.Is(err, ErrClientClosed) {
		client.log.Warn("send websocket group chat ack failed", zap.Error(err))
	}
}

func (d *Dispatcher) handleChatSendError(client *Client, err error, unavailableMessage string) {
	switch {
	case errors.Is(err, service.ErrMessageTargetRequired):
		_ = client.SendError(ErrorBadRequest, "target_uuid is required", TypeChatSend)
	case errors.Is(err, service.ErrMessageContentRequired):
		_ = client.SendError(ErrorBadRequest, "content is required", TypeChatSend)
	case errors.Is(err, service.ErrMessageContentTooLong):
		_ = client.SendError(ErrorBadRequest, "content is too long", TypeChatSend)
	case errors.Is(err, service.ErrMessageFriendRequired), errors.Is(err, service.ErrMessageGroupForbidden):
		_ = client.SendError(ErrorPermissionDenied, "message send permission denied", TypeChatSend)
	case errors.Is(err, service.ErrMessageTargetUnavailable), errors.Is(err, service.ErrMessageTargetNotFound):
		_ = client.SendError(ErrorTargetUnavailable, unavailableMessage, TypeChatSend)
	default:
		client.log.Warn("persist websocket message failed", zap.Error(err))
		_ = client.SendError(ErrorInternal, "message send failed", TypeChatSend)
	}
}

func newChatMessageData(message *model.Message) ChatMessageData {
	return ChatMessageData{
		MessageID:  message.UUID,
		FromUUID:   message.SenderUUID,
		TargetUUID: message.TargetUUID,
		TargetType: message.TargetType,
		Content:    message.Content,
		SentAt:     message.SentAt,
	}
}
