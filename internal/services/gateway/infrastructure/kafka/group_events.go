package kafka

import (
	"context"
	"fmt"

	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	coregroup "github.com/JekYUlll/Dipole/internal/services/core/domain/group"
)

// NewGroupEventHandler builds a Gateway fan-out handler for a group event.
func NewGroupEventHandler[T any](
	hub EventSender,
	eventType string,
	buildData func(coregroup.GroupEventPayload) T,
) platformKafka.Handler {
	return func(ctx context.Context, event platformKafka.Event) error {
		envelope, err := requireEnvelope(event)
		if err != nil {
			return fmt.Errorf("decode %s envelope: %w", eventType, err)
		}
		payload, err := coregroup.DecodeEventPayload(envelope.EventType, envelope.Payload)
		if err != nil {
			return fmt.Errorf("decode %s payload: %w", eventType, err)
		}

		data := buildData(payload)
		for _, recipientUUID := range payload.RecipientUUIDs {
			sendEventToUser(ctx, hub, recipientUUID, eventType, data)
		}
		return nil
	}
}
