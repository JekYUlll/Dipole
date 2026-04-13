package kafka

import (
	"encoding/json"
	"testing"

	kafkago "github.com/segmentio/kafka-go"
)

func TestConsumerTopicName(t *testing.T) {
	t.Parallel()

	consumer := &Consumer{
		topicPrefix: "dipole",
	}

	if topic := consumer.topicName("group.created"); topic != "dipole.group.created" {
		t.Fatalf("unexpected topic: %s", topic)
	}
}

func TestDecodeHeaders(t *testing.T) {
	t.Parallel()

	headers := decodeHeaders([]kafkago.Header{
		{Key: "event_type", Value: []byte("message.direct.created")},
		{Key: "version", Value: []byte("v1")},
	})

	if headers["event_type"] != "message.direct.created" {
		t.Fatalf("unexpected event_type header: %s", headers["event_type"])
	}
	if headers["version"] != "v1" {
		t.Fatalf("unexpected version header: %s", headers["version"])
	}
}

func TestNewEnvelopeAndDecode(t *testing.T) {
	t.Parallel()

	envelope, err := NewEnvelope("message.direct.created", map[string]any{
		"message_id": "M100",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if envelope.EventType != "message.direct.created" {
		t.Fatalf("unexpected event type: %s", envelope.EventType)
	}

	raw, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	decoded, err := decodeEnvelope(raw)
	if err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if decoded.EventID != envelope.EventID {
		t.Fatalf("expected event id %s, got %s", envelope.EventID, decoded.EventID)
	}
}
