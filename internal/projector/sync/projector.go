package syncprojector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	platformkafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	"github.com/JekYUlll/Dipole/internal/service"
)

var topics = []string{"message.direct.created", "message.group.created"}

type Projector struct {
	store application.SyncProjectionStore
}

func New(store application.SyncProjectionStore) (*Projector, error) {
	if store == nil {
		return nil, errors.New("Sync projection store is required")
	}
	return &Projector{store: store}, nil
}

func Topics() []string { return append([]string(nil), topics...) }

func (p *Projector) Handler() platformkafka.Handler { return p.Project }

func (p *Projector) Project(_ context.Context, event platformkafka.Event) error {
	projection, fanout, err := projectionFromEvent(event)
	if err != nil {
		return err
	}
	if !fanout {
		return nil
	}
	if err := p.store.Apply(projection); err != nil {
		return fmt.Errorf("apply Sync projection %s: %w", projection.MessageUUID, err)
	}
	return nil
}

func projectionFromEvent(event platformkafka.Event) (*model.SyncProjection, bool, error) {
	if event.DecodeErr != nil {
		return nil, false, fmt.Errorf("decode Kafka envelope: %w", event.DecodeErr)
	}
	if event.Envelope == nil {
		return nil, false, errors.New("Kafka envelope is required")
	}
	expectedTarget := int8(-1)
	switch event.Envelope.EventType {
	case topics[0]:
		expectedTarget = model.MessageTargetDirect
	case topics[1]:
		expectedTarget = model.MessageTargetGroup
	default:
		return nil, false, fmt.Errorf("unsupported Sync projection event type %q", event.Envelope.EventType)
	}
	var payload service.MessageEventPayload
	if err := json.Unmarshal(event.Envelope.Payload, &payload); err != nil {
		return nil, false, fmt.Errorf("decode Sync projection payload: %w", err)
	}
	if payload.TargetType != expectedTarget {
		return nil, false, fmt.Errorf("Sync event target type %d conflicts with %s", payload.TargetType, event.Envelope.EventType)
	}
	if strings.TrimSpace(payload.MessageID) == "" || strings.TrimSpace(payload.ConversationKey) == "" || payload.MessageSeq == 0 {
		return nil, false, errors.New("Sync projection requires message_id, conversation_key, and message_seq")
	}
	fanout := payload.SyncFanout == nil || *payload.SyncFanout
	if !fanout {
		return nil, false, nil
	}
	recipients := append([]string(nil), payload.RecipientUUIDs...)
	if len(recipients) == 0 && expectedTarget == model.MessageTargetDirect {
		recipients = []string{payload.SenderUUID, payload.TargetUUID}
	}
	if len(recipients) == 0 {
		return nil, false, errors.New("Sync group projection requires recipient snapshot")
	}
	return &model.SyncProjection{
		EventID: event.Envelope.EventID, MessageUUID: strings.TrimSpace(payload.MessageID),
		ConversationKey: strings.TrimSpace(payload.ConversationKey), MessageSeq: payload.MessageSeq,
		RecipientUUIDs: recipients,
	}, true, nil
}
