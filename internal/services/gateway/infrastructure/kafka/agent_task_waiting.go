package kafka

import (
	"context"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/application"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	wsTransport "github.com/JekYUlll/Dipole/internal/transport/ws"
)

type AgentTaskWaitingObserver interface{ Observe(outcome string) }

// NewAgentTaskWaitingHandler delivers a low-sensitivity locator. The client
// must re-read the task as its authenticated owner before acting on it.
func NewAgentTaskWaitingHandler(hub EventSender, observers ...AgentTaskWaitingObserver) platformKafka.Handler {
	var observer AgentTaskWaitingObserver
	if len(observers) > 0 {
		observer = observers[0]
	}
	return func(ctx context.Context, event platformKafka.Event) error {
		envelope, err := requireEnvelope(event)
		if err != nil {
			observeAgentTaskWaiting(observer, "invalid")
			return fmt.Errorf("decode Agent Task waiting envelope: %w", err)
		}
		payload, err := application.DecodeAgentTaskWaitingNotificationV1(envelope.EventType, envelope.Payload)
		if err != nil {
			observeAgentTaskWaiting(observer, "invalid")
			return err
		}
		if sent := sendEventToUser(ctx, hub, payload.PrincipalUUID, wsTransport.TypeAgentTaskWaiting, wsTransport.AgentTaskWaitingEventData{
			TaskUUID: payload.TaskUUID, PendingKind: payload.PendingKind, Revision: payload.Revision,
		}); sent > 0 {
			observeAgentTaskWaiting(observer, "online")
		} else {
			observeAgentTaskWaiting(observer, "offline")
		}
		return nil
	}
}

func observeAgentTaskWaiting(observer AgentTaskWaitingObserver, outcome string) {
	if observer != nil {
		observer.Observe(outcome)
	}
}
