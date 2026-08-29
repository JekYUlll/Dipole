package routing

import (
	"context"
	"hash/fnv"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/logger"
	"github.com/JekYUlll/Dipole/internal/model"
	cassandraData "github.com/JekYUlll/Dipole/internal/platform/cassandra"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

type HighWatermarkReader interface {
	LatestConversationSequence(conversationKey string) (uint64, error)
}

type TimelineRangeReader interface {
	ListRange(ctx context.Context, conversationKey string, firstSeq, lastSeq uint64) ([]cassandraData.TimelineRecord, error)
}

type ReadObservation struct {
	ConversationKey     string
	Operation           string
	Route               string
	FallbackReason      string
	VerificationOutcome string
	BeforeSeq           uint64
	AfterSeq            uint64
	HighWatermark       uint64
	ResultCount         int
	Latency             time.Duration
}

type MessageStore struct {
	application.MessageStore
	highWatermark    HighWatermarkReader
	timeline         TimelineRangeReader
	percentage       int
	verifyPercentage int
	observe          func(ReadObservation)
	requests         *prometheus.CounterVec
	latency          *prometheus.HistogramVec
	verifications    *prometheus.CounterVec
}

var _ application.MessageStore = (*MessageStore)(nil)
var _ application.MessageMetadataStore = (*MessageStore)(nil)

func (s *MessageStore) GetMetadataByUUID(uuid string) (*model.MessageMetadata, error) {
	if store, ok := s.MessageStore.(application.MessageMetadataStore); ok {
		return store.GetMetadataByUUID(uuid)
	}
	message, err := s.MessageStore.GetByUUID(uuid)
	return model.MetadataFromMessage(message), err
}

func (s *MessageStore) GetMetadataBySenderAndClientMessageID(senderUUID, clientMessageID string) (*model.MessageMetadata, error) {
	if store, ok := s.MessageStore.(application.MessageMetadataStore); ok {
		return store.GetMetadataBySenderAndClientMessageID(senderUUID, clientMessageID)
	}
	message, err := s.MessageStore.GetBySenderAndClientMessageID(senderUUID, clientMessageID)
	return model.MetadataFromMessage(message), err
}

func NewMessageStore(primary application.MessageStore, highWatermark HighWatermarkReader, timeline TimelineRangeReader, percentage int, observe func(ReadObservation)) *MessageStore {
	return NewMessageStoreWithVerification(primary, highWatermark, timeline, percentage, 0, observe)
}

func NewMessageStoreWithVerification(primary application.MessageStore, highWatermark HighWatermarkReader, timeline TimelineRangeReader, percentage, verifyPercentage int, observe func(ReadObservation)) *MessageStore {
	if percentage < 0 {
		percentage = 0
	}
	if percentage > 100 {
		percentage = 100
	}
	if verifyPercentage < 0 {
		verifyPercentage = 0
	}
	if verifyPercentage > 100 {
		verifyPercentage = 100
	}
	if observe == nil {
		observe = logReadObservation
	}
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "dipole_message_read_route_total",
		Help: "Message Seq reads by selected storage route and fallback reason.",
	}, []string{"route", "fallback_reason"})
	latency := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "dipole_message_read_route_duration_seconds",
		Help:    "Message Seq read latency by selected storage route.",
		Buckets: prometheus.DefBuckets,
	}, []string{"route"})
	verifications := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "dipole_message_read_verification_total",
		Help: "Sampled Cassandra message reads compared with MySQL by operation and outcome.",
	}, []string{"operation", "outcome"})
	return &MessageStore{
		MessageStore: primary, highWatermark: highWatermark, timeline: timeline,
		percentage: percentage, verifyPercentage: verifyPercentage, observe: observe,
		requests: requests, latency: latency, verifications: verifications,
	}
}

func (s *MessageStore) ListByConversationSeqAfter(conversationKey string, afterSeq uint64, limit int) ([]*model.Message, error) {
	if limit <= 0 {
		return s.MessageStore.ListByConversationSeqAfter(conversationKey, afterSeq, limit)
	}
	if !conversationInCohort(conversationKey, s.percentage) {
		startedAt := time.Now()
		page, err := s.MessageStore.ListByConversationSeqAfter(conversationKey, afterSeq, limit)
		s.record(ReadObservation{
			ConversationKey: conversationKey, Operation: "after_seq", Route: "mysql", AfterSeq: afterSeq,
			ResultCount: len(page), Latency: time.Since(startedAt),
		})
		return page, err
	}
	startedAt := time.Now()
	observation := ReadObservation{ConversationKey: conversationKey, Operation: "after_seq", AfterSeq: afterSeq}
	highWatermark, err := s.highWatermark.LatestConversationSequence(conversationKey)
	observation.HighWatermark = highWatermark
	if err != nil {
		return s.fallback(conversationKey, afterSeq, limit, observation, "high_watermark_error", startedAt)
	}
	if afterSeq >= highWatermark {
		return s.completeAfter(conversationKey, afterSeq, limit, []*model.Message{}, observation, startedAt)
	}

	lastSeq := highWatermark
	if uint64(limit) < highWatermark-afterSeq {
		lastSeq = afterSeq + uint64(limit)
	}
	records, err := s.timeline.ListRange(context.Background(), conversationKey, afterSeq+1, lastSeq)
	if err != nil {
		return s.fallback(conversationKey, afterSeq, limit, observation, "cassandra_error", startedAt)
	}
	if !continuous(records, afterSeq+1, lastSeq) {
		return s.fallback(conversationKey, afterSeq, limit, observation, "incomplete_page", startedAt)
	}

	return s.completeAfter(conversationKey, afterSeq, limit, recordsToMessages(records), observation, startedAt)
}

func (s *MessageStore) ListByConversationSeqBefore(conversationKey string, beforeSeq uint64, limit int) ([]*model.Message, error) {
	if limit <= 0 {
		return s.MessageStore.ListByConversationSeqBefore(conversationKey, beforeSeq, limit)
	}
	if !conversationInCohort(conversationKey, s.percentage) {
		startedAt := time.Now()
		page, err := s.MessageStore.ListByConversationSeqBefore(conversationKey, beforeSeq, limit)
		s.record(ReadObservation{
			ConversationKey: conversationKey, Operation: "before_seq", Route: "mysql", BeforeSeq: beforeSeq,
			ResultCount: len(page), Latency: time.Since(startedAt),
		})
		return page, err
	}

	startedAt := time.Now()
	observation := ReadObservation{ConversationKey: conversationKey, Operation: "before_seq", BeforeSeq: beforeSeq}
	if beforeSeq == 1 {
		return s.completeBefore(conversationKey, beforeSeq, limit, []*model.Message{}, observation, startedAt)
	}
	highWatermark, err := s.highWatermark.LatestConversationSequence(conversationKey)
	observation.HighWatermark = highWatermark
	if err != nil {
		return s.fallbackBefore(conversationKey, beforeSeq, limit, observation, "high_watermark_error", startedAt)
	}
	lastSeq := highWatermark
	if beforeSeq > 1 && beforeSeq-1 < lastSeq {
		lastSeq = beforeSeq - 1
	}
	if lastSeq == 0 {
		return s.completeBefore(conversationKey, beforeSeq, limit, []*model.Message{}, observation, startedAt)
	}
	firstSeq := uint64(1)
	if uint64(limit) < lastSeq {
		firstSeq = lastSeq - uint64(limit) + 1
	}
	records, err := s.timeline.ListRange(context.Background(), conversationKey, firstSeq, lastSeq)
	if err != nil {
		return s.fallbackBefore(conversationKey, beforeSeq, limit, observation, "cassandra_error", startedAt)
	}
	if !continuous(records, firstSeq, lastSeq) {
		return s.fallbackBefore(conversationKey, beforeSeq, limit, observation, "incomplete_page", startedAt)
	}
	return s.completeBefore(conversationKey, beforeSeq, limit, recordsToMessages(records), observation, startedAt)
}

func (s *MessageStore) completeAfter(conversationKey string, afterSeq uint64, limit int, page []*model.Message, observation ReadObservation, startedAt time.Time) ([]*model.Message, error) {
	return s.completeVerified(page, observation, startedAt, func() ([]*model.Message, error) {
		return s.MessageStore.ListByConversationSeqAfter(conversationKey, afterSeq, limit)
	})
}

func (s *MessageStore) completeBefore(conversationKey string, beforeSeq uint64, limit int, page []*model.Message, observation ReadObservation, startedAt time.Time) ([]*model.Message, error) {
	return s.completeVerified(page, observation, startedAt, func() ([]*model.Message, error) {
		return s.MessageStore.ListByConversationSeqBefore(conversationKey, beforeSeq, limit)
	})
}

func (s *MessageStore) completeVerified(page []*model.Message, observation ReadObservation, startedAt time.Time, mysqlRead func() ([]*model.Message, error)) ([]*model.Message, error) {
	observation.Route = "cassandra"
	if verificationInCohort(observation.ConversationKey, observation.Operation, s.verifyPercentage) {
		mysqlPage, err := mysqlRead()
		switch {
		case err != nil:
			observation.VerificationOutcome = "mysql_error"
		case !equalMessagePages(page, mysqlPage):
			observation.Route = "mysql_fallback"
			observation.FallbackReason = "payload_mismatch"
			observation.VerificationOutcome = "mismatch"
			page = mysqlPage
		default:
			observation.VerificationOutcome = "match"
		}
	}
	observation.ResultCount = len(page)
	observation.Latency = time.Since(startedAt)
	s.record(observation)
	return page, nil
}

func (s *MessageStore) fallback(conversationKey string, afterSeq uint64, limit int, observation ReadObservation, reason string, startedAt time.Time) ([]*model.Message, error) {
	page, err := s.MessageStore.ListByConversationSeqAfter(conversationKey, afterSeq, limit)
	observation.Route = "mysql_fallback"
	observation.FallbackReason = reason
	observation.ResultCount = len(page)
	observation.Latency = time.Since(startedAt)
	s.record(observation)
	return page, err
}

func (s *MessageStore) fallbackBefore(conversationKey string, beforeSeq uint64, limit int, observation ReadObservation, reason string, startedAt time.Time) ([]*model.Message, error) {
	page, err := s.MessageStore.ListByConversationSeqBefore(conversationKey, beforeSeq, limit)
	observation.Route = "mysql_fallback"
	observation.FallbackReason = reason
	observation.ResultCount = len(page)
	observation.Latency = time.Since(startedAt)
	s.record(observation)
	return page, err
}

func (s *MessageStore) record(observation ReadObservation) {
	s.requests.WithLabelValues(observation.Route, observation.FallbackReason).Inc()
	s.latency.WithLabelValues(observation.Route).Observe(observation.Latency.Seconds())
	if observation.VerificationOutcome != "" {
		s.verifications.WithLabelValues(observation.Operation, observation.VerificationOutcome).Inc()
	}
	s.observe(observation)
}

func (s *MessageStore) Describe(descriptions chan<- *prometheus.Desc) {
	s.requests.Describe(descriptions)
	s.latency.Describe(descriptions)
	s.verifications.Describe(descriptions)
}

func (s *MessageStore) Collect(metrics chan<- prometheus.Metric) {
	s.requests.Collect(metrics)
	s.latency.Collect(metrics)
	s.verifications.Collect(metrics)
}

func equalMessagePages(left, right []*model.Message) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if !equalMessage(left[index], right[index]) {
			return false
		}
	}
	return true
}

func equalMessage(left, right *model.Message) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.UUID == right.UUID && left.ClientMessageID == right.ClientMessageID &&
		left.ConversationKey == right.ConversationKey && left.Seq == right.Seq &&
		left.SenderUUID == right.SenderUUID && left.TargetType == right.TargetType &&
		left.TargetUUID == right.TargetUUID && left.MessageType == right.MessageType &&
		left.Content == right.Content && left.FileID == right.FileID &&
		left.FileName == right.FileName && left.FileSize == right.FileSize &&
		left.FileURL == right.FileURL && left.FileContentType == right.FileContentType &&
		equalTimePointer(left.FileExpiresAt, right.FileExpiresAt) && left.SentAt.Equal(right.SentAt)
}

func equalTimePointer(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.Equal(*right)
}

func continuous(records []cassandraData.TimelineRecord, firstSeq, lastSeq uint64) bool {
	if uint64(len(records)) != lastSeq-firstSeq+1 {
		return false
	}
	for index, record := range records {
		if record.Projection.MessageSeq != firstSeq+uint64(index) {
			return false
		}
	}
	return true
}

func recordsToMessages(records []cassandraData.TimelineRecord) []*model.Message {
	messages := make([]*model.Message, len(records))
	for index, record := range records {
		projection := record.Projection
		messages[index] = &model.Message{
			UUID: projection.MessageUUID, ClientMessageID: projection.ClientMessageID,
			ConversationKey: projection.ConversationKey, Seq: projection.MessageSeq,
			SenderUUID: projection.SenderUUID, TargetType: projection.TargetType,
			TargetUUID: projection.TargetUUID, MessageType: projection.MessageType,
			Content: projection.Content, FileID: projection.FileID, FileName: projection.FileName,
			FileSize: projection.FileSize, FileURL: projection.FileURL,
			FileContentType: projection.FileContentType, FileExpiresAt: projection.FileExpiresAt,
			SentAt: projection.SentAt,
		}
	}
	return messages
}

func conversationInCohort(conversationKey string, percentage int) bool {
	if percentage <= 0 {
		return false
	}
	if percentage >= 100 {
		return true
	}
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(conversationKey))
	return int(hash.Sum32()%100) < percentage
}

func verificationInCohort(conversationKey, operation string, percentage int) bool {
	return conversationInCohort(conversationKey+"\x00verify\x00"+operation, percentage)
}

func logReadObservation(observation ReadObservation) {
	fields := []zap.Field{
		zap.String("conversation_key", observation.ConversationKey),
		zap.String("operation", observation.Operation),
		zap.String("route", observation.Route),
		zap.String("fallback_reason", observation.FallbackReason),
		zap.String("verification_outcome", observation.VerificationOutcome),
		zap.Uint64("before_seq", observation.BeforeSeq),
		zap.Uint64("after_seq", observation.AfterSeq),
		zap.Uint64("high_watermark", observation.HighWatermark),
		zap.Int("result_count", observation.ResultCount),
		zap.Duration("latency", observation.Latency),
	}
	if observation.FallbackReason == "" {
		logger.L().Debug("message read routed", fields...)
		return
	}
	logger.L().Warn("Cassandra message read fell back", fields...)
}
