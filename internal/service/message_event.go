package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/JekYUlll/Dipole/internal/model"
)

type MessageMutationType string

const (
	MessageMutationCreated  MessageMutationType = "created"
	MessageMutationEdited   MessageMutationType = "edited"
	MessageMutationRecalled MessageMutationType = "recalled"
	MessageMutationDeleted  MessageMutationType = "deleted"
)

var (
	ErrMessageMutationTypeMismatch     = errors.New("message mutation type does not match event type")
	ErrMessageMutationRevisionRequired = errors.New("message mutation revision is required")
	ErrMessageMutationRevisionInvalid  = errors.New("message mutation revision is invalid")
	ErrMessageMutationActorRequired    = errors.New("message mutation actor is required")
	ErrMessageEventChannelMismatch     = errors.New("message event channel does not match target type")
	ErrUnsupportedMessageEventType     = errors.New("unsupported message event type")
)

// DecodeMessageEventPayload is the shared v1 consumer boundary. Unknown JSON
// fields remain accepted so minor schema additions can roll out producer-first.
func DecodeMessageEventPayload(eventType string, raw json.RawMessage) (MessageEventPayload, error) {
	expectedTarget, err := MessageEventTargetType(eventType)
	if err != nil {
		return MessageEventPayload{}, err
	}

	var payload MessageEventPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return MessageEventPayload{}, fmt.Errorf("decode message event payload: %w", err)
	}
	if _, mutation := messageMutationTypeFromEvent(eventType); mutation {
		if err := NormalizeMessageMutation(eventType, &payload); err != nil {
			return MessageEventPayload{}, err
		}
	}
	if payload.TargetType != expectedTarget {
		return MessageEventPayload{}, fmt.Errorf("%w: event=%s target_type=%d", ErrMessageEventChannelMismatch, eventType, payload.TargetType)
	}
	return payload, nil
}

func MessageEventTargetType(eventType string) (int8, error) {
	parts := strings.Split(strings.TrimSpace(eventType), ".")
	if len(parts) != 3 || parts[0] != "message" || (parts[1] != "direct" && parts[1] != "group") {
		return 0, fmt.Errorf("%w: %q", ErrUnsupportedMessageEventType, eventType)
	}
	if _, mutation := messageMutationTypeFromEvent(eventType); !mutation && parts[2] != "send_requested" {
		return 0, fmt.Errorf("%w: %q", ErrUnsupportedMessageEventType, eventType)
	}
	if parts[1] == "direct" {
		return model.MessageTargetDirect, nil
	}
	return model.MessageTargetGroup, nil
}

// NormalizeMessageMutation keeps legacy created events readable while making
// future mutation facts carry an explicit type, revision, and actor.
func NormalizeMessageMutation(eventType string, payload *MessageEventPayload) error {
	expected, ok := messageMutationTypeFromEvent(eventType)
	if !ok || payload == nil {
		return nil
	}

	if payload.MutationType == "" && expected == MessageMutationCreated {
		payload.MutationType = MessageMutationCreated
	}
	if payload.MutationType != expected {
		return fmt.Errorf("%w: event=%s payload=%s", ErrMessageMutationTypeMismatch, expected, payload.MutationType)
	}

	if payload.Revision == 0 && expected == MessageMutationCreated {
		payload.Revision = 1
	}
	if payload.Revision == 0 {
		return ErrMessageMutationRevisionRequired
	}
	if expected == MessageMutationCreated && payload.Revision != 1 {
		return fmt.Errorf("%w: created revision must be 1", ErrMessageMutationRevisionInvalid)
	}

	if strings.TrimSpace(payload.ActorUUID) == "" && expected == MessageMutationCreated {
		payload.ActorUUID = strings.TrimSpace(payload.SenderUUID)
	}
	if strings.TrimSpace(payload.ActorUUID) == "" {
		return ErrMessageMutationActorRequired
	}
	return nil
}

func MessageMutationEventType(targetType int8, mutation MessageMutationType) (string, error) {
	channel := ""
	switch targetType {
	case model.MessageTargetDirect:
		channel = "direct"
	case model.MessageTargetGroup:
		channel = "group"
	default:
		return "", fmt.Errorf("unsupported message target type %d", targetType)
	}

	switch mutation {
	case MessageMutationCreated, MessageMutationEdited, MessageMutationRecalled, MessageMutationDeleted:
		return "message." + channel + "." + string(mutation), nil
	default:
		return "", fmt.Errorf("unsupported message mutation type %q", mutation)
	}
}

func MessageMutationAggregateID(messageID string, mutation MessageMutationType, revision uint64) (string, error) {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return "", errors.New("message mutation message id is required")
	}
	if mutation == MessageMutationCreated && revision == 1 {
		return messageID, nil
	}
	if revision == 0 {
		return "", ErrMessageMutationRevisionRequired
	}
	switch mutation {
	case MessageMutationEdited, MessageMutationRecalled, MessageMutationDeleted:
		return fmt.Sprintf("%s@r%d", messageID, revision), nil
	default:
		return "", fmt.Errorf("unsupported message mutation type %q", mutation)
	}
}

func MessageSearchMutation(eventType string, payload MessageEventPayload) (*model.MessageSearchMutation, error) {
	if err := NormalizeMessageMutation(eventType, &payload); err != nil {
		return nil, fmt.Errorf("normalize Search mutation: %w", err)
	}
	parts := strings.Split(strings.TrimSpace(eventType), ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("unsupported Search event type %q", eventType)
	}
	expectedTarget := int8(-1)
	switch parts[1] {
	case "direct":
		expectedTarget = model.MessageTargetDirect
	case "group":
		expectedTarget = model.MessageTargetGroup
	default:
		return nil, fmt.Errorf("unsupported Search event channel %q", parts[1])
	}
	if payload.TargetType != expectedTarget {
		return nil, fmt.Errorf("Search event channel %s conflicts with target type %d", parts[1], payload.TargetType)
	}
	mutation := &model.MessageSearchMutation{MessageUUID: payload.MessageID, Revision: payload.Revision}
	switch payload.MutationType {
	case MessageMutationCreated, MessageMutationEdited:
		mutation.Type = model.MessageSearchMutationUpsert
		mutation.Document = &model.MessageSearchDocument{
			MessageUUID: payload.MessageID, ConversationKey: payload.ConversationKey, MessageSeq: payload.MessageSeq,
			Revision: payload.Revision, SenderUUID: payload.SenderUUID, MessageType: payload.MessageType,
			Content: payload.Content, SentAt: payload.SentAt,
		}
	case MessageMutationRecalled, MessageMutationDeleted:
		mutation.Type = model.MessageSearchMutationTombstone
	default:
		return nil, fmt.Errorf("unsupported Search mutation %q", payload.MutationType)
	}
	if _, err := mutation.State(); err != nil {
		return nil, fmt.Errorf("validate Search mutation: %w", err)
	}
	return mutation, nil
}

func messageMutationTypeFromEvent(eventType string) (MessageMutationType, bool) {
	parts := strings.Split(strings.TrimSpace(eventType), ".")
	if len(parts) != 3 || parts[0] != "message" || (parts[1] != "direct" && parts[1] != "group") {
		return "", false
	}
	mutation := MessageMutationType(parts[2])
	switch mutation {
	case MessageMutationCreated, MessageMutationEdited, MessageMutationRecalled, MessageMutationDeleted:
		return mutation, true
	default:
		return "", false
	}
}
