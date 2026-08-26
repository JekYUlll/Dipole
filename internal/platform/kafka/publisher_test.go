package kafka

import (
	"strings"
	"testing"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/JekYUlll/Dipole/internal/config"
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
		topicPrefix:  "dipole",
		timeout:      5 * time.Second,
		requiredAcks: kafkago.RequireAll,
		writers:      make(map[string]*kafkago.Writer),
	}

	writer := publisher.writerForTopic("dipole.message.direct.created")
	if _, ok := writer.Balancer.(*kafkago.Hash); !ok {
		t.Fatalf("expected kafka hash balancer, got %T", writer.Balancer)
	}
	if writer.RequiredAcks != kafkago.RequireAll {
		t.Fatalf("expected all replicas acknowledgement, got %v", writer.RequiredAcks)
	}
}

func TestNormalizeRequiredAcks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value string
		want  kafkago.RequiredAcks
	}{
		{"", kafkago.RequireOne},
		{"one", kafkago.RequireOne},
		{"all", kafkago.RequireAll},
		{"-1", kafkago.RequireAll},
	}
	for _, test := range tests {
		got, err := normalizeRequiredAcks(test.value)
		if err != nil || got != test.want {
			t.Fatalf("required acks %q = %v, %v; want %v", test.value, got, err, test.want)
		}
	}
	if _, err := normalizeRequiredAcks("none"); err == nil {
		t.Fatal("expected unsafe no-ack mode to be rejected")
	}
}

func TestTopicConfigEntriesAreExplicit(t *testing.T) {
	t.Parallel()

	entries := topicConfigEntries(2, 24*time.Hour)
	got := make(map[string]string, len(entries))
	for _, entry := range entries {
		got[entry.ConfigName] = entry.ConfigValue
	}
	if got["min.insync.replicas"] != "2" || got["retention.ms"] != "86400000" {
		t.Fatalf("unexpected topic config entries: %+v", got)
	}
}

func TestPublisherRejectsImpossibleDurabilityBeforeDial(t *testing.T) {
	t.Parallel()

	_, err := newPublisher(config.Kafka{
		Brokers:                []string{"unreachable.invalid:9092"},
		TopicReplicationFactor: 1,
		TopicMinInSyncReplicas: 2,
		RequiredAcks:           "all",
	})
	if err == nil || !strings.Contains(err.Error(), "min ISR 2 exceeds replication factor 1") {
		t.Fatalf("expected durability configuration error, got %v", err)
	}
}
