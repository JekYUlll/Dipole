package kafka

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/JekYUlll/Dipole/internal/config"
)

var Subscriber *Consumer

type Event struct {
	Topic     string
	Key       []byte
	Value     []byte
	Headers   map[string]string
	Partition int
	Offset    int64
	Time      time.Time
}

type Handler func(context.Context, Event) error

type Consumer struct {
	clientID    string
	groupID     string
	topicPrefix string
	brokers     []string

	mu       sync.RWMutex
	handlers map[string]Handler
	readers  map[string]*kafkago.Reader
}

func InitConsumer() error {
	cfg := config.KafkaConfig()
	if !cfg.Enabled {
		Subscriber = nil
		return nil
	}

	consumer, err := newConsumer(cfg)
	if err != nil {
		return err
	}

	Subscriber = consumer
	return nil
}

func CloseConsumer() error {
	if Subscriber == nil {
		return nil
	}

	err := Subscriber.Close()
	Subscriber = nil
	return err
}

func newConsumer(cfg config.Kafka) (*Consumer, error) {
	brokers := normalizeBrokers(cfg.Brokers)
	if len(brokers) == 0 {
		return nil, fmt.Errorf("kafka brokers are empty")
	}

	timeout := time.Duration(cfg.DialTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if err := pingBroker(brokers[0], timeout); err != nil {
		return nil, err
	}

	clientID := strings.TrimSpace(cfg.ClientID)
	if clientID == "" {
		clientID = "dipole"
	}

	return &Consumer{
		clientID:    clientID,
		groupID:     clientID + "-consumer",
		topicPrefix: strings.TrimSpace(cfg.TopicPrefix),
		brokers:     brokers,
		handlers:    make(map[string]Handler),
		readers:     make(map[string]*kafkago.Reader),
	}, nil
}

func (c *Consumer) Register(topic string, handler Handler) {
	if c == nil || handler == nil {
		return
	}

	topic = c.topicName(topic)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers[topic] = handler
}

func (c *Consumer) Start(ctx context.Context) error {
	if c == nil {
		return nil
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	for topic, handler := range c.handlers {
		reader := c.readerForTopic(topic)
		go c.consumeLoop(ctx, reader, topic, handler)
	}

	return nil
}

func (c *Consumer) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var errs []error
	for topic, reader := range c.readers {
		if err := reader.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close kafka reader %s: %w", topic, err))
		}
		delete(c.readers, topic)
	}

	return errors.Join(errs...)
}

func (c *Consumer) consumeLoop(ctx context.Context, reader *kafkago.Reader, topic string, handler Handler) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		message, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, context.Canceled) {
				return
			}
			time.Sleep(500 * time.Millisecond)
			continue
		}

		event := Event{
			Topic:     topic,
			Key:       message.Key,
			Value:     message.Value,
			Headers:   decodeHeaders(message.Headers),
			Partition: message.Partition,
			Offset:    message.Offset,
			Time:      message.Time,
		}

		if err := handler(ctx, event); err == nil {
			_ = reader.CommitMessages(ctx, message)
		}
	}
}

func (c *Consumer) readerForTopic(topic string) *kafkago.Reader {
	c.mu.Lock()
	defer c.mu.Unlock()

	if reader, ok := c.readers[topic]; ok {
		return reader
	}

	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:     c.brokers,
		GroupID:     c.groupID,
		Topic:       topic,
		StartOffset: kafkago.LastOffset,
		MaxWait:     500 * time.Millisecond,
	})
	c.readers[topic] = reader
	return reader
}

func (c *Consumer) topicName(topic string) string {
	topic = strings.TrimSpace(topic)
	if c.topicPrefix == "" {
		return topic
	}
	if topic == "" {
		return c.topicPrefix
	}

	return c.topicPrefix + "." + topic
}

func decodeHeaders(headers []kafkago.Header) map[string]string {
	if len(headers) == 0 {
		return nil
	}

	decoded := make(map[string]string, len(headers))
	for _, header := range headers {
		decoded[header.Key] = string(header.Value)
	}

	return decoded
}
