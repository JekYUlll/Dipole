package kafka

import (
	"context"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/application"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	wsTransport "github.com/JekYUlll/Dipole/internal/transport/ws"
)

// NewAgentTaskWaitingHandler delivers a low-sensitivity locator. The client
// must re-read the task as its authenticated owner before acting on it.
func NewAgentTaskWaitingHandler(hub EventSender) platformKafka.Handler {
	return func(ctx context.Context, event platformKafka.Event) error {
		envelope, err := requireEnvelope(event)
		if err != nil {
			return fmt.Errorf("decode Agent Task waiting envelope: %w", err)
		}
		payload, err := application.DecodeAgentTaskWaitingNotificationV1(envelope.EventType, envelope.Payload)
		if err != nil {
			return err
		}
		sendEventToUser(ctx, hub, payload.PrincipalUUID, wsTransport.TypeAgentTaskWaiting, wsTransport.AgentTaskWaitingEventData{
			TaskUUID: payload.TaskUUID, PendingKind: payload.PendingKind, Revision: payload.Revision,
		})
		return nil
	}
}
