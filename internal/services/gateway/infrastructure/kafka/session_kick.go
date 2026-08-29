package kafka

import (
	"context"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/compat/service"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
)

type ConnectionController interface {
	DisconnectConnections(userUUID string, connectionIDs []string, reason string) int
	DisconnectAllConnections(userUUID string, reason string) int
}

// NewSessionKickHandler builds the Gateway handler for forced logout events.
func NewSessionKickHandler(controller ConnectionController) platformKafka.Handler {
	return func(_ context.Context, event platformKafka.Event) error {
		envelope, err := requireEnvelope(event)
		if err != nil {
			return fmt.Errorf("decode session kick envelope: %w", err)
		}

		payload, err := service.DecodeSessionKickEventPayload(envelope.EventType, envelope.Payload)
		if err != nil {
			return fmt.Errorf("decode session kick payload: %w", err)
		}
		if payload.All {
			controller.DisconnectAllConnections(payload.UserUUID, payload.Reason)
			return nil
		}

		controller.DisconnectConnections(payload.UserUUID, payload.ConnectionIDs, payload.Reason)
		return nil
	}
}
