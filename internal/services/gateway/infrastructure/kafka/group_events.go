package kafka

import (
	"context"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/compat/service"
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
		payload, err := service.DecodeGroupEventPayload(envelope.EventType, envelope.Payload)
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
