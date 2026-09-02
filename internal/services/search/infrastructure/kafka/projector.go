package searchprojector

import (
	"context"
	"errors"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	messagedomain "github.com/JekYUlll/Dipole/internal/services/message/domain"
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
	payload, err := messagedomain.DecodeMessageEventPayload(event.Envelope.EventType, event.Envelope.Payload)
	if err != nil {
		return nil, fmt.Errorf("decode Search projection payload: %w", err)
	}
	return messagedomain.MessageSearchMutation(event.Envelope.EventType, payload)
}
