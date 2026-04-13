package kafka

import (
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
