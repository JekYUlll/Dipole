package corecontact

import (
	"encoding/json"
	"fmt"

	platformEvents "github.com/JekYUlll/Dipole/internal/platform/events"
)

func DecodeFriendDeletedPayload(eventType string, raw json.RawMessage) (ContactFriendDeletedPayload, error) {
	if err := platformEvents.RequireType(eventType, "contact.friend.deleted"); err != nil {
		return ContactFriendDeletedPayload{}, err
	}
	var payload ContactFriendDeletedPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ContactFriendDeletedPayload{}, fmt.Errorf("decode Contact event payload: %w", err)
	}
	return payload, nil
}
