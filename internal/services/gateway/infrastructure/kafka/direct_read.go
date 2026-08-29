package kafka

import (
	"context"
	"fmt"

	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	coreconversation "github.com/JekYUlll/Dipole/internal/services/core/domain/conversation"
	wsTransport "github.com/JekYUlll/Dipole/internal/transport/ws"
)

type ContextEventSender interface {
	SendEventToUserContext(ctx context.Context, userUUID, eventType string, data any) int
}

// NewDirectReadHandler builds the Gateway handler for direct-conversation read
// receipts while keeping event decoding inside the service boundary.
func NewDirectReadHandler(hub EventSender) platformKafka.Handler {
	return func(ctx context.Context, event platformKafka.Event) error {
		envelope, err := requireEnvelope(event)
		if err != nil {
			return fmt.Errorf("decode direct read envelope: %w", err)
		}

		payload, err := coreconversation.DecodeReadReceipt(envelope.EventType, envelope.Payload)
		if err != nil {
			return fmt.Errorf("decode direct read payload: %w", err)
		}

		sendEventToUser(ctx, hub, payload.TargetUUID, wsTransport.TypeChatRead, wsTransport.ChatReadData{
			ReaderUUID:          payload.ReaderUUID,
			TargetUUID:          payload.TargetUUID,
			TargetType:          payload.TargetType,
			ConversationKey:     payload.ConversationKey,
			LastReadMessageUUID: payload.LastReadMessageUUID,
			LastReadSeq:         payload.LastReadSeq,
			ReadAt:              payload.ReadAt,
		})
		return nil
	}
}

func sendEventToUser(ctx context.Context, hub EventSender, userUUID, eventType string, data any) int {
	if contextual, ok := hub.(ContextEventSender); ok {
		return contextual.SendEventToUserContext(ctx, userUUID, eventType, data)
	}
	return hub.SendEventToUser(userUUID, eventType, data)
}

func requireEnvelope(event platformKafka.Event) (*platformKafka.Envelope, error) {
	if event.Envelope == nil {
		return nil, fmt.Errorf("kafka event envelope is missing")
	}
	return event.Envelope, nil
}
