package coregroup

import (
	"encoding/json"
	"fmt"

	platformEvents "github.com/JekYUlll/Dipole/internal/platform/events"
)

func DecodeEventPayload(eventType string, raw json.RawMessage) (GroupEventPayload, error) {
	if err := platformEvents.RequireType(eventType,
		"group.created", "group.updated", "group.members.added", "group.members.removed", "group.dismissed",
	); err != nil {
		return GroupEventPayload{}, err
	}
	var payload GroupEventPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return GroupEventPayload{}, fmt.Errorf("decode Group event payload: %w", err)
	}
	return payload, nil
}
