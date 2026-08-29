package application

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
	platformEvents "github.com/JekYUlll/Dipole/internal/platform/events"
)

// GroupEventPayload is the versioned payload consumed by services interested
// in group lifecycle events.
type GroupEventPayload struct {
	GroupUUID      string    `json:"group_uuid"`
	GroupName      string    `json:"group_name,omitempty"`
	Name           string    `json:"name,omitempty"`
	Notice         string    `json:"notice,omitempty"`
	Avatar         string    `json:"avatar,omitempty"`
	OperatorUUID   string    `json:"operator_uuid,omitempty"`
	MemberUUIDs    []string  `json:"member_uuids,omitempty"`
	RecipientUUIDs []string  `json:"recipient_uuids,omitempty"`
	OccurredAt     time.Time `json:"occurred_at"`
}

func DecodeGroupEventPayload(eventType string, raw json.RawMessage) (GroupEventPayload, error) {
	if err := platformEvents.RequireType(eventType,
		"group.created", "group.updated", "group.members.added", "group.members.removed", "group.dismissed",
	); err != nil {
		return GroupEventPayload{}, err
	}
	var payload GroupEventPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return GroupEventPayload{}, fmt.Errorf("decode group event payload: %w", err)
	}
	return payload, nil
}

type SessionKickEventPayload struct {
	UserUUID      string    `json:"user_uuid"`
	ConnectionIDs []string  `json:"connection_ids,omitempty"`
	All           bool      `json:"all"`
	Reason        string    `json:"reason"`
	OccurredAt    time.Time `json:"occurred_at"`
}

func DecodeSessionKickEventPayload(eventType string, raw json.RawMessage) (SessionKickEventPayload, error) {
	if err := platformEvents.RequireType(eventType, "session.force_logout"); err != nil {
		return SessionKickEventPayload{}, err
	}
	var payload SessionKickEventPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return SessionKickEventPayload{}, fmt.Errorf("decode session event payload: %w", err)
	}
	return payload, nil
}

type ContactFriendDeletedPayload struct {
	UserUUID   string    `json:"user_uuid"`
	FriendUUID string    `json:"friend_uuid"`
	OccurredAt time.Time `json:"occurred_at"`
}

func DecodeContactFriendDeletedPayload(eventType string, raw json.RawMessage) (ContactFriendDeletedPayload, error) {
	if err := platformEvents.RequireType(eventType, "contact.friend.deleted"); err != nil {
		return ContactFriendDeletedPayload{}, err
	}
	var payload ContactFriendDeletedPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ContactFriendDeletedPayload{}, fmt.Errorf("decode contact event payload: %w", err)
	}
	return payload, nil
}

type ConversationReadReceipt struct {
	ReaderUUID          string    `json:"reader_uuid"`
	TargetUUID          string    `json:"target_uuid"`
	TargetType          int8      `json:"target_type"`
	ConversationKey     string    `json:"conversation_key"`
	LastReadMessageUUID string    `json:"last_read_message_uuid"`
	LastReadSeq         uint64    `json:"last_read_seq"`
	ReadAt              time.Time `json:"read_at"`
}

var ErrConversationReadReceiptTargetMismatch = errors.New("conversation read event must target a direct conversation")

func DecodeConversationReadReceipt(eventType string, raw json.RawMessage) (ConversationReadReceipt, error) {
	if err := platformEvents.RequireType(eventType, "conversation.direct.read"); err != nil {
		return ConversationReadReceipt{}, err
	}
	var payload ConversationReadReceipt
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ConversationReadReceipt{}, fmt.Errorf("decode conversation read payload: %w", err)
	}
	if payload.TargetType != model.MessageTargetDirect {
		return ConversationReadReceipt{}, fmt.Errorf("%w: target_type=%d", ErrConversationReadReceiptTargetMismatch, payload.TargetType)
	}
	return payload, nil
}
