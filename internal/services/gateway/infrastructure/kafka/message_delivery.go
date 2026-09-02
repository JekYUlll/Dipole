package kafka

import (
	"context"
	"fmt"
	"strings"

	"github.com/JekYUlll/Dipole/internal/model"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	messagedomain "github.com/JekYUlll/Dipole/internal/services/message/domain"
	wsTransport "github.com/JekYUlll/Dipole/internal/transport/ws"
)

// NewDirectMessageHandler builds the Gateway direct-message delivery handler.
func NewDirectMessageHandler(hub EventSender, timelineNotifyMode string) platformKafka.Handler {
	return func(ctx context.Context, event platformKafka.Event) error {
		payload, err := decodeMessageEventPayload(event)
		if err != nil {
			return fmt.Errorf("decode direct message for delivery: %w", err)
		}

		if timelineNotifyMode != wsTransport.TimelineNotifyPrimary {
			sendEventToUser(ctx, hub, payload.TargetUUID, wsTransport.TypeChatMessage, chatMessageData(payload))
		}
		if notify, ok := timelineNotifyData(event.Envelope, payload, timelineNotifyMode); ok {
			sendEventToUser(ctx, hub, payload.TargetUUID, wsTransport.TypeSyncItemNotifyV1, notify)
		}
		return nil
	}
}

func decodeMessageEventPayload(event platformKafka.Event) (messagedomain.MessageEventPayload, error) {
	envelope, err := requireEnvelope(event)
	if err != nil {
		return messagedomain.MessageEventPayload{}, err
	}
	payload, err := messagedomain.DecodeMessageEventPayload(envelope.EventType, envelope.Payload)
	if err != nil {
		return messagedomain.MessageEventPayload{}, fmt.Errorf("decode message event contract: %w", err)
	}
	return payload, nil
}

func chatMessageData(payload messagedomain.MessageEventPayload) wsTransport.ChatMessageData {
	return wsTransport.ChatMessageData{
		MessageID: payload.MessageID, MessageSeq: payload.MessageSeq,
		FromUUID: payload.SenderUUID, TargetUUID: payload.TargetUUID,
		TargetType: payload.TargetType, MessageType: payload.MessageType,
		Content: payload.Content, File: payloadToWSFile(payload), SentAt: payload.SentAt,
	}
}

func payloadToWSFile(payload messagedomain.MessageEventPayload) *wsTransport.FilePayload {
	if payload.MessageType != model.MessageTypeFile {
		return nil
	}
	return &wsTransport.FilePayload{
		FileID: payload.FileID, FileName: payload.FileName, FileSize: payload.FileSize,
		DownloadPath: "/api/v1/files/" + payload.FileID + "/download",
		ContentType:  payload.FileContentType, FileExpiresAt: payload.FileExpiresAt,
	}
}

func timelineNotifyData(envelope *platformKafka.Envelope, payload messagedomain.MessageEventPayload, mode string) (wsTransport.SyncItemNotifyData, bool) {
	if (mode != wsTransport.TimelineNotifyShadow && mode != wsTransport.TimelineNotifyPrimary) ||
		payload.MessageSeq == 0 || strings.TrimSpace(payload.MessageID) == "" || strings.TrimSpace(payload.ConversationKey) == "" {
		return wsTransport.SyncItemNotifyData{}, false
	}
	eventID := payload.MessageID
	if envelope != nil && strings.TrimSpace(envelope.EventID) != "" {
		eventID = strings.TrimSpace(envelope.EventID)
	}
	return wsTransport.SyncItemNotifyData{
		SchemaVersion: "v1", EventID: eventID, MessageUUID: payload.MessageID,
		ConversationKey: payload.ConversationKey, MessageSeq: payload.MessageSeq,
		TargetType: payload.TargetType, TargetUUID: payload.TargetUUID,
	}, true
}
