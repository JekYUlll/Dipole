package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
)

type Publisher struct {
	clientID    string
	topicPrefix string
	brokers     []string
	timeout     time.Duration

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

	publisher, err := newPublisher(cfg)
	if err != nil {
		return err
	}

	Client = publisher
	return nil
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
		clientID:    strings.TrimSpace(cfg.ClientID),
		topicPrefix: strings.TrimSpace(cfg.TopicPrefix),
		brokers:     brokers,
		timeout:     writeTimeout,
		writers:     make(map[string]*kafkago.Writer),
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

	if headers == nil {
		headers = map[string]string{}
	}
	headers["event_type"] = envelope.EventType
	headers["version"] = envelope.Version
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
		Balancer:     &kafkago.LeastBytes{},
		RequiredAcks: kafkago.RequireOne,
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
