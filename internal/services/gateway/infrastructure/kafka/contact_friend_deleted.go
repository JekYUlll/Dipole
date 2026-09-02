package kafka

import (
	"context"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/application"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	wsTransport "github.com/JekYUlll/Dipole/internal/transport/ws"
)

// NewContactFriendDeletedHandler builds the Gateway handler for contact
// deletion notifications.
func NewContactFriendDeletedHandler(hub EventSender) platformKafka.Handler {
	return func(ctx context.Context, event platformKafka.Event) error {
		envelope, err := requireEnvelope(event)
		if err != nil {
			return fmt.Errorf("decode contact friend deleted envelope: %w", err)
		}

		payload, err := application.DecodeContactFriendDeletedPayload(envelope.EventType, envelope.Payload)
		if err != nil {
			return fmt.Errorf("decode contact friend deleted payload: %w", err)
		}

		sendEventToUser(ctx, hub, payload.UserUUID, wsTransport.TypeContactFriendDeleted, wsTransport.ContactFriendDeletedEventData{
			UserUUID:   payload.UserUUID,
			FriendUUID: payload.FriendUUID,
			OccurredAt: payload.OccurredAt,
		})
		return nil
	}
}
