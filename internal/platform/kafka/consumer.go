package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/platform/correlation"
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
	clientID          string
	groupID           string
	topicPrefix       string
	brokers           []string
	dialTimeout       time.Duration
	maxAttempts       int
	backoff           time.Duration
	groupBalancer     kafkago.GroupBalancer
	heartbeatInterval time.Duration
	sessionTimeout    time.Duration
	rebalanceTimeout  time.Duration
	startOffset       int64
	failurePublisher  *Publisher

	fetched        atomic.Uint64
	handled        atomic.Uint64
	committed      atomic.Uint64
	fetchErrors    atomic.Uint64
	commitErrors   atomic.Uint64
	retryPublished atomic.Uint64
	deadPublished  atomic.Uint64

	mu       sync.RWMutex
	handlers map[string][]Handler
	readers  map[string]*kafkago.Reader
}

type ConsumerStats struct {
	ClientID       string
	GroupID        string
	Fetched        uint64
	Handled        uint64
	Committed      uint64
	FetchErrors    uint64
	CommitErrors   uint64
	RetryPublished uint64
	DeadPublished  uint64
	Readers        []ConsumerReaderStats
}

type ConsumerReaderStats struct {
	Topic       string
	Messages    int64
	Bytes       int64
	Rebalances  int64
	Timeouts    int64
	Errors      int64
	QueueLength int64
}

func InitConsumer() error {
	cfg := config.KafkaConfig()
	return initConsumer(cfg)
}

func InitConsumerForService(serviceName string) error {
	cfg := config.KafkaConfig()
	if !cfg.Enabled {
		Subscriber = nil
		return nil
	}
	consumer, err := NewConsumerForService(cfg, serviceName)
	if err != nil {
		return err
	}
	Subscriber = consumer
	return nil
}

func InitReplayableConsumerForService(serviceName string) error {
	cfg := config.KafkaConfig()
	if !cfg.Enabled {
		Subscriber = nil
		return nil
	}
	consumer, err := NewReplayableConsumerForService(cfg, serviceName)
	if err != nil {
		return err
	}
	Subscriber = consumer
	return nil
}

// NewConsumerForService builds an isolated consumer with a service-owned group.
func NewConsumerForService(cfg config.Kafka, serviceName string) (*Consumer, error) {
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return nil, errors.New("kafka consumer service name is required")
	}
	if !cfg.Enabled {
		return nil, errors.New("kafka consumer requires kafka.enabled")
	}
	cfg.ClientID = serviceName
	return newConsumer(cfg)
}

// NewReplayableConsumerForService starts a new group at the earliest retained
// offset. Kafka still resumes an existing group from its committed offsets.
func NewReplayableConsumerForService(cfg config.Kafka, serviceName string) (*Consumer, error) {
	consumer, err := NewConsumerForService(cfg, serviceName)
	if err != nil {
		return nil, err
	}
	consumer.startOffset = kafkago.FirstOffset
	return consumer, nil
}

func initConsumer(cfg config.Kafka) error {
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
	groupBalancer, heartbeat, session, rebalance, err := normalizeConsumerGroupPolicy(
		cfg.ConsumerGroupBalancer,
		cfg.ConsumerHeartbeatSeconds,
		cfg.ConsumerSessionTimeoutSeconds,
		cfg.ConsumerRebalanceTimeoutSeconds,
	)
	if err != nil {
		return nil, err
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
		clientID:          clientID,
		groupID:           clientID + "-consumer",
		topicPrefix:       strings.TrimSpace(cfg.TopicPrefix),
		brokers:           brokers,
		dialTimeout:       timeout,
		maxAttempts:       normalizeRetryMaxAttempts(cfg.ConsumeRetryMaxAttempts),
		backoff:           normalizeRetryBackoff(cfg.ConsumeRetryBackoffMS),
		groupBalancer:     groupBalancer,
		heartbeatInterval: heartbeat,
		sessionTimeout:    session,
		rebalanceTimeout:  rebalance,
		handlers:          make(map[string][]Handler),
		readers:           make(map[string]*kafkago.Reader),
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

// UseFailurePublisher injects the publisher used for retry and dead-letter
// transfer. Call it before Start; runtime consumers otherwise use Client.
func (c *Consumer) UseFailurePublisher(publisher *Publisher) {
	if c != nil {
		c.failurePublisher = publisher
	}
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
			c.fetchErrors.Add(1)
			time.Sleep(500 * time.Millisecond)
			continue
		}
		c.fetched.Add(1)

		event := Event{
			Topic:     topic,
			Key:       message.Key,
			Value:     message.Value,
			Headers:   decodeHeaders(message.Headers),
			Partition: message.Partition,
			Offset:    message.Offset,
			Time:      message.Time,
		}
		envelope, decodeErr := DecodeEnvelope(message.Value)
		if decodeErr == nil {
			event.Envelope = envelope
		} else {
			event.DecodeErr = decodeErr
		}

		if c.handleWithRetry(ctx, event, handlers) {
			c.handled.Add(1)
			if err := reader.CommitMessages(ctx, message); err != nil {
				c.commitErrors.Add(1)
				continue
			}
			c.committed.Add(1)
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
	ctx = correlationContext(ctx, event)
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

func correlationContext(ctx context.Context, event Event) context.Context {
	requestID := event.Headers[correlation.RequestEventHeader]
	traceID := event.Headers[correlation.TraceEventHeader]
	eventID := event.Headers[correlation.EventHeader]
	if event.Envelope != nil {
		if event.Envelope.RequestID != "" {
			requestID = event.Envelope.RequestID
		}
		if event.Envelope.TraceID != "" {
			traceID = event.Envelope.TraceID
		}
		if event.Envelope.EventID != "" {
			eventID = event.Envelope.EventID
		}
	}
	ctx, _ = correlation.Ensure(ctx, requestID, traceID)
	return correlation.WithEventID(ctx, eventID)
}

func (c *Consumer) readerForTopic(topic string) *kafkago.Reader {
	c.mu.Lock()
	defer c.mu.Unlock()

	if reader, ok := c.readers[topic]; ok {
		return reader
	}

	reader := kafkago.NewReader(c.readerConfig(topic))
	c.readers[topic] = reader
	return reader
}

func (c *Consumer) readerConfig(topic string) kafkago.ReaderConfig {
	startOffset := c.startOffset
	if startOffset != kafkago.FirstOffset && startOffset != kafkago.LastOffset {
		startOffset = kafkago.LastOffset
	}
	return kafkago.ReaderConfig{
		Brokers:               c.brokers,
		GroupID:               c.groupID,
		Topic:                 topic,
		StartOffset:           startOffset,
		MaxWait:               kafkaReaderMaxWait,
		GroupBalancers:        []kafkago.GroupBalancer{c.groupBalancer},
		HeartbeatInterval:     c.heartbeatInterval,
		SessionTimeout:        c.sessionTimeout,
		RebalanceTimeout:      c.rebalanceTimeout,
		WatchPartitionChanges: true,
		Dialer:                &kafkago.Dialer{Timeout: c.dialTimeout, ClientID: c.clientID},
	}
}

// CollectStats returns cumulative delivery outcomes and per-reader deltas
// since the previous collection, matching kafka-go Reader.Stats semantics.
func (c *Consumer) CollectStats() ConsumerStats {
	if c == nil {
		return ConsumerStats{}
	}
	result := ConsumerStats{
		ClientID:       c.clientID,
		GroupID:        c.groupID,
		Fetched:        c.fetched.Load(),
		Handled:        c.handled.Load(),
		Committed:      c.committed.Load(),
		FetchErrors:    c.fetchErrors.Load(),
		CommitErrors:   c.commitErrors.Load(),
		RetryPublished: c.retryPublished.Load(),
		DeadPublished:  c.deadPublished.Load(),
	}
	c.mu.RLock()
	readers := make(map[string]*kafkago.Reader, len(c.readers))
	for topic, reader := range c.readers {
		readers[topic] = reader
	}
	c.mu.RUnlock()
	for topic, reader := range readers {
		stats := reader.Stats()
		result.Readers = append(result.Readers, ConsumerReaderStats{
			Topic: topic, Messages: stats.Messages, Bytes: stats.Bytes,
			Rebalances: stats.Rebalances, Timeouts: stats.Timeouts,
			Errors: stats.Errors, QueueLength: stats.QueueLength,
		})
	}
	sort.Slice(result.Readers, func(i, j int) bool { return result.Readers[i].Topic < result.Readers[j].Topic })
	return result
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

func DecodeEnvelope(value []byte) (*Envelope, error) {
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
	publisher := c.retryPublisher()
	if publisher == nil {
		return false
	}

	attempt := headerRetryAttempt(event.Headers)
	headers := cloneHeaders(event.Headers)
	headers["last_error"] = lastErr.Error()

	baseTopic := c.baseTopicName(event.Topic)
	if attempt+1 < c.maxAttempts {
		headers["retry_attempt"] = strconv.Itoa(attempt + 1)
		retryTopic := retryTopicName(baseTopic)
		published := publisher.Publish(ctx, retryTopic, Message{
			Key:     event.Key,
			Value:   event.Value,
			Headers: headers,
		}) == nil
		if published {
			c.retryPublished.Add(1)
		}
		return published
	}

	return c.publishDeadLetter(ctx, event, lastErr, "handler_failed")
}

func (c *Consumer) publishDeadLetter(ctx context.Context, event Event, lastErr error, reason string) bool {
	publisher := c.retryPublisher()
	if publisher == nil {
		return false
	}
	headers := c.deadLetterHeaders(event, lastErr, reason)
	deadTopic := deadTopicName(c.baseTopicName(event.Topic))
	published := publisher.Publish(ctx, deadTopic, Message{
		Key:     event.Key,
		Value:   event.Value,
		Headers: headers,
	}) == nil
	if published {
		c.deadPublished.Add(1)
	}
	return published
}

func (c *Consumer) retryPublisher() *Publisher {
	if c != nil && c.failurePublisher != nil {
		return c.failurePublisher
	}
	return Client
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

func normalizeConsumerGroupPolicy(balancerName string, heartbeatSeconds, sessionSeconds, rebalanceSeconds int) (kafkago.GroupBalancer, time.Duration, time.Duration, time.Duration, error) {
	var balancer kafkago.GroupBalancer
	switch strings.ToLower(strings.TrimSpace(balancerName)) {
	case "", "roundrobin", "round-robin":
		balancer = kafkago.RoundRobinGroupBalancer{}
	case "range":
		balancer = kafkago.RangeGroupBalancer{}
	default:
		return nil, 0, 0, 0, fmt.Errorf("unsupported kafka consumer group balancer %q", balancerName)
	}
	if heartbeatSeconds <= 0 {
		heartbeatSeconds = 3
	}
	if sessionSeconds <= 0 {
		sessionSeconds = 30
	}
	if rebalanceSeconds <= 0 {
		rebalanceSeconds = 30
	}
	heartbeat := time.Duration(heartbeatSeconds) * time.Second
	session := time.Duration(sessionSeconds) * time.Second
	rebalance := time.Duration(rebalanceSeconds) * time.Second
	if heartbeat >= session {
		return nil, 0, 0, 0, fmt.Errorf("kafka consumer heartbeat %s must be shorter than session timeout %s", heartbeat, session)
	}
	if rebalance < session {
		return nil, 0, 0, 0, fmt.Errorf("kafka consumer rebalance timeout %s must be at least session timeout %s", rebalance, session)
	}
	return balancer, heartbeat, session, rebalance, nil
}
