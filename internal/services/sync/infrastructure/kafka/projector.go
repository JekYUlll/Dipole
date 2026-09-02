package syncprojector

import (
	"context"
	"errors"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	platformkafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	messagedomain "github.com/JekYUlll/Dipole/internal/services/message/domain"
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
	payload, err := messagedomain.DecodeMessageEventPayload(event.Envelope.EventType, event.Envelope.Payload)
	if err != nil {
		return nil, false, fmt.Errorf("decode Sync projection payload: %w", err)
	}
	return messagedomain.MessageSyncProjection(event.Envelope.EventID, event.Envelope.EventType, payload)
}
