package shadow

import (
	"context"
	"sync"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	cassandraData "github.com/JekYUlll/Dipole/internal/data/cassandra"
	"github.com/JekYUlll/Dipole/internal/logger"
	"github.com/JekYUlll/Dipole/internal/model"
	"go.uber.org/zap"
)

type TimelineRangeReader interface {
	ListRange(ctx context.Context, conversationKey string, firstSeq, lastSeq uint64) ([]cassandraData.TimelineRecord, error)
}

type MessageComparison struct {
	Operation    string
	Match        bool
	Skipped      bool
	SkipReason   string
	Conversation string
	FirstSeq     uint64
	LastSeq      uint64
	PrimaryCount int
	ShadowCount  int
	ShadowError  string
}

type MessageStore struct {
	primary  application.MessageStore
	timeline TimelineRangeReader
	observe  func(MessageComparison)
	slots    chan struct{}
	work     sync.WaitGroup
}

const defaultMaxConcurrentComparisons = 32

var _ application.MessageStore = (*MessageStore)(nil)

func NewMessageStore(primary application.MessageStore, timeline TimelineRangeReader, observe func(MessageComparison)) *MessageStore {
	return newMessageStore(primary, timeline, observe, defaultMaxConcurrentComparisons)
}

func newMessageStore(primary application.MessageStore, timeline TimelineRangeReader, observe func(MessageComparison), maxConcurrent int) *MessageStore {
	if observe == nil {
		observe = logComparison
	}
	if maxConcurrent <= 0 {
		maxConcurrent = defaultMaxConcurrentComparisons
	}
	return &MessageStore{
		primary: primary, timeline: timeline, observe: observe,
		slots: make(chan struct{}, maxConcurrent),
	}
}

func (s *MessageStore) CreateWithSync(message *model.Message, recipients []string) error {
	return s.primary.CreateWithSync(message, recipients)
}

func (s *MessageStore) StoreWithOutboxAndSync(message *model.Message, buildOutbox application.MessageOutboxBuilder, recipients []string) error {
	return s.primary.StoreWithOutboxAndSync(message, buildOutbox, recipients)
}

func (s *MessageStore) EnsureOutbox(event *model.OutboxEvent) error {
	return s.primary.EnsureOutbox(event)
}

func (s *MessageStore) EnsureSyncInbox(message *model.Message, recipients []string) error {
	return s.primary.EnsureSyncInbox(message, recipients)
}

func (s *MessageStore) GetByUUID(uuid string) (*model.Message, error) {
	return s.primary.GetByUUID(uuid)
}

func (s *MessageStore) GetBySenderAndClientMessageID(senderUUID, clientMessageID string) (*model.Message, error) {
	return s.primary.GetBySenderAndClientMessageID(senderUUID, clientMessageID)
}

func (s *MessageStore) HasConversationMessages(conversationKey string) (bool, error) {
	return s.primary.HasConversationMessages(conversationKey)
}

func (s *MessageStore) ListByConversationKey(conversationKey string, beforeID uint, limit int) ([]*model.Message, error) {
	page, err := s.primary.ListByConversationKey(conversationKey, beforeID, limit)
	s.compare("list_conversation_before_id", conversationKey, page, err)
	return page, err
}

func (s *MessageStore) ListByConversationKeyAfter(conversationKey string, afterID uint, limit int) ([]*model.Message, error) {
	page, err := s.primary.ListByConversationKeyAfter(conversationKey, afterID, limit)
	s.compare("list_conversation_after_id", conversationKey, page, err)
	return page, err
}

func (s *MessageStore) ListByConversationSeqBefore(conversationKey string, beforeSeq uint64, limit int) ([]*model.Message, error) {
	page, err := s.primary.ListByConversationSeqBefore(conversationKey, beforeSeq, limit)
	s.compare("list_conversation_seq_before", conversationKey, page, err)
	return page, err
}

func (s *MessageStore) ListByConversationSeqAfter(conversationKey string, afterSeq uint64, limit int) ([]*model.Message, error) {
	page, err := s.primary.ListByConversationSeqAfter(conversationKey, afterSeq, limit)
	s.compare("list_conversation_seq_after", conversationKey, page, err)
	return page, err
}

func (s *MessageStore) ListOfflineByUserUUID(userUUID string, afterID uint, limit int) ([]*model.Message, error) {
	return s.primary.ListOfflineByUserUUID(userUUID, afterID, limit)
}

func (s *MessageStore) FindLatestAccessibleFileMessage(fileUUID, userUUID string) (*model.Message, error) {
	return s.primary.FindLatestAccessibleFileMessage(fileUUID, userUUID)
}

func (s *MessageStore) compare(operation, conversationKey string, page []*model.Message, primaryErr error) {
	comparison := MessageComparison{
		Operation: operation, Conversation: conversationKey, PrimaryCount: len(page),
	}
	if primaryErr != nil {
		comparison.Skipped = true
		comparison.SkipReason = "primary_query_failed"
		s.observe(comparison)
		return
	}
	if len(page) == 0 {
		comparison.Skipped = true
		comparison.SkipReason = "empty_primary_page"
		s.observe(comparison)
		return
	}

	snapshot := cloneMessages(page)
	var comparable bool
	comparison.FirstSeq, comparison.LastSeq, comparable = sequenceBounds(snapshot)
	if !comparable {
		comparison.Skipped = true
		comparison.SkipReason = "invalid_primary_sequence"
		s.observe(comparison)
		return
	}
	select {
	case s.slots <- struct{}{}:
	default:
		comparison.Skipped = true
		comparison.SkipReason = "shadow_capacity_exhausted"
		s.observe(comparison)
		return
	}
	s.work.Add(1)
	go func() {
		defer s.work.Done()
		defer func() { <-s.slots }()
		records, err := s.timeline.ListRange(context.Background(), conversationKey, comparison.FirstSeq, comparison.LastSeq)
		comparison.ShadowCount = len(records)
		if err != nil {
			comparison.ShadowError = err.Error()
		} else {
			comparison.Match = equalPage(snapshot, records)
		}
		s.observe(comparison)
	}()
}

func (s *MessageStore) Wait() {
	if s != nil {
		s.work.Wait()
	}
}

func cloneMessages(messages []*model.Message) []*model.Message {
	cloned := make([]*model.Message, len(messages))
	for index, message := range messages {
		if message == nil {
			continue
		}
		copy := *message
		if message.FileExpiresAt != nil {
			expiresAt := *message.FileExpiresAt
			copy.FileExpiresAt = &expiresAt
		}
		cloned[index] = &copy
	}
	return cloned
}

func sequenceBounds(messages []*model.Message) (uint64, uint64, bool) {
	if len(messages) == 0 || messages[0] == nil || messages[0].Seq == 0 {
		return 0, 0, false
	}
	first, last := messages[0].Seq, messages[0].Seq
	for _, message := range messages[1:] {
		if message == nil || message.Seq == 0 {
			return 0, 0, false
		}
		if message.Seq < first {
			first = message.Seq
		}
		if message.Seq > last {
			last = message.Seq
		}
	}
	return first, last, true
}

func equalPage(primary []*model.Message, shadow []cassandraData.TimelineRecord) bool {
	if len(primary) != len(shadow) {
		return false
	}
	for index, message := range primary {
		if message == nil || !equalProjection(message, shadow[index].Projection) {
			return false
		}
	}
	return true
}

func equalProjection(message *model.Message, projection cassandraData.TimelineProjection) bool {
	return message.UUID == projection.MessageUUID &&
		message.ClientMessageID == projection.ClientMessageID &&
		message.ConversationKey == projection.ConversationKey &&
		message.Seq == projection.MessageSeq &&
		message.SenderUUID == projection.SenderUUID &&
		message.TargetType == projection.TargetType &&
		message.TargetUUID == projection.TargetUUID &&
		message.MessageType == projection.MessageType &&
		message.Content == projection.Content &&
		message.FileID == projection.FileID &&
		message.FileName == projection.FileName &&
		message.FileSize == projection.FileSize &&
		message.FileURL == projection.FileURL &&
		message.FileContentType == projection.FileContentType &&
		equalOptionalTime(message.FileExpiresAt, projection.FileExpiresAt) &&
		message.SentAt.Equal(projection.SentAt)
}

func equalOptionalTime(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.Equal(*right)
}

func logComparison(comparison MessageComparison) {
	fields := []zap.Field{
		zap.String("operation", comparison.Operation),
		zap.Bool("match", comparison.Match),
		zap.Bool("skipped", comparison.Skipped),
		zap.String("skip_reason", comparison.SkipReason),
		zap.String("conversation_key", comparison.Conversation),
		zap.Uint64("first_seq", comparison.FirstSeq),
		zap.Uint64("last_seq", comparison.LastSeq),
		zap.Int("primary_count", comparison.PrimaryCount),
		zap.Int("shadow_count", comparison.ShadowCount),
		zap.String("shadow_error", comparison.ShadowError),
	}
	if comparison.Skipped || comparison.Match {
		logger.L().Debug("Cassandra message shadow query observed", fields...)
		return
	}
	logger.L().Warn("Cassandra message shadow query mismatch", fields...)
}
