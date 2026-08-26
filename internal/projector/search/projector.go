package searchprojector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	"github.com/JekYUlll/Dipole/internal/service"
)

var topics = []string{
	"message.direct.created", "message.direct.edited", "message.direct.recalled", "message.direct.deleted",
	"message.group.created", "message.group.edited", "message.group.recalled", "message.group.deleted",
}

type Projector struct{ index application.SearchIndex }

func New(index application.SearchIndex) (*Projector, error) {
	if index == nil {
		return nil, errors.New("Search index is required")
	}
	return &Projector{index: index}, nil
}

func Topics() []string { return append([]string(nil), topics...) }

func (p *Projector) Handler() platformKafka.Handler { return p.Project }

func (p *Projector) Project(_ context.Context, event platformKafka.Event) error {
	mutation, err := mutationFromEvent(event)
	if err != nil {
		return err
	}
	if err := p.index.Apply(mutation); err != nil {
		return fmt.Errorf("apply Search mutation %s@r%d: %w", mutation.MessageUUID, mutation.Revision, err)
	}
	return nil
}

func mutationFromEvent(event platformKafka.Event) (*model.MessageSearchMutation, error) {
	if event.DecodeErr != nil {
		return nil, fmt.Errorf("decode Kafka envelope: %w", event.DecodeErr)
	}
	if event.Envelope == nil {
		return nil, errors.New("Kafka envelope is required")
	}
	var payload service.MessageEventPayload
	if err := json.Unmarshal(event.Envelope.Payload, &payload); err != nil {
		return nil, fmt.Errorf("decode Search projection payload: %w", err)
	}
	if err := service.NormalizeMessageMutation(event.Envelope.EventType, &payload); err != nil {
		return nil, fmt.Errorf("normalize Search mutation: %w", err)
	}
	parts := strings.Split(strings.TrimSpace(event.Envelope.EventType), ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("unsupported Search event type %q", event.Envelope.EventType)
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
	case service.MessageMutationCreated, service.MessageMutationEdited:
		mutation.Type = model.MessageSearchMutationUpsert
		mutation.Document = &model.MessageSearchDocument{
			MessageUUID: payload.MessageID, ConversationKey: payload.ConversationKey, MessageSeq: payload.MessageSeq,
			Revision: payload.Revision, SenderUUID: payload.SenderUUID, MessageType: payload.MessageType,
			Content: payload.Content, SentAt: payload.SentAt,
		}
	case service.MessageMutationRecalled, service.MessageMutationDeleted:
		mutation.Type = model.MessageSearchMutationTombstone
	default:
		return nil, fmt.Errorf("unsupported Search mutation %q", payload.MutationType)
	}
	if _, err := mutation.State(); err != nil {
		return nil, fmt.Errorf("validate Search mutation: %w", err)
	}
	return mutation, nil
}
