package kafka

import (
	"context"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/logger"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/platform/kafka"
	messagedomain "github.com/JekYUlll/Dipole/internal/services/message/domain"
	"go.uber.org/zap"
)

type MessagePersister interface {
	PersistRequestedMessage(messagedomain.MessageEventPayload) (*model.Message, error)
}

type MessagePersisterWithContext interface {
	PersistRequestedMessageContext(context.Context, messagedomain.MessageEventPayload) (*model.Message, error)
}

// RegisterPersistenceHandlers registers only the Message-owned persistence
// commands. Delivery and projection handlers remain at their owning services.
func RegisterPersistenceHandlers(subscriber *kafka.Consumer, persister MessagePersister) {
	if subscriber == nil || persister == nil {
		return
	}
	subscriber.Register("message.direct.send_requested", persistMessageHandler(persister, "direct"))
	subscriber.Register("message.group.send_requested", persistMessageHandler(persister, "group"))
}

func persistMessageHandler(persister MessagePersister, label string) kafka.Handler {
	return func(ctx context.Context, event kafka.Event) error {
		payload, err := decodeMessageEventPayload(event)
		if err != nil {
			logger.Warn("decode "+label+" message requested payload failed", zap.Error(err))
			return err
		}

		if contextual, ok := persister.(MessagePersisterWithContext); ok {
			_, err = contextual.PersistRequestedMessageContext(ctx, payload)
		} else {
			_, err = persister.PersistRequestedMessage(payload)
		}
		if err != nil {
			logger.Warn("persist "+label+" message from kafka failed", zap.Error(err))
			return err
		}

		logger.Info(label+" message persisted from kafka",
			zap.String("message_id", payload.MessageID),
			zap.Int64("offset", event.Offset),
		)
		return nil
	}
}

func decodeMessageEventPayload(event kafka.Event) (messagedomain.MessageEventPayload, error) {
	if event.Envelope == nil {
		return messagedomain.MessageEventPayload{}, fmt.Errorf("kafka event envelope is missing")
	}
	payload, err := messagedomain.DecodeMessageEventPayload(event.Envelope.EventType, event.Envelope.Payload)
	if err != nil {
		return messagedomain.MessageEventPayload{}, fmt.Errorf("decode message event contract: %w", err)
	}
	return payload, nil
}
