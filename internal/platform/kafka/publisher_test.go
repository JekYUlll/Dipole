package kafka

import (
	"testing"
	"time"

	kafkago "github.com/segmentio/kafka-go"
)

func TestNormalizeBrokers(t *testing.T) {
	t.Parallel()

	brokers := normalizeBrokers([]string{
		" 127.0.0.1:9092 ",
		"",
		"127.0.0.1:9092",
		"127.0.0.1:9093",
	})

	if len(brokers) != 2 {
		t.Fatalf("expected 2 brokers, got %d", len(brokers))
	}
	if brokers[0] != "127.0.0.1:9092" {
		t.Fatalf("unexpected first broker: %s", brokers[0])
	}
	if brokers[1] != "127.0.0.1:9093" {
		t.Fatalf("unexpected second broker: %s", brokers[1])
	}
}

func TestPublisherTopicName(t *testing.T) {
	t.Parallel()

	publisher := &Publisher{
		topicPrefix: "dipole",
		timeout:     5 * time.Second,
	}

	if topic := publisher.topicName("message.created"); topic != "dipole.message.created" {
		t.Fatalf("unexpected topic: %s", topic)
	}
}

func TestNormalizeTopicPartitions(t *testing.T) {
	t.Parallel()

	if got := normalizeTopicPartitions(0); got != 1 {
		t.Fatalf("expected 1 partition fallback, got %d", got)
	}
	if got := normalizeTopicPartitions(6); got != 6 {
		t.Fatalf("expected 6 partitions, got %d", got)
	}
}

func TestWriterUsesHashBalancer(t *testing.T) {
	t.Parallel()

	publisher := &Publisher{
		topicPrefix: "dipole",
		timeout:     5 * time.Second,
		writers:     make(map[string]*kafkago.Writer),
	}

	writer := publisher.writerForTopic("dipole.message.direct.created")
	if _, ok := writer.Balancer.(*kafkago.Hash); !ok {
		t.Fatalf("expected kafka hash balancer, got %T", writer.Balancer)
	}
}
