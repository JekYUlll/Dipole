package coresession

import (
	"encoding/json"
	"fmt"

	platformEvents "github.com/JekYUlll/Dipole/internal/platform/events"
)

func DecodeKickEventPayload(eventType string, raw json.RawMessage) (SessionKickEventPayload, error) {
	if err := platformEvents.RequireType(eventType, "session.force_logout"); err != nil {
		return SessionKickEventPayload{}, err
	}
	var payload SessionKickEventPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return SessionKickEventPayload{}, fmt.Errorf("decode Session event payload: %w", err)
	}
	return payload, nil
}
