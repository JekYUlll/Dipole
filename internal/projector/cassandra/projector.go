package cassandraprojector

import (
	"context"
	"fmt"
	"strings"

	"github.com/JekYUlll/Dipole/internal/compat/service"
	cassandradata "github.com/JekYUlll/Dipole/internal/data/cassandra"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
)

const (
	directCreatedEvent = "message.direct.created"
	groupCreatedEvent  = "message.group.created"
)

type timelineAppender interface {
	Append(context.Context, cassandradata.TimelineProjection) (cassandradata.AppendResult, error)
}

type Projector struct {
	timeline timelineAppender
}

func New(timeline timelineAppender) (*Projector, error) {
	if timeline == nil {
		return nil, fmt.Errorf("Cassandra Timeline appender is required")
	}
	return &Projector{timeline: timeline}, nil
}

func (p *Projector) Handler() platformKafka.Handler {
	return p.Project
}

func (p *Projector) Project(ctx context.Context, event platformKafka.Event) error {
	projection, err := projectionFromEvent(event)
	if err != nil {
		return err
	}
	if _, err := p.timeline.Append(ctx, projection); err != nil {
		return fmt.Errorf("project message %s to Cassandra: %w", projection.MessageUUID, err)
	}
	return nil
}

func projectionFromEvent(event platformKafka.Event) (cassandradata.TimelineProjection, error) {
	if event.DecodeErr != nil {
		return cassandradata.TimelineProjection{}, fmt.Errorf("decode Kafka envelope: %w", event.DecodeErr)
	}
	if event.Envelope == nil {
		return cassandradata.TimelineProjection{}, fmt.Errorf("Kafka envelope is required")
	}
	envelope := event.Envelope
	eventType := strings.TrimSpace(envelope.EventType)
	switch eventType {
	case directCreatedEvent:
	case groupCreatedEvent:
	default:
		return cassandradata.TimelineProjection{}, fmt.Errorf("unsupported Cassandra projection event type %q", eventType)
	}

	payload, err := service.DecodeMessageEventPayload(eventType, envelope.Payload)
	if err != nil {
		return cassandradata.TimelineProjection{}, fmt.Errorf("decode Cassandra projection payload: %w", err)
	}

	return cassandradata.TimelineProjection{
		EventID:         envelope.EventID,
		EventVersion:    envelope.Version,
		ConversationKey: payload.ConversationKey,
		MessageSeq:      payload.MessageSeq,
		MessageUUID:     payload.MessageID,
		ClientMessageID: payload.ClientMessageID,
		SenderUUID:      payload.SenderUUID,
		TargetType:      payload.TargetType,
		TargetUUID:      payload.TargetUUID,
		MessageType:     payload.MessageType,
		Content:         payload.Content,
		FileID:          payload.FileID,
		FileName:        payload.FileName,
		FileSize:        payload.FileSize,
		FileURL:         payload.FileURL,
		FileContentType: payload.FileContentType,
		FileExpiresAt:   payload.FileExpiresAt,
		SentAt:          payload.SentAt,
	}, nil
}
