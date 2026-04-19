package model

import "time"

const (
	OutboxStatusPending    = "pending"
	OutboxStatusProcessing = "processing"
	OutboxStatusPublished  = "published"
)

// OutboxEvent stores the final Kafka message bytes that still need to be published.
// The business row and the outbox row are inserted in the same DB transaction so
// a process crash or transient Kafka failure cannot leave "message persisted but
// created event missing" gaps in the async pipeline.
type OutboxEvent struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	AggregateType string     `gorm:"size:32;not null;uniqueIndex:idx_outbox_aggregate_event,priority:1" json:"aggregate_type"`
	AggregateID   string     `gorm:"size:64;not null;uniqueIndex:idx_outbox_aggregate_event,priority:2" json:"aggregate_id"`
	EventType     string     `gorm:"size:128;not null;uniqueIndex:idx_outbox_aggregate_event,priority:3;index:idx_outbox_status_next_retry,priority:2" json:"event_type"`
	Topic         string     `gorm:"size:128;not null" json:"topic"`
	MessageKey    string     `gorm:"size:128;not null" json:"message_key"`
	Value         []byte     `gorm:"not null" json:"value"`
	HeadersJSON   []byte     `json:"headers_json,omitempty"`
	Status        string     `gorm:"size:16;not null;index:idx_outbox_status_next_retry,priority:1" json:"status"`
	RetryCount    int        `gorm:"not null;default:0" json:"retry_count"`
	LastError     string     `gorm:"size:512" json:"last_error,omitempty"`
	NextRetryAt   *time.Time `gorm:"index:idx_outbox_status_next_retry,priority:3" json:"next_retry_at,omitempty"`
	LockedAt      *time.Time `gorm:"index" json:"locked_at,omitempty"`
	PublishedAt   *time.Time `json:"published_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
