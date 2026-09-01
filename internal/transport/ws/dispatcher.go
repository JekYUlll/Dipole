package ws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/platform/correlation"
	messagedomain "github.com/JekYUlll/Dipole/internal/services/message/domain"
)

type inboundHandler interface {
	Handle(client *Client, payload []byte)
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
	messageService      applicationPort.MessageCommand
	conversationUpdater conversationUpdater
	syncDispatch        bool
	limiter             messageRateLimiter
	timelineNotifyMode  string
}

func (d *Dispatcher) WithTimelineNotifyMode(mode string) *Dispatcher {
	d.timelineNotifyMode = mode
	return d
}

func NewDispatcher(hub *Hub, messageService applicationPort.MessageCommand, conversationUpdater conversationUpdater, syncDispatch bool) *Dispatcher {
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
		_ = client.SendError(ErrorBadRequest, "message payload is invalid", "", "")
		return
	}
	ctx, _ := correlation.Ensure(context.Background(), envelope.RequestID, envelope.TraceID)

	switch envelope.Type {
	case TypePing:
		_ = client.SendEvent(TypePong, PongData{ServerTime: time.Now().UTC()})
	case TypeChatSend:
		d.handleChatSend(ctx, client, envelope.Data)
	case TypeChatSendFile:
		d.handleChatSendFile(ctx, client, envelope.Data)
	default:
		_ = client.SendError(
			ErrorUnsupportedType,
			fmt.Sprintf("unsupported message type: %s", envelope.Type),
			envelope.Type,
			"",
		)
	}
}

func (d *Dispatcher) handleChatSend(ctx context.Context, client *Client, raw json.RawMessage) {
	var input SendTextMessageInput
	if err := json.Unmarshal(raw, &input); err != nil {
		_ = client.SendError(ErrorBadRequest, "chat.send payload is invalid", TypeChatSend, "")
		return
	}

	if d.limiter != nil {
		allowed, retryAfter := d.limiter.AllowMessageSend(client.sessionUser.UUID)
		if !allowed {
			_ = client.SendError(
				ErrorRateLimited,
				formatRateLimitMessage("message send rate limit exceeded", retryAfter),
				TypeChatSend,
				input.ClientMessageID,
			)
			return
		}
	}

	targetUUID := strings.TrimSpace(input.TargetUUID)
	if strings.HasPrefix(targetUUID, "G") {
		d.handleGroupChatSend(ctx, client, targetUUID, input.Content, input.ClientMessageID)
		return
	}

	var message *model.Message
	var err error
	if contextual, ok := d.messageService.(applicationPort.MessageCommandContext); ok {
		message, err = contextual.SendDirectMessageContext(ctx, client.sessionUser.UUID, targetUUID, input.Content, input.ClientMessageID)
	} else {
		message, err = d.messageService.SendDirectMessage(client.sessionUser.UUID, targetUUID, input.Content, input.ClientMessageID)
	}
	if err != nil {
		d.handleChatSendError(client, err, "target user is unavailable", TypeChatSend, input.ClientMessageID)
		return
	}

	d.dispatchDirect(ctx, client, message)
}

func (d *Dispatcher) handleChatSendFile(ctx context.Context, client *Client, raw json.RawMessage) {
	var input SendFileMessageInput
	if err := json.Unmarshal(raw, &input); err != nil {
		_ = client.SendError(ErrorBadRequest, "chat.send_file payload is invalid", TypeChatSendFile, "")
		return
	}

	if d.limiter != nil {
		allowed, retryAfter := d.limiter.AllowMessageSend(client.sessionUser.UUID)
		if !allowed {
			_ = client.SendError(
				ErrorRateLimited,
				formatRateLimitMessage("message send rate limit exceeded", retryAfter),
				TypeChatSendFile,
				input.ClientMessageID,
			)
			return
		}
	}

	targetUUID := strings.TrimSpace(input.TargetUUID)
	if strings.HasPrefix(targetUUID, "G") {
		d.handleGroupFileSend(ctx, client, targetUUID, input.FileID, input.ClientMessageID)
		return
	}

	var message *model.Message
	var err error
	if contextual, ok := d.messageService.(applicationPort.MessageCommandContext); ok {
		message, err = contextual.SendDirectFileMessageContext(ctx, client.sessionUser.UUID, targetUUID, input.FileID, input.ClientMessageID)
	} else {
		message, err = d.messageService.SendDirectFileMessage(client.sessionUser.UUID, targetUUID, input.FileID, input.ClientMessageID)
	}
	if err != nil {
		d.handleChatSendError(client, err, "target user is unavailable", TypeChatSendFile, input.ClientMessageID)
		return
	}

	d.dispatchDirect(ctx, client, message)
}

func (d *Dispatcher) handleGroupChatSend(ctx context.Context, client *Client, groupUUID, content string, clientMessageID string) {
	var message *model.Message
	var recipients []string
	var err error
	if contextual, ok := d.messageService.(applicationPort.MessageCommandContext); ok {
		message, recipients, err = contextual.SendGroupMessageContext(ctx, client.sessionUser.UUID, groupUUID, content, clientMessageID)
	} else {
		message, recipients, err = d.messageService.SendGroupMessage(client.sessionUser.UUID, groupUUID, content, clientMessageID)
	}
	if err != nil {
		d.handleChatSendError(client, err, "target group is unavailable", TypeChatSend, clientMessageID)
		return
	}

	d.dispatchGroup(ctx, client, message, recipients)
}

func (d *Dispatcher) handleGroupFileSend(ctx context.Context, client *Client, groupUUID, fileUUID string, clientMessageID string) {
	var message *model.Message
	var recipients []string
	var err error
	if contextual, ok := d.messageService.(applicationPort.MessageCommandContext); ok {
		message, recipients, err = contextual.SendGroupFileMessageContext(ctx, client.sessionUser.UUID, groupUUID, fileUUID, clientMessageID)
	} else {
		message, recipients, err = d.messageService.SendGroupFileMessage(client.sessionUser.UUID, groupUUID, fileUUID, clientMessageID)
	}
	if err != nil {
		d.handleChatSendError(client, err, "target group is unavailable", TypeChatSendFile, clientMessageID)
		return
	}

	d.dispatchGroup(ctx, client, message, recipients)
}

// dispatchDirect handles post-send steps for direct messages: conversation
// update, optional sync WS push, and ACK back to sender.
func (d *Dispatcher) dispatchDirect(ctx context.Context, client *Client, message *model.Message) {
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
		if d.timelineNotifyMode == TimelineNotifyPrimary {
			deliveredCount = d.sendTimelineNotification(message.TargetUUID, message)
		} else {
			deliveredCount = d.hub.SendEventToUser(message.TargetUUID, TypeChatMessage, eventData)
			d.sendTimelineNotification(message.TargetUUID, message)
		}
		if message.TargetUUID == client.sessionUser.UUID {
			deliveredCount = max(deliveredCount-1, 0)
		}
	}
	ack := ChatSentData{
		ChatMessageData: eventData,
		Delivered:       deliveredCount > 0,
		ClientMessageID: message.ClientMessageID,
	}
	if err := client.SendEventContext(ctx, TypeChatSent, ack); err != nil && !errors.Is(err, ErrClientClosed) {
		client.log.Warn("send websocket chat ack failed", zap.Error(err))
	}
}

// dispatchGroup handles post-send steps for group messages: conversation
// update, optional sync WS push to each recipient, and ACK back to sender.
func (d *Dispatcher) dispatchGroup(ctx context.Context, client *Client, message *model.Message, recipients []string) {
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
			if d.timelineNotifyMode == TimelineNotifyPrimary {
				deliveredCount += d.sendTimelineNotification(recipientUUID, message)
			} else {
				deliveredCount += d.hub.SendEventToUser(recipientUUID, TypeChatMessage, eventData)
				d.sendTimelineNotification(recipientUUID, message)
			}
		}
	}
	ack := ChatSentData{
		ChatMessageData: eventData,
		Delivered:       deliveredCount > 0,
		ClientMessageID: message.ClientMessageID,
	}
	if err := client.SendEventContext(ctx, TypeChatSent, ack); err != nil && !errors.Is(err, ErrClientClosed) {
		client.log.Warn("send websocket group chat ack failed", zap.Error(err))
	}
}

func (d *Dispatcher) sendTimelineNotification(recipientUUID string, message *model.Message) int {
	if (d.timelineNotifyMode != TimelineNotifyShadow && d.timelineNotifyMode != TimelineNotifyPrimary) || message == nil || message.Seq == 0 || strings.TrimSpace(message.UUID) == "" || strings.TrimSpace(message.ConversationKey) == "" {
		return 0
	}
	return d.hub.SendEventToUser(recipientUUID, TypeSyncItemNotifyV1, SyncItemNotifyData{
		SchemaVersion: "v1", EventID: message.UUID, MessageUUID: message.UUID,
		ConversationKey: message.ConversationKey, MessageSeq: message.Seq,
		TargetType: message.TargetType, TargetUUID: message.TargetUUID,
	})
}

func (d *Dispatcher) handleChatSendError(client *Client, err error, unavailableMessage string, requestType string, clientMessageID string) {
	switch {
	case errors.Is(err, messagedomain.ErrMessageTargetRequired):
		_ = client.SendError(ErrorBadRequest, "target_uuid is required", requestType, clientMessageID)
	case errors.Is(err, messagedomain.ErrMessageContentRequired):
		_ = client.SendError(ErrorBadRequest, "content is required", requestType, clientMessageID)
	case errors.Is(err, messagedomain.ErrMessageContentTooLong):
		_ = client.SendError(ErrorBadRequest, "content is too long", requestType, clientMessageID)
	case errors.Is(err, messagedomain.ErrMessageFileRequired):
		_ = client.SendError(ErrorBadRequest, "file_id is required", requestType, clientMessageID)
	case errors.Is(err, messagedomain.ErrMessageFriendRequired), errors.Is(err, messagedomain.ErrMessageGroupForbidden):
		_ = client.SendError(ErrorPermissionDenied, "message send permission denied", requestType, clientMessageID)
	case errors.Is(err, messagedomain.ErrMessageTargetUnavailable), errors.Is(err, messagedomain.ErrMessageTargetNotFound):
		_ = client.SendError(ErrorTargetUnavailable, unavailableMessage, requestType, clientMessageID)
	case errors.Is(err, messagedomain.ErrMessageFileUnavailable):
		_ = client.SendError(ErrorBadRequest, "file is unavailable", requestType, clientMessageID)
	default:
		client.log.Warn("persist websocket message failed", zap.Error(err))
		_ = client.SendError(ErrorInternal, "message send failed", requestType, clientMessageID)
	}
}

func newChatMessageData(message *model.Message) ChatMessageData {
	data := ChatMessageData{
		MessageID:   message.UUID,
		MessageSeq:  message.Seq,
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
			ContentPath:   "/api/v1/files/" + message.FileID + "/content",
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
