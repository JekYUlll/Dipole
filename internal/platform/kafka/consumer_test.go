package kafka

import (
	"context"
	"errors"
	"testing"
	"time"

	kafkago "github.com/segmentio/kafka-go"
)

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
