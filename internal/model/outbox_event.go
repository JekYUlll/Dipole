package model

import "time"

const (
	OutboxStatusPending    = "pending"
	OutboxStatusProcessing = "processing"
	OutboxStatusPublished  = "published"
)

// OutboxEvent stores the final Kafka message bytes that still need to be published.
type OutboxEvent struct {
	ID            uint       `json:"id"`
	AggregateType string     `json:"aggregate_type"`
	AggregateID   string     `json:"aggregate_id"`
	EventType     string     `json:"event_type"`
	Topic         string     `json:"topic"`
	MessageKey    string     `json:"message_key"`
	Value         []byte     `json:"value"`
	HeadersJSON   []byte     `json:"headers_json,omitempty"`
	Status        string     `json:"status"`
	RetryCount    int        `json:"retry_count"`
	LastError     string     `json:"last_error,omitempty"`
	NextRetryAt   *time.Time `json:"next_retry_at,omitempty"`
	LockedAt      *time.Time `json:"locked_at,omitempty"`
	PublishedAt   *time.Time `json:"published_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
