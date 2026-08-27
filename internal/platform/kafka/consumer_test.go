package kafka

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/platform/correlation"
	kafkago "github.com/segmentio/kafka-go"
)

func TestHandleAllRestoresEnvelopeCorrelation(t *testing.T) {
	t.Parallel()
	consumer := &Consumer{}
	event := Event{Envelope: &Envelope{EventID: "E1", RequestID: "R1", TraceID: "T1"}}
	err := consumer.handleAll(context.Background(), event, []Handler{func(ctx context.Context, _ Event) error {
		if got := correlation.FromContext(ctx); got != (correlation.IDs{RequestID: "R1", TraceID: "T1", EventID: "E1"}) {
			t.Fatalf("unexpected handler correlation: %+v", got)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("handle event: %v", err)
	}
}

func TestConsumerStartDoesNotDeadlockWhenRegisteringReaders(t *testing.T) {
	t.Parallel()

	consumer := &Consumer{
		brokers:  []string{"127.0.0.1:9092"},
		handlers: make(map[string][]Handler),
		readers:  make(map[string]*kafkago.Reader),
	}

	consumer.Register("message.direct.created", func(context.Context, Event) error { return nil })
	consumer.Register("message.group.created", func(context.Context, Event) error { return nil })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = consumer.Start(ctx)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("consumer start timed out")
	}
}

func TestConsumerDoesNotInvokeHandlersForInvalidEnvelope(t *testing.T) {
	previousClient := Client
	Client = nil
	t.Cleanup(func() { Client = previousClient })

	consumer := &Consumer{maxAttempts: 3}
	handled := 0
	committed := consumer.handleWithRetry(context.Background(), Event{
		Topic:     "message.created",
		Value:     []byte(`{"version":"v2"}`),
		DecodeErr: ErrUnsupportedEventVersion,
	}, []Handler{func(context.Context, Event) error {
		handled++
		return nil
	}})
	if committed {
		t.Fatal("invalid envelope must stay uncommitted when DLQ publish is unavailable")
	}
	if handled != 0 {
		t.Fatalf("invalid envelope reached %d business handlers", handled)
	}
}

func TestRetryAndDeadTopicNames(t *testing.T) {
	t.Parallel()
	if got := retryTopicName("dipole.message.created"); got != "dipole.message.created.retry" {
		t.Fatalf("unexpected retry topic: %s", got)
	}
	if got := deadTopicName("dipole.message.created.retry"); got != "dipole.message.created.dead" {
		t.Fatalf("unexpected dead topic: %s", got)
	}
}

func TestDeadLetterHeadersPreserveSchemaDiagnostics(t *testing.T) {
	t.Parallel()
	consumer := &Consumer{topicPrefix: "dipole"}
	headers := consumer.deadLetterHeaders(Event{
		Topic: "dipole.message.created.retry",
		Headers: map[string]string{
			"schema_version": "v2",
			"retry_attempt":  "2",
		},
	}, errors.New("unsupported schema"), "invalid_envelope")
	if headers["schema_version"] != "v2" || headers["retry_attempt"] != "2" {
		t.Fatalf("source headers were not preserved: %+v", headers)
	}
	if headers["dead_reason"] != "invalid_envelope" || headers["original_topic"] != "message.created" || headers["last_error"] == "" || headers["failed_at"] == "" {
		t.Fatalf("missing dead-letter diagnostics: %+v", headers)
	}
}

func TestConsumerReaderConfigUsesExplicitRebalancePolicy(t *testing.T) {
	t.Parallel()

	consumer := &Consumer{
		clientID:          "dipole-message",
		groupID:           "dipole-message-consumer",
		brokers:           []string{"kafka-1:9092", "kafka-2:9092"},
		dialTimeout:       5 * time.Second,
		groupBalancer:     kafkago.RoundRobinGroupBalancer{},
		heartbeatInterval: 3 * time.Second,
		sessionTimeout:    30 * time.Second,
		rebalanceTimeout:  45 * time.Second,
	}
	config := consumer.readerConfig("dipole.message.direct.created")
	if config.GroupID != consumer.groupID || len(config.GroupBalancers) != 1 || config.GroupBalancers[0].ProtocolName() != "roundrobin" {
		t.Fatalf("unexpected group configuration: %+v", config)
	}
	if config.HeartbeatInterval != 3*time.Second || config.SessionTimeout != 30*time.Second || config.RebalanceTimeout != 45*time.Second {
		t.Fatalf("unexpected rebalance timing: %+v", config)
	}
	if config.Dialer == nil || config.Dialer.ClientID != "dipole-message" || config.Dialer.Timeout != 5*time.Second {
		t.Fatalf("unexpected consumer dialer: %+v", config.Dialer)
	}
	if config.StartOffset != kafkago.LastOffset {
		t.Fatalf("default consumer must start new groups at latest offset, got %d", config.StartOffset)
	}
}

func TestReplayableConsumerReaderStartsNewGroupFromBeginning(t *testing.T) {
	t.Parallel()

	consumer := &Consumer{startOffset: kafkago.FirstOffset}
	config := consumer.readerConfig("dipole.message.direct.created")
	if config.StartOffset != kafkago.FirstOffset {
		t.Fatalf("replayable consumer must start new groups at earliest retained offset, got %d", config.StartOffset)
	}
}

func TestNormalizeConsumerGroupPolicy(t *testing.T) {
	t.Parallel()

	balancer, heartbeat, session, rebalance, err := normalizeConsumerGroupPolicy("roundrobin", 3, 30, 45)
	if err != nil || balancer.ProtocolName() != "roundrobin" || heartbeat != 3*time.Second || session != 30*time.Second || rebalance != 45*time.Second {
		t.Fatalf("unexpected normalized policy: %T %v %v %v %v", balancer, heartbeat, session, rebalance, err)
	}
	if _, _, _, _, err := normalizeConsumerGroupPolicy("cooperative-sticky", 3, 30, 30); err == nil {
		t.Fatal("expected unsupported balancer to be rejected")
	}
	if _, _, _, _, err := normalizeConsumerGroupPolicy("range", 30, 10, 30); err == nil {
		t.Fatal("expected heartbeat >= session timeout to be rejected")
	}
}

func TestConsumerCollectStatsIncludesCumulativeOutcomes(t *testing.T) {
	t.Parallel()

	consumer := &Consumer{clientID: "core", groupID: "core-consumer", readers: make(map[string]*kafkago.Reader)}
	consumer.fetched.Add(7)
	consumer.handled.Add(6)
	consumer.committed.Add(5)
	consumer.commitErrors.Add(1)
	consumer.retryPublished.Add(2)
	consumer.deadPublished.Add(1)

	stats := consumer.CollectStats()
	if stats.ClientID != "core" || stats.GroupID != "core-consumer" || stats.Fetched != 7 || stats.Handled != 6 || stats.Committed != 5 || stats.CommitErrors != 1 || stats.RetryPublished != 2 || stats.DeadPublished != 1 {
		t.Fatalf("unexpected consumer stats: %+v", stats)
	}
}
