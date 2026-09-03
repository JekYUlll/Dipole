package shadow

import (
	"context"
	"sync"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/logger"
	"github.com/JekYUlll/Dipole/internal/model"
	"go.uber.org/zap"
)

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
	timeline application.ConversationTimelineReader
	observe  func(MessageComparison)
	slots    chan struct{}
	work     sync.WaitGroup
}

const defaultMaxConcurrentComparisons = 32

var _ application.MessageStore = (*MessageStore)(nil)
var _ application.MessageMetadataStore = (*MessageStore)(nil)

func NewMessageStore(primary application.MessageStore, timeline application.ConversationTimelineReader, observe func(MessageComparison)) *MessageStore {
	return newMessageStore(primary, timeline, observe, defaultMaxConcurrentComparisons)
}

func newMessageStore(primary application.MessageStore, timeline application.ConversationTimelineReader, observe func(MessageComparison), maxConcurrent int) *MessageStore {
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

func (s *MessageStore) GetMetadataByUUID(uuid string) (*model.MessageMetadata, error) {
	if store, ok := s.primary.(application.MessageMetadataStore); ok {
		return store.GetMetadataByUUID(uuid)
	}
	message, err := s.primary.GetByUUID(uuid)
	return model.MetadataFromMessage(message), err
}

func (s *MessageStore) GetMetadataBySenderAndClientMessageID(senderUUID, clientMessageID string) (*model.MessageMetadata, error) {
	if store, ok := s.primary.(application.MessageMetadataStore); ok {
		return store.GetMetadataBySenderAndClientMessageID(senderUUID, clientMessageID)
	}
	message, err := s.primary.GetBySenderAndClientMessageID(senderUUID, clientMessageID)
	return model.MetadataFromMessage(message), err
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
		shadowPage, err := s.timeline.ListConversationRange(context.Background(), conversationKey, comparison.FirstSeq, comparison.LastSeq)
		comparison.ShadowCount = len(shadowPage)
		if err != nil {
			comparison.ShadowError = err.Error()
		} else {
			comparison.Match = equalPage(snapshot, shadowPage)
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

func equalPage(primary, shadow []*model.Message) bool {
	if len(primary) != len(shadow) {
		return false
	}
	for index, message := range primary {
		if message == nil || shadow[index] == nil || !equalMessage(message, shadow[index]) {
			return false
		}
	}
	return true
}

func equalMessage(left, right *model.Message) bool {
	return left.UUID == right.UUID &&
		left.ClientMessageID == right.ClientMessageID &&
		left.ConversationKey == right.ConversationKey &&
		left.Seq == right.Seq &&
		left.SenderUUID == right.SenderUUID &&
		left.TargetType == right.TargetType &&
		left.TargetUUID == right.TargetUUID &&
		left.MessageType == right.MessageType &&
		left.Content == right.Content &&
		left.FileID == right.FileID &&
		left.FileName == right.FileName &&
		left.FileSize == right.FileSize &&
		left.FileURL == right.FileURL &&
		left.FileContentType == right.FileContentType &&
		equalOptionalTime(left.FileExpiresAt, right.FileExpiresAt) &&
		left.SentAt.Equal(right.SentAt)
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
