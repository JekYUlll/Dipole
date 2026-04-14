package kafka

import (
	"context"
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
