package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/JekYUlll/Dipole/internal/model"
	coresession "github.com/JekYUlll/Dipole/internal/services/core/domain/session"
)

var (
	ErrUnsupportedDomainEventType     = errors.New("unsupported domain event type")
	ErrConversationReadTargetMismatch = errors.New("conversation read event must target a direct conversation")
)

func DecodeGroupEventPayload(eventType string, raw json.RawMessage) (GroupEventPayload, error) {
	if err := requireDomainEventType(eventType,
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

func DecodeConversationReadReceipt(eventType string, raw json.RawMessage) (ConversationReadReceipt, error) {
	if err := requireDomainEventType(eventType, "conversation.direct.read"); err != nil {
		return ConversationReadReceipt{}, err
	}
	var payload ConversationReadReceipt
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ConversationReadReceipt{}, fmt.Errorf("decode Conversation read payload: %w", err)
	}
	if payload.TargetType != model.MessageTargetDirect {
		return ConversationReadReceipt{}, fmt.Errorf("%w: target_type=%d", ErrConversationReadTargetMismatch, payload.TargetType)
	}
	return payload, nil
}

func DecodeContactFriendDeletedPayload(eventType string, raw json.RawMessage) (ContactFriendDeletedPayload, error) {
	if err := requireDomainEventType(eventType, "contact.friend.deleted"); err != nil {
		return ContactFriendDeletedPayload{}, err
	}
	var payload ContactFriendDeletedPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ContactFriendDeletedPayload{}, fmt.Errorf("decode Contact event payload: %w", err)
	}
	return payload, nil
}

func DecodeSessionKickEventPayload(eventType string, raw json.RawMessage) (coresession.SessionKickEventPayload, error) {
	if err := requireDomainEventType(eventType, "session.force_logout"); err != nil {
		return coresession.SessionKickEventPayload{}, err
	}
	var payload coresession.SessionKickEventPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return coresession.SessionKickEventPayload{}, fmt.Errorf("decode Session event payload: %w", err)
	}
	return payload, nil
}

func requireDomainEventType(actual string, allowed ...string) error {
	actual = strings.TrimSpace(actual)
	for _, eventType := range allowed {
		if actual == eventType {
			return nil
		}
	}
	return fmt.Errorf("%w: %q", ErrUnsupportedDomainEventType, actual)
}
