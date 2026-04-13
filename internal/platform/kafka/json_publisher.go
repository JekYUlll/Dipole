package kafka

import (
	"context"
	"encoding/json"
	"fmt"
)

type JSONPublisher struct {
	publisher *Publisher
}

func NewJSONPublisher(publisher *Publisher) *JSONPublisher {
	return &JSONPublisher{publisher: publisher}
}

func (p *JSONPublisher) PublishJSON(ctx context.Context, topic string, key string, payload any, headers map[string]string) error {
	if p == nil || p.publisher == nil {
		return nil
	}

	value, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal kafka payload for %s: %w", topic, err)
	}

	message := Message{
		Value:   value,
		Headers: headers,
	}
	if key != "" {
		message.Key = []byte(key)
	}

	if err := p.publisher.Publish(ctx, topic, message); err != nil {
		return fmt.Errorf("publish kafka json message to %s: %w", topic, err)
	}

	return nil
}

func (p *JSONPublisher) PublishEvent(ctx context.Context, topic string, key string, eventType string, payload any, headers map[string]string) error {
	envelope, err := NewEnvelope(eventType, payload)
	if err != nil {
		return fmt.Errorf("create kafka event envelope for %s: %w", topic, err)
	}

	if headers == nil {
		headers = map[string]string{}
	}
	headers["event_type"] = envelope.EventType
	headers["version"] = envelope.Version
	headers["source"] = envelope.Source
	headers["event_id"] = envelope.EventID

	return p.PublishJSON(ctx, topic, key, envelope, headers)
}
