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

var Client *Publisher

const (
	// IM text/file messages favor tail latency over batching throughput.
	kafkaWriterBatchSize    = 1
	kafkaWriterBatchTimeout = 5 * time.Millisecond
	topicMetadataAttempts   = 30
	topicMetadataBackoff    = 100 * time.Millisecond
)

type Publisher struct {
	clientID     string
	topicPrefix  string
	brokers      []string
	timeout      time.Duration
	partitions   int
	replicas     int
	minISR       int
	retention    time.Duration
	requiredAcks kafkago.RequiredAcks

	mu      sync.Mutex
	writers map[string]*kafkago.Writer
}

type Message struct {
	Key     []byte
	Value   []byte
	Headers map[string]string
}

func Init() error {
	cfg := config.KafkaConfig()
	if !cfg.Enabled {
		Client = nil
		return nil
	}

	publisher, err := NewPublisher(cfg)
	if err != nil {
		return err
	}

	Client = publisher
	return nil
}

// NewPublisher builds an isolated publisher for service runtimes and integration tests.
func NewPublisher(cfg config.Kafka) (*Publisher, error) {
	return newPublisher(cfg)
}

func Close() error {
	if Client == nil {
		return nil
	}

	err := Client.Close()
	Client = nil
	return err
}

func newPublisher(cfg config.Kafka) (*Publisher, error) {
	brokers := normalizeBrokers(cfg.Brokers)
	if len(brokers) == 0 {
		return nil, fmt.Errorf("kafka brokers are empty")
	}
	requiredAcks, err := normalizeRequiredAcks(cfg.RequiredAcks)
	if err != nil {
		return nil, err
	}
	minISR := normalizeTopicMinISR(cfg.TopicMinInSyncReplicas)
	replicas := normalizeTopicReplicationFactor(cfg.TopicReplicationFactor)
	if minISR > replicas {
		return nil, fmt.Errorf("kafka topic min ISR %d exceeds replication factor %d", minISR, replicas)
	}

	timeout := time.Duration(cfg.DialTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	if err := pingBroker(brokers[0], timeout); err != nil {
		return nil, err
	}

	writeTimeout := time.Duration(cfg.WriteTimeoutSeconds) * time.Second
	if writeTimeout <= 0 {
		writeTimeout = 5 * time.Second
	}

	return &Publisher{
		clientID:     strings.TrimSpace(cfg.ClientID),
		topicPrefix:  strings.TrimSpace(cfg.TopicPrefix),
		brokers:      brokers,
		timeout:      writeTimeout,
		partitions:   normalizeTopicPartitions(cfg.TopicPartitions),
		replicas:     replicas,
		minISR:       minISR,
		retention:    normalizeTopicRetention(cfg.TopicRetentionHours),
		requiredAcks: requiredAcks,
		writers:      make(map[string]*kafkago.Writer),
	}, nil
}

func (p *Publisher) Publish(ctx context.Context, topic string, message Message) error {
	if p == nil {
		return errors.New("kafka publisher is not initialized")
	}

	topic = p.topicName(topic)
	writer := p.writerForTopic(topic)

	headers := make([]kafkago.Header, 0, len(message.Headers))
	for key, value := range message.Headers {
		headers = append(headers, kafkago.Header{
			Key:   key,
			Value: []byte(value),
		})
	}

	writeCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	if err := writer.WriteMessages(writeCtx, kafkago.Message{
		Key:     message.Key,
		Value:   message.Value,
		Headers: headers,
		Time:    time.Now(),
	}); err != nil {
		return fmt.Errorf("publish kafka message to %s: %w", topic, err)
	}

	return nil
}

func (p *Publisher) PublishJSON(ctx context.Context, topic string, key string, payload any, headers map[string]string) error {
	if p == nil {
		return nil
	}

	value, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal kafka payload for %s: %w", topic, err)
	}

	message := Message{
		Value:   value,
		Headers: headers,
	}
	if key != "" {
		message.Key = []byte(key)
	}

	if err := p.Publish(ctx, topic, message); err != nil {
		return fmt.Errorf("publish kafka json message to %s: %w", topic, err)
	}

	return nil
}

func (p *Publisher) PublishEvent(ctx context.Context, topic string, key string, eventType string, payload any, headers map[string]string) error {
	envelope, err := NewEnvelope(eventType, payload)
	if err != nil {
		return fmt.Errorf("create kafka event envelope for %s: %w", topic, err)
	}

	headers = cloneHeaders(headers)
	headers["event_type"] = envelope.EventType
	headers["version"] = envelope.Version
	headers["schema_version"] = envelope.Version
	headers["source"] = envelope.Source
	headers["event_id"] = envelope.EventID

	return p.PublishJSON(ctx, topic, key, envelope, headers)
}

func (p *Publisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var errs []error
	for topic, writer := range p.writers {
		if err := writer.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close kafka writer %s: %w", topic, err))
		}
		delete(p.writers, topic)
	}

	return errors.Join(errs...)
}

func (p *Publisher) EnsureTopics(topics []string) error {
	if p == nil {
		return errors.New("kafka publisher is not initialized")
	}

	configs := make([]kafkago.TopicConfig, 0, len(topics)*3)
	seen := make(map[string]struct{}, len(topics)*3)
	for _, topic := range topics {
		fullTopic := p.topicName(topic)
		for _, candidate := range []string{fullTopic, retryTopicName(fullTopic), deadTopicName(fullTopic)} {
			if strings.TrimSpace(candidate) == "" {
				continue
			}
			if _, ok := seen[candidate]; ok {
				continue
			}
			seen[candidate] = struct{}{}
			configs = append(configs, kafkago.TopicConfig{
				Topic:             candidate,
				NumPartitions:     p.partitions,
				ReplicationFactor: p.replicas,
				ConfigEntries:     topicConfigEntries(p.minISR, p.retention),
			})
		}
	}

	if len(configs) == 0 {
		return nil
	}

	conn, err := kafkago.DialContext(context.Background(), "tcp", p.brokers[0])
	if err != nil {
		return fmt.Errorf("dial kafka broker for ensure topics: %w", err)
	}
	defer func() { _ = conn.Close() }()

	controller, err := conn.Controller()
	if err != nil {
		return fmt.Errorf("get kafka controller: %w", err)
	}

	controllerConn, err := kafkago.DialContext(
		context.Background(),
		"tcp",
		fmt.Sprintf("%s:%d", controller.Host, controller.Port),
	)
	if err != nil {
		return fmt.Errorf("dial kafka controller: %w", err)
	}
	defer func() { _ = controllerConn.Close() }()

	if err := controllerConn.CreateTopics(configs...); err != nil && !isTopicAlreadyExistsError(err) {
		return fmt.Errorf("create kafka topics: %w", err)
	}

	metadataContext, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	partitionMetadata, err := readTopicPartitionsWithRetry(metadataContext, func() ([]kafkago.Partition, error) {
		return controllerConn.ReadPartitions(topicNames(configs)...)
	})
	if err != nil {
		return fmt.Errorf("read kafka partitions after create: %w", err)
	}

	existing := make(map[string]int, len(configs))
	for _, partition := range partitionMetadata {
		existing[partition.Topic]++
	}

	expand := make([]kafkago.TopicPartitionsConfig, 0)
	for _, cfg := range configs {
		if existing[cfg.Topic] >= cfg.NumPartitions {
			continue
		}
		expand = append(expand, kafkago.TopicPartitionsConfig{
			Name:  cfg.Topic,
			Count: int32(cfg.NumPartitions),
		})
	}
	if len(expand) == 0 {
		return nil
	}

	client := &kafkago.Client{
		Addr:      controllerConn.RemoteAddr(),
		Transport: &kafkago.Transport{ClientID: p.clientID},
	}
	response, err := client.CreatePartitions(context.Background(), &kafkago.CreatePartitionsRequest{
		Addr:   controllerConn.RemoteAddr(),
		Topics: expand,
	})
	if err != nil {
		return fmt.Errorf("expand kafka topic partitions: %w", err)
	}
	for topic, topicErr := range response.Errors {
		if topicErr == nil {
			continue
		}
		return fmt.Errorf("expand kafka topic %s partitions: %w", topic, topicErr)
	}

	return nil
}

func readTopicPartitionsWithRetry(ctx context.Context, read func() ([]kafkago.Partition, error)) ([]kafkago.Partition, error) {
	var lastErr error
	for attempt := 1; attempt <= topicMetadataAttempts; attempt++ {
		partitions, err := read()
		if err == nil {
			return partitions, nil
		}
		lastErr = err
		if attempt == topicMetadataAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return nil, errors.Join(lastErr, ctx.Err())
		case <-time.After(topicMetadataBackoff):
		}
	}
	return nil, lastErr
}

func (p *Publisher) topicName(topic string) string {
	topic = strings.TrimSpace(topic)
	if p.topicPrefix == "" {
		return topic
	}
	if topic == "" {
		return p.topicPrefix
	}

	return p.topicPrefix + "." + topic
}

func (p *Publisher) writerForTopic(topic string) *kafkago.Writer {
	p.mu.Lock()
	defer p.mu.Unlock()

	if writer, ok := p.writers[topic]; ok {
		return writer
	}

	writer := &kafkago.Writer{
		Addr:         kafkago.TCP(p.brokers...),
		Topic:        topic,
		Balancer:     &kafkago.Hash{},
		RequiredAcks: p.requiredAcks,
		Async:        false,
		BatchSize:    kafkaWriterBatchSize,
		BatchTimeout: kafkaWriterBatchTimeout,
		Transport: &kafkago.Transport{
			ClientID: p.clientID,
		},
	}
	p.writers[topic] = writer
	return writer
}

func pingBroker(broker string, timeout time.Duration) error {
	conn, err := kafkago.DialContext(context.Background(), "tcp", broker)
	if err != nil {
		return fmt.Errorf("dial kafka broker %s: %w", broker, err)
	}
	defer func() { _ = conn.Close() }()

	_ = conn.SetDeadline(time.Now().Add(timeout))
	if _, err := conn.Brokers(); err != nil {
		return fmt.Errorf("fetch kafka brokers from %s: %w", broker, err)
	}

	return nil
}

func normalizeBrokers(brokers []string) []string {
	normalized := make([]string, 0, len(brokers))
	seen := make(map[string]struct{}, len(brokers))
	for _, broker := range brokers {
		broker = strings.TrimSpace(broker)
		if broker == "" {
			continue
		}
		if _, ok := seen[broker]; ok {
			continue
		}
		seen[broker] = struct{}{}
		normalized = append(normalized, broker)
	}

	return normalized
}

func normalizeTopicPartitions(partitions int) int {
	if partitions <= 0 {
		return 1
	}
	return partitions
}

func normalizeTopicReplicationFactor(replicas int) int {
	if replicas <= 0 {
		return 1
	}
	return replicas
}

func normalizeTopicMinISR(minISR int) int {
	if minISR <= 0 {
		return 1
	}
	return minISR
}

func normalizeTopicRetention(hours int) time.Duration {
	if hours <= 0 {
		hours = 168
	}
	return time.Duration(hours) * time.Hour
}

func normalizeRequiredAcks(value string) (kafkago.RequiredAcks, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "one", "1":
		return kafkago.RequireOne, nil
	case "all", "-1":
		return kafkago.RequireAll, nil
	default:
		return 0, fmt.Errorf("kafka required_acks must be one or all, got %q", value)
	}
}

func topicConfigEntries(minISR int, retention time.Duration) []kafkago.ConfigEntry {
	if retention <= 0 {
		retention = normalizeTopicRetention(0)
	}
	return []kafkago.ConfigEntry{
		{ConfigName: "min.insync.replicas", ConfigValue: strconv.Itoa(normalizeTopicMinISR(minISR))},
		{ConfigName: "retention.ms", ConfigValue: strconv.FormatInt(retention.Milliseconds(), 10)},
	}
}

func isTopicAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "topic with this name already exists")
}

func topicNames(configs []kafkago.TopicConfig) []string {
	names := make([]string, 0, len(configs))
	for _, cfg := range configs {
		if strings.TrimSpace(cfg.Topic) == "" {
			continue
		}
		names = append(names, cfg.Topic)
	}
	return names
}
