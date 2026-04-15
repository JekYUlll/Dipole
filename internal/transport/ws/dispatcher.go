package ws

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

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
	SendDirectFileMessage(senderUUID, targetUUID, fileUUID string) (*model.Message, error)
	SendGroupFileMessage(senderUUID, groupUUID, fileUUID string) (*model.Message, []string, error)
}

type conversationUpdater interface {
	UpdateDirectConversations(message *model.Message) error
	UpdateGroupConversations(message *model.Message) error
}

type messageRateLimiter interface {
	AllowMessageSend(userUUID string) (bool, time.Duration)
}

type Dispatcher struct {
	hub                 *Hub
	messageService      directMessageService
	conversationUpdater conversationUpdater
	syncDispatch        bool
	limiter             messageRateLimiter
}

func NewDispatcher(hub *Hub, messageService directMessageService, conversationUpdater conversationUpdater, syncDispatch bool) *Dispatcher {
	return &Dispatcher{
		hub:                 hub,
		messageService:      messageService,
		conversationUpdater: conversationUpdater,
		syncDispatch:        syncDispatch,
	}
}

func (d *Dispatcher) WithLimiter(limiter messageRateLimiter) *Dispatcher {
	d.limiter = limiter
	return d
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
	case TypeChatSendFile:
		d.handleChatSendFile(client, envelope.Data)
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

	if d.limiter != nil {
		allowed, retryAfter := d.limiter.AllowMessageSend(client.sessionUser.UUID)
		if !allowed {
			_ = client.SendError(
				ErrorRateLimited,
				formatRateLimitMessage("message send rate limit exceeded", retryAfter),
				TypeChatSend,
			)
			return
		}
	}

	targetUUID := strings.TrimSpace(input.TargetUUID)
	if strings.HasPrefix(targetUUID, "G") {
		d.handleGroupChatSend(client, targetUUID, input.Content)
		return
	}

	message, err := d.messageService.SendDirectMessage(client.sessionUser.UUID, targetUUID, input.Content)
	if err != nil {
		d.handleChatSendError(client, err, "target user is unavailable", TypeChatSend)
		return
	}

	// conversationUpdater is nil when Kafka is enabled; conversation updates are
	// handled asynchronously by the Kafka consumer in that case.
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

func (d *Dispatcher) handleChatSendFile(client *Client, raw json.RawMessage) {
	var input SendFileMessageInput
	if err := json.Unmarshal(raw, &input); err != nil {
		_ = client.SendError(ErrorBadRequest, "chat.send_file payload is invalid", TypeChatSendFile)
		return
	}

	if d.limiter != nil {
		allowed, retryAfter := d.limiter.AllowMessageSend(client.sessionUser.UUID)
		if !allowed {
			_ = client.SendError(
				ErrorRateLimited,
				formatRateLimitMessage("message send rate limit exceeded", retryAfter),
				TypeChatSendFile,
			)
			return
		}
	}

	targetUUID := strings.TrimSpace(input.TargetUUID)
	if strings.HasPrefix(targetUUID, "G") {
		d.handleGroupFileSend(client, targetUUID, input.FileID)
		return
	}

	message, err := d.messageService.SendDirectFileMessage(client.sessionUser.UUID, targetUUID, input.FileID)
	if err != nil {
		d.handleChatSendError(client, err, "target user is unavailable", TypeChatSendFile)
		return
	}

	// conversationUpdater is nil when Kafka is enabled; conversation updates are
	// handled asynchronously by the Kafka consumer in that case.
	if d.conversationUpdater != nil {
		if err := d.conversationUpdater.UpdateDirectConversations(message); err != nil {
			client.log.Warn("update direct conversations for file failed", zap.Error(err))
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
		client.log.Warn("send websocket file ack failed", zap.Error(err))
	}
}

func (d *Dispatcher) handleGroupChatSend(client *Client, groupUUID, content string) {
	message, recipients, err := d.messageService.SendGroupMessage(client.sessionUser.UUID, groupUUID, content)
	if err != nil {
		d.handleChatSendError(client, err, "target group is unavailable", TypeChatSend)
		return
	}

	// conversationUpdater is nil when Kafka is enabled; conversation updates are
	// handled asynchronously by the Kafka consumer in that case.
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

func (d *Dispatcher) handleGroupFileSend(client *Client, groupUUID, fileUUID string) {
	message, recipients, err := d.messageService.SendGroupFileMessage(client.sessionUser.UUID, groupUUID, fileUUID)
	if err != nil {
		d.handleChatSendError(client, err, "target group is unavailable", TypeChatSendFile)
		return
	}

	// conversationUpdater is nil when Kafka is enabled; conversation updates are
	// handled asynchronously by the Kafka consumer in that case.
	if d.conversationUpdater != nil {
		if err := d.conversationUpdater.UpdateGroupConversations(message); err != nil {
			client.log.Warn("update group conversations for file failed", zap.Error(err))
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
		client.log.Warn("send websocket group file ack failed", zap.Error(err))
	}
}

func (d *Dispatcher) handleChatSendError(client *Client, err error, unavailableMessage string, requestType string) {
	switch {
	case errors.Is(err, service.ErrMessageTargetRequired):
		_ = client.SendError(ErrorBadRequest, "target_uuid is required", requestType)
	case errors.Is(err, service.ErrMessageContentRequired):
		_ = client.SendError(ErrorBadRequest, "content is required", requestType)
	case errors.Is(err, service.ErrMessageContentTooLong):
		_ = client.SendError(ErrorBadRequest, "content is too long", requestType)
	case errors.Is(err, service.ErrMessageFileRequired):
		_ = client.SendError(ErrorBadRequest, "file_id is required", requestType)
	case errors.Is(err, service.ErrMessageFriendRequired), errors.Is(err, service.ErrMessageGroupForbidden):
		_ = client.SendError(ErrorPermissionDenied, "message send permission denied", requestType)
	case errors.Is(err, service.ErrMessageTargetUnavailable), errors.Is(err, service.ErrMessageTargetNotFound):
		_ = client.SendError(ErrorTargetUnavailable, unavailableMessage, requestType)
	case errors.Is(err, service.ErrMessageFileUnavailable):
		_ = client.SendError(ErrorBadRequest, "file is unavailable", requestType)
	default:
		client.log.Warn("persist websocket message failed", zap.Error(err))
		_ = client.SendError(ErrorInternal, "message send failed", requestType)
	}
}

func newChatMessageData(message *model.Message) ChatMessageData {
	data := ChatMessageData{
		MessageID:   message.UUID,
		FromUUID:    message.SenderUUID,
		TargetUUID:  message.TargetUUID,
		TargetType:  message.TargetType,
		MessageType: message.MessageType,
		Content:     message.Content,
		SentAt:      message.SentAt,
	}
	if message.MessageType == model.MessageTypeFile {
		data.File = &FilePayload{
			FileID:        message.FileID,
			FileName:      message.FileName,
			FileSize:      message.FileSize,
			DownloadPath:  "/api/v1/files/" + message.FileID + "/download",
			ContentType:   message.FileContentType,
			FileExpiresAt: message.FileExpiresAt,
		}
	}
	return data
}

func formatRateLimitMessage(message string, retryAfter time.Duration) string {
	seconds := int(retryAfter.Seconds())
	if retryAfter > 0 && seconds == 0 {
		seconds = 1
	}
	if seconds <= 0 {
		return message
	}

	return fmt.Sprintf("%s, retry after %d seconds", message, seconds)
}
