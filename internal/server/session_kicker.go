package server

import (
	"context"
	"fmt"
	"time"

	"github.com/JekYUlll/Dipole/internal/service"
	wsTransport "github.com/JekYUlll/Dipole/internal/transport/ws"
)

type sessionKickPublisher interface {
	PublishEvent(ctx context.Context, topic string, key string, eventType string, payload any, headers map[string]string) error
}

type sessionKicker struct {
	hub    *wsTransport.Hub
	events sessionKickPublisher
	async  bool
}

func newSessionKicker(hub *wsTransport.Hub, events sessionKickPublisher, async bool) *sessionKicker {
	return &sessionKicker{
		hub:    hub,
		events: events,
		async:  async,
	}
}

func (k *sessionKicker) KickConnections(userUUID string, connectionIDs []string) error {
	payload := service.SessionKickEventPayload{
		UserUUID:      userUUID,
		ConnectionIDs: connectionIDs,
		All:           false,
		Reason:        "forced_logout",
		OccurredAt:    time.Now().UTC(),
	}

	return k.dispatch(payload)
}

func (k *sessionKicker) KickAllConnections(userUUID string) error {
	payload := service.SessionKickEventPayload{
		UserUUID:   userUUID,
		All:        true,
		Reason:     "forced_logout_all",
		OccurredAt: time.Now().UTC(),
	}

	return k.dispatch(payload)
}

func (k *sessionKicker) dispatch(payload service.SessionKickEventPayload) error {
	if k == nil {
		return nil
	}

	if k.async && k.events != nil {
		if err := k.events.PublishEvent(
			context.Background(),
			"session.force_logout",
			payload.UserUUID,
			"session.force_logout",
			payload,
			nil,
		); err != nil {
			return fmt.Errorf("publish session kick event: %w", err)
		}
		return nil
	}

	if k.hub == nil {
		return nil
	}

	if payload.All {
		k.hub.DisconnectAllConnections(payload.UserUUID, payload.Reason)
		return nil
	}

	k.hub.DisconnectConnections(payload.UserUUID, payload.ConnectionIDs, payload.Reason)
	return nil
}
