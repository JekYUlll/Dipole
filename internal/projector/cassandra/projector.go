package cassandraprojector

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	cassandradata "github.com/JekYUlll/Dipole/internal/data/cassandra"
	"github.com/JekYUlll/Dipole/internal/model"
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

type createdMessagePayload struct {
	MutationType    string     `json:"mutation_type,omitempty"`
	Revision        uint64     `json:"revision,omitempty"`
	MessageID       string     `json:"message_id"`
	ClientMessageID string     `json:"client_message_id,omitempty"`
	ConversationKey string     `json:"conversation_key"`
	MessageSeq      uint64     `json:"message_seq"`
	SenderUUID      string     `json:"sender_uuid"`
	TargetUUID      string     `json:"target_uuid"`
	TargetType      int8       `json:"target_type"`
	MessageType     int8       `json:"message_type"`
	Content         string     `json:"content"`
	FileID          string     `json:"file_id,omitempty"`
	FileName        string     `json:"file_name,omitempty"`
	FileSize        int64      `json:"file_size,omitempty"`
	FileURL         string     `json:"file_url,omitempty"`
	FileContentType string     `json:"file_content_type,omitempty"`
	FileExpiresAt   *time.Time `json:"file_expires_at,omitempty"`
	SentAt          time.Time  `json:"sent_at"`
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
	expectedTargetType := int8(-1)
	switch eventType {
	case directCreatedEvent:
		expectedTargetType = model.MessageTargetDirect
	case groupCreatedEvent:
		expectedTargetType = model.MessageTargetGroup
	default:
		return cassandradata.TimelineProjection{}, fmt.Errorf("unsupported Cassandra projection event type %q", eventType)
	}

	var payload createdMessagePayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return cassandradata.TimelineProjection{}, fmt.Errorf("decode Cassandra projection payload: %w", err)
	}
	mutationType := strings.TrimSpace(payload.MutationType)
	if mutationType == "" {
		mutationType = "created"
	}
	if mutationType != "created" {
		return cassandradata.TimelineProjection{}, fmt.Errorf("unsupported Cassandra projection mutation %q", mutationType)
	}
	revision := payload.Revision
	if revision == 0 {
		revision = 1
	}
	if revision != 1 {
		return cassandradata.TimelineProjection{}, fmt.Errorf("created Cassandra projection revision must be 1, got %d", revision)
	}
	if payload.TargetType != expectedTargetType {
		return cassandradata.TimelineProjection{}, fmt.Errorf("event type %s conflicts with target type %d", eventType, payload.TargetType)
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
