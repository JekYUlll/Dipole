package cassandrareconcile

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"

	cassandrabackfill "github.com/JekYUlll/Dipole/internal/backfill/cassandra"
	cassandradata "github.com/JekYUlll/Dipole/internal/data/cassandra"
)

type Source interface {
	ListAfter(context.Context, uint64, uint64, int) ([]cassandrabackfill.SourceMessage, error)
}

type Target interface {
	Lookup(context.Context, string, uint64) (cassandradata.TimelineRecord, bool, error)
}

type Config struct {
	JobName         string
	HighWatermarkID uint64
	BatchSize       int
	SampleModulus   uint64
	MaxExamples     int
}

type MismatchExample struct {
	Kind            string `json:"kind"`
	MessageUUID     string `json:"message_uuid,omitempty"`
	ConversationKey string `json:"conversation_key"`
	MessageSeq      uint64 `json:"message_seq"`
	ExpectedHash    string `json:"expected_hash,omitempty"`
	ActualHash      string `json:"actual_hash,omitempty"`
}

type Report struct {
	JobName             string            `json:"job_name"`
	SourceHighWatermark uint64            `json:"source_high_watermark_id"`
	StartedAt           time.Time         `json:"started_at"`
	CompletedAt         time.Time         `json:"completed_at"`
	SourceCount         uint64            `json:"source_count"`
	TargetFoundCount    uint64            `json:"target_found_count"`
	HashMatchedCount    uint64            `json:"hash_matched_count"`
	MissingCount        uint64            `json:"missing_count"`
	HashMismatchCount   uint64            `json:"hash_mismatch_count"`
	SampledCount        uint64            `json:"sampled_count"`
	SampleMismatchCount uint64            `json:"sample_mismatch_count"`
	SourceSeqGapCount   uint64            `json:"source_seq_gap_count"`
	Consistent          bool              `json:"consistent"`
	Examples            []MismatchExample `json:"examples,omitempty"`
}

type Reconciler struct {
	source Source
	target Target
	config Config
	now    func() time.Time
}

func New(source Source, target Target, cfg Config) (*Reconciler, error) {
	switch {
	case source == nil:
		return nil, errors.New("Cassandra reconciliation source is required")
	case target == nil:
		return nil, errors.New("Cassandra reconciliation target is required")
	case strings.TrimSpace(cfg.JobName) == "":
		return nil, errors.New("Cassandra reconciliation job name is required")
	case cfg.BatchSize <= 0:
		return nil, errors.New("Cassandra reconciliation batch size must be positive")
	case cfg.BatchSize > cassandrabackfill.MaxBatchSize:
		return nil, fmt.Errorf("Cassandra reconciliation batch size cannot exceed %d", cassandrabackfill.MaxBatchSize)
	case cfg.SampleModulus == 0:
		return nil, errors.New("Cassandra reconciliation sample modulus must be positive")
	case cfg.MaxExamples < 0:
		return nil, errors.New("Cassandra reconciliation max examples cannot be negative")
	}
	return &Reconciler{source: source, target: target, config: cfg, now: time.Now}, nil
}

func (r *Reconciler) Run(ctx context.Context) (Report, error) {
	report := Report{
		JobName: r.config.JobName, SourceHighWatermark: r.config.HighWatermarkID,
		StartedAt: r.now().UTC(),
	}
	lastSequences := make(map[string]uint64)
	var afterID uint64
	for afterID < r.config.HighWatermarkID {
		messages, err := r.source.ListAfter(ctx, afterID, r.config.HighWatermarkID, r.config.BatchSize)
		if err != nil {
			return report, fmt.Errorf("list Cassandra reconciliation source: %w", err)
		}
		if len(messages) == 0 {
			return report, fmt.Errorf("Cassandra reconciliation source ended before high watermark %d", r.config.HighWatermarkID)
		}
		for _, sourceMessage := range messages {
			if sourceMessage.SourceID <= afterID || sourceMessage.SourceID > r.config.HighWatermarkID {
				return report, fmt.Errorf("Cassandra reconciliation source ID %d is outside (%d, %d]", sourceMessage.SourceID, afterID, r.config.HighWatermarkID)
			}
			if err := r.compareMessage(ctx, &report, lastSequences, sourceMessage); err != nil {
				return report, err
			}
			afterID = sourceMessage.SourceID
		}
	}
	report.CompletedAt = r.now().UTC()
	report.Consistent = report.TargetFoundCount == report.SourceCount &&
		report.HashMatchedCount == report.SourceCount &&
		report.MissingCount == 0 && report.HashMismatchCount == 0 &&
		report.SampleMismatchCount == 0 && report.SourceSeqGapCount == 0
	return report, nil
}

func (r *Reconciler) compareMessage(ctx context.Context, report *Report, lastSequences map[string]uint64, sourceMessage cassandrabackfill.SourceMessage) error {
	message := sourceMessage.Message
	report.SourceCount++
	expectedSequence := lastSequences[message.ConversationKey] + 1
	if message.Seq != expectedSequence {
		report.SourceSeqGapCount++
		r.addExample(report, MismatchExample{Kind: "source_seq_gap", MessageUUID: message.UUID, ConversationKey: message.ConversationKey, MessageSeq: message.Seq})
	}
	lastSequences[message.ConversationKey] = message.Seq

	expected := cassandrabackfill.ProjectionForMessage(message)
	expectedHash, err := expected.PayloadHash()
	if err != nil {
		return fmt.Errorf("hash Cassandra reconciliation source message %s: %w", message.UUID, err)
	}
	record, found, err := r.target.Lookup(ctx, message.ConversationKey, message.Seq)
	if err != nil {
		return fmt.Errorf("read Cassandra reconciliation target message %s: %w", message.UUID, err)
	}
	if !found {
		report.MissingCount++
		r.addExample(report, MismatchExample{Kind: "missing", MessageUUID: message.UUID, ConversationKey: message.ConversationKey, MessageSeq: message.Seq, ExpectedHash: expectedHash})
		return nil
	}
	report.TargetFoundCount++
	if record.PayloadHash == expectedHash {
		report.HashMatchedCount++
	} else {
		report.HashMismatchCount++
		r.addExample(report, MismatchExample{Kind: "hash_mismatch", MessageUUID: message.UUID, ConversationKey: message.ConversationKey, MessageSeq: message.Seq, ExpectedHash: expectedHash, ActualHash: record.PayloadHash})
	}
	if report.SourceCount == 1 || stableSample(message.UUID, r.config.SampleModulus) {
		report.SampledCount++
		if !samePayload(expected, record.Projection) {
			report.SampleMismatchCount++
			r.addExample(report, MismatchExample{Kind: "sample_mismatch", MessageUUID: message.UUID, ConversationKey: message.ConversationKey, MessageSeq: message.Seq, ExpectedHash: expectedHash, ActualHash: record.PayloadHash})
		}
	}
	return nil
}

func (r *Reconciler) addExample(report *Report, example MismatchExample) {
	if len(report.Examples) < r.config.MaxExamples {
		report.Examples = append(report.Examples, example)
	}
}

func stableSample(messageUUID string, modulus uint64) bool {
	digest := sha256.Sum256([]byte(messageUUID))
	return binary.BigEndian.Uint64(digest[:8])%modulus == 0
}

func samePayload(left, right cassandradata.TimelineProjection) bool {
	return left.ConversationKey == right.ConversationKey && left.MessageSeq == right.MessageSeq &&
		left.MessageUUID == right.MessageUUID && left.ClientMessageID == right.ClientMessageID &&
		left.SenderUUID == right.SenderUUID && left.TargetType == right.TargetType &&
		left.TargetUUID == right.TargetUUID && left.MessageType == right.MessageType &&
		left.Content == right.Content && left.FileID == right.FileID && left.FileName == right.FileName &&
		left.FileSize == right.FileSize && left.FileURL == right.FileURL &&
		left.FileContentType == right.FileContentType && sameOptionalTime(left.FileExpiresAt, right.FileExpiresAt) &&
		left.SentAt.Equal(right.SentAt)
}

func sameOptionalTime(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Equal(*right)
}
