package service

import (
	"encoding/json"

	"github.com/JekYUlll/Dipole/internal/model"
	messagedomain "github.com/JekYUlll/Dipole/internal/services/message/domain"
)

// Message event symbols remain available here while legacy consumers migrate
// to the Message service domain package.
type MessageEventPayload = messagedomain.MessageEventPayload
type MessageMutationType = messagedomain.MessageMutationType

const (
	MessageMutationCreated  = messagedomain.MessageMutationCreated
	MessageMutationEdited   = messagedomain.MessageMutationEdited
	MessageMutationRecalled = messagedomain.MessageMutationRecalled
	MessageMutationDeleted  = messagedomain.MessageMutationDeleted
)

var (
	ErrMessageMutationTypeMismatch     = messagedomain.ErrMessageMutationTypeMismatch
	ErrMessageMutationRevisionRequired = messagedomain.ErrMessageMutationRevisionRequired
	ErrMessageMutationRevisionInvalid  = messagedomain.ErrMessageMutationRevisionInvalid
	ErrMessageMutationActorRequired    = messagedomain.ErrMessageMutationActorRequired
	ErrMessageEventChannelMismatch     = messagedomain.ErrMessageEventChannelMismatch
	ErrUnsupportedMessageEventType     = messagedomain.ErrUnsupportedMessageEventType
)

func DecodeMessageEventPayload(eventType string, raw json.RawMessage) (MessageEventPayload, error) {
	return messagedomain.DecodeMessageEventPayload(eventType, raw)
}

func MessageEventTargetType(eventType string) (int8, error) {
	return messagedomain.MessageEventTargetType(eventType)
}

func NormalizeMessageMutation(eventType string, payload *MessageEventPayload) error {
	return messagedomain.NormalizeMessageMutation(eventType, payload)
}

func MessageMutationEventType(targetType int8, mutation MessageMutationType) (string, error) {
	return messagedomain.MessageMutationEventType(targetType, mutation)
}

func MessageMutationAggregateID(messageID string, mutation MessageMutationType, revision uint64) (string, error) {
	return messagedomain.MessageMutationAggregateID(messageID, mutation, revision)
}

func MessageSearchMutation(eventType string, payload MessageEventPayload) (*model.MessageSearchMutation, error) {
	return messagedomain.MessageSearchMutation(eventType, payload)
}

func MessageSyncProjection(eventID, eventType string, payload MessageEventPayload) (*model.SyncProjection, bool, error) {
	return messagedomain.MessageSyncProjection(eventID, eventType, payload)
}
