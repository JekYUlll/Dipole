package cassandra

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/apache/cassandra-gocql-driver/v2"
)

const (
	DefaultTimelineBucketSize uint64 = 10_000
	TimelineTableName                = "timeline_by_conversation_bucket"
)

var ErrProjectionConflict = errors.New("Cassandra timeline projection conflicts with existing sequence")

type TimelineProjection struct {
	EventID         string
	EventVersion    string
	ConversationKey string
	MessageSeq      uint64
	MessageUUID     string
	ClientMessageID string
	SenderUUID      string
	TargetType      int8
	TargetUUID      string
	MessageType     int8
	Content         string
	FileID          string
	FileName        string
	FileSize        int64
	FileURL         string
	FileContentType string
	FileExpiresAt   *time.Time
	SentAt          time.Time
}

type AppendResult struct {
	Inserted  bool
	Duplicate bool
}

type TimelineRecord struct {
	Projection  TimelineProjection
	PayloadHash string
}

type TimelineStore struct {
	session    *gocql.Session
	bucketSize uint64
}

func NewTimelineStore(session *gocql.Session, bucketSize uint64) (*TimelineStore, error) {
	if session == nil {
		return nil, fmt.Errorf("Cassandra session is required")
	}
	if bucketSize == 0 {
		return nil, fmt.Errorf("Cassandra timeline bucket size must be positive")
	}
	return &TimelineStore{session: session, bucketSize: bucketSize}, nil
}

func BucketForSequence(sequence, bucketSize uint64) (int64, error) {
	if sequence == 0 {
		return 0, fmt.Errorf("message sequence must be positive")
	}
	if bucketSize == 0 {
		return 0, fmt.Errorf("timeline bucket size must be positive")
	}
	return int64((sequence - 1) / bucketSize), nil
}

func (s *TimelineStore) Append(ctx context.Context, projection TimelineProjection) (AppendResult, error) {
	if err := validateProjection(projection); err != nil {
		return AppendResult{}, err
	}
	bucket, err := BucketForSequence(projection.MessageSeq, s.bucketSize)
	if err != nil {
		return AppendResult{}, err
	}
	payloadHash, err := projection.PayloadHash()
	if err != nil {
		return AppendResult{}, err
	}

	existing := make(map[string]interface{})
	applied, err := s.session.Query(`
INSERT INTO timeline_by_conversation_bucket (
    conversation_key, bucket, message_seq, message_uuid, client_message_id,
    sender_uuid, target_type, target_uuid, message_type, content,
    file_id, file_name, file_size, file_url, file_content_type, file_expires_at,
    sent_at, event_id, event_version, payload_hash, projected_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
IF NOT EXISTS`,
		projection.ConversationKey,
		bucket,
		int64(projection.MessageSeq),
		projection.MessageUUID,
		projection.ClientMessageID,
		projection.SenderUUID,
		projection.TargetType,
		projection.TargetUUID,
		projection.MessageType,
		projection.Content,
		projection.FileID,
		projection.FileName,
		projection.FileSize,
		projection.FileURL,
		projection.FileContentType,
		projection.FileExpiresAt,
		projection.SentAt.UTC(),
		projection.EventID,
		projection.EventVersion,
		payloadHash,
		time.Now().UTC(),
	).WithContext(ctx).MapScanCAS(existing)
	if err != nil {
		return AppendResult{}, fmt.Errorf("append Cassandra timeline projection: %w", err)
	}
	if applied {
		return AppendResult{Inserted: true}, nil
	}

	existingHash, _ := existing["payload_hash"].(string)
	if existingHash == payloadHash {
		return AppendResult{Duplicate: true}, nil
	}
	return AppendResult{}, fmt.Errorf("%w: conversation=%s seq=%d", ErrProjectionConflict, projection.ConversationKey, projection.MessageSeq)
}

func (s *TimelineStore) Lookup(ctx context.Context, conversationKey string, sequence uint64) (TimelineRecord, bool, error) {
	conversationKey = strings.TrimSpace(conversationKey)
	if conversationKey == "" {
		return TimelineRecord{}, false, fmt.Errorf("Cassandra timeline lookup conversation key is required")
	}
	bucket, err := BucketForSequence(sequence, s.bucketSize)
	if err != nil {
		return TimelineRecord{}, false, err
	}

	var record TimelineRecord
	var fileExpiresAt *time.Time
	err = s.session.Query(`
SELECT message_uuid, client_message_id, sender_uuid, target_type, target_uuid,
       message_type, content, file_id, file_name, file_size, file_url,
       file_content_type, file_expires_at, sent_at, payload_hash
FROM timeline_by_conversation_bucket
WHERE conversation_key = ? AND bucket = ? AND message_seq = ?`,
		conversationKey, bucket, int64(sequence),
	).WithContext(ctx).Scan(
		&record.Projection.MessageUUID,
		&record.Projection.ClientMessageID,
		&record.Projection.SenderUUID,
		&record.Projection.TargetType,
		&record.Projection.TargetUUID,
		&record.Projection.MessageType,
		&record.Projection.Content,
		&record.Projection.FileID,
		&record.Projection.FileName,
		&record.Projection.FileSize,
		&record.Projection.FileURL,
		&record.Projection.FileContentType,
		&fileExpiresAt,
		&record.Projection.SentAt,
		&record.PayloadHash,
	)
	if errors.Is(err, gocql.ErrNotFound) {
		return TimelineRecord{}, false, nil
	}
	if err != nil {
		return TimelineRecord{}, false, fmt.Errorf("lookup Cassandra timeline projection: %w", err)
	}
	record.Projection.ConversationKey = conversationKey
	record.Projection.MessageSeq = sequence
	record.Projection.FileExpiresAt = fileExpiresAt
	return record, true, nil
}

func (p TimelineProjection) PayloadHash() (string, error) {
	p.EventID = ""
	p.EventVersion = ""
	if p.FileExpiresAt != nil {
		value := p.FileExpiresAt.UTC()
		p.FileExpiresAt = &value
	}
	p.SentAt = p.SentAt.UTC()
	raw, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("marshal Cassandra timeline projection: %w", err)
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

func validateProjection(projection TimelineProjection) error {
	switch {
	case strings.TrimSpace(projection.EventID) == "":
		return fmt.Errorf("projection event ID is required")
	case strings.TrimSpace(projection.EventVersion) == "":
		return fmt.Errorf("projection event version is required")
	case strings.TrimSpace(projection.ConversationKey) == "":
		return fmt.Errorf("projection conversation key is required")
	case projection.MessageSeq == 0:
		return fmt.Errorf("projection message sequence must be positive")
	case strings.TrimSpace(projection.MessageUUID) == "":
		return fmt.Errorf("projection message UUID is required")
	case projection.SentAt.IsZero():
		return fmt.Errorf("projection sent time is required")
	default:
		return nil
	}
}
