package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/JekYUlll/Dipole/internal/config"
)

var Subscriber *Consumer

const kafkaReaderMaxWait = 10 * time.Millisecond

type Event struct {
	Topic     string
	Key       []byte
	Value     []byte
	Headers   map[string]string
	Envelope  *Envelope
	DecodeErr error
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
	maxAttempts int
	backoff     time.Duration

	mu       sync.RWMutex
	handlers map[string][]Handler
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
		maxAttempts: normalizeRetryMaxAttempts(cfg.ConsumeRetryMaxAttempts),
		backoff:     normalizeRetryBackoff(cfg.ConsumeRetryBackoffMS),
		handlers:    make(map[string][]Handler),
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
	c.handlers[topic] = append(c.handlers[topic], handler)
	retryTopic := retryTopicName(topic)
	c.handlers[retryTopic] = append(c.handlers[retryTopic], handler)
}

func (c *Consumer) Start(ctx context.Context) error {
	if c == nil {
		return nil
	}

	c.mu.RLock()
	snapshots := make(map[string][]Handler, len(c.handlers))
	for topic, handlers := range c.handlers {
		copied := make([]Handler, len(handlers))
		copy(copied, handlers)
		snapshots[topic] = copied
	}
	c.mu.RUnlock()

	for topic, handlers := range snapshots {
		reader := c.readerForTopic(topic)
		go c.consumeLoop(ctx, reader, topic, handlers)
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

func (c *Consumer) consumeLoop(ctx context.Context, reader *kafkago.Reader, topic string, handlers []Handler) {
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
		envelope, decodeErr := decodeEnvelope(message.Value)
		if decodeErr == nil {
			event.Envelope = envelope
		} else {
			event.DecodeErr = decodeErr
		}

		if c.handleWithRetry(ctx, event, handlers) {
			_ = reader.CommitMessages(ctx, message)
		}
	}
}

func (c *Consumer) handleWithRetry(ctx context.Context, event Event, handlers []Handler) bool {
	if event.DecodeErr != nil {
		return c.publishDeadLetter(ctx, event, event.DecodeErr, "invalid_envelope")
	}

	attempts := c.maxAttempts
	if attempts <= 0 {
		attempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		lastErr = c.handleAll(ctx, event, handlers)
		if lastErr == nil {
			return true
		}
		if attempt < attempts {
			time.Sleep(c.backoff * time.Duration(attempt))
		}
	}

	if c.publishRetryOrDeadLetter(ctx, event, lastErr) {
		return true
	}

	return false
}

func (c *Consumer) handleAll(ctx context.Context, event Event, handlers []Handler) error {
	for _, handler := range handlers {
		if handler == nil {
			continue
		}
		if err := handler(ctx, event); err != nil {
			return err
		}
	}

	return nil
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
		MaxWait:     kafkaReaderMaxWait,
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

func decodeEnvelope(value []byte) (*Envelope, error) {
	var envelope Envelope
	if err := json.Unmarshal(value, &envelope); err != nil {
		return nil, fmt.Errorf("decode kafka event envelope: %w", err)
	}
	if err := validateEnvelope(&envelope); err != nil {
		return nil, err
	}

	return &envelope, nil
}

func (c *Consumer) publishRetryOrDeadLetter(ctx context.Context, event Event, lastErr error) bool {
	if Client == nil {
		return false
	}

	attempt := headerRetryAttempt(event.Headers)
	headers := cloneHeaders(event.Headers)
	headers["last_error"] = lastErr.Error()

	baseTopic := c.baseTopicName(event.Topic)
	if attempt+1 < c.maxAttempts {
		headers["retry_attempt"] = strconv.Itoa(attempt + 1)
		retryTopic := retryTopicName(baseTopic)
		return Client.Publish(ctx, retryTopic, Message{
			Key:     event.Key,
			Value:   event.Value,
			Headers: headers,
		}) == nil
	}

	return c.publishDeadLetter(ctx, event, lastErr, "handler_failed")
}

func (c *Consumer) publishDeadLetter(ctx context.Context, event Event, lastErr error, reason string) bool {
	if Client == nil {
		return false
	}
	headers := c.deadLetterHeaders(event, lastErr, reason)
	deadTopic := deadTopicName(c.baseTopicName(event.Topic))
	return Client.Publish(ctx, deadTopic, Message{
		Key:     event.Key,
		Value:   event.Value,
		Headers: headers,
	}) == nil
}

func (c *Consumer) deadLetterHeaders(event Event, lastErr error, reason string) map[string]string {
	headers := cloneHeaders(event.Headers)
	headers["retry_attempt"] = strconv.Itoa(headerRetryAttempt(event.Headers))
	headers["last_error"] = lastErr.Error()
	headers["dead_reason"] = reason
	headers["original_topic"] = c.baseTopicName(event.Topic)
	headers["failed_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	return headers
}

func (c *Consumer) baseTopicName(topic string) string {
	prefix := strings.TrimSpace(c.topicPrefix)
	if prefix != "" {
		prefix += "."
		if after, ok := strings.CutPrefix(topic, prefix); ok {
			topic = after
		}
	}
	topic = strings.TrimSuffix(topic, ".retry")
	return topic
}

func retryTopicName(topic string) string {
	if strings.HasSuffix(topic, ".retry") {
		return topic
	}
	return topic + ".retry"
}

func deadTopicName(topic string) string {
	topic = strings.TrimSuffix(topic, ".retry")
	return topic + ".dead"
}

func headerRetryAttempt(headers map[string]string) int {
	if headers == nil {
		return 0
	}
	raw := strings.TrimSpace(headers["retry_attempt"])
	if raw == "" {
		return 0
	}
	attempt, err := strconv.Atoi(raw)
	if err != nil || attempt < 0 {
		return 0
	}
	return attempt
}

func cloneHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(headers))
	for key, value := range headers {
		cloned[key] = value
	}
	return cloned
}

func normalizeRetryMaxAttempts(attempts int) int {
	if attempts <= 0 {
		return 3
	}
	return attempts
}

func normalizeRetryBackoff(backoffMS int) time.Duration {
	if backoffMS <= 0 {
		backoffMS = 500
	}
	return time.Duration(backoffMS) * time.Millisecond
}
