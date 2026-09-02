package shadow

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	cassandraData "github.com/JekYUlll/Dipole/internal/platform/cassandra"
)

type primaryMessageStore struct {
	application.MessageStore
	page         []*model.Message
	err          error
	offlineCalls int
	writeCalls   int
}

func (s *primaryMessageStore) ListByConversationKey(string, uint, int) ([]*model.Message, error) {
	return s.page, s.err
}

func (s *primaryMessageStore) ListByConversationKeyAfter(string, uint, int) ([]*model.Message, error) {
	return s.page, s.err
}

func (s *primaryMessageStore) ListByConversationSeqBefore(string, uint64, int) ([]*model.Message, error) {
	return s.page, s.err
}

func (s *primaryMessageStore) ListByConversationSeqAfter(string, uint64, int) ([]*model.Message, error) {
	return s.page, s.err
}

func (s *primaryMessageStore) ListOfflineByUserUUID(string, uint, int) ([]*model.Message, error) {
	s.offlineCalls++
	return s.page, s.err
}

func (s *primaryMessageStore) CreateWithSync(*model.Message, []string) error {
	s.writeCalls++
	return nil
}

func (s *primaryMessageStore) GetByUUID(string) (*model.Message, error) {
	if len(s.page) == 0 {
		return nil, s.err
	}
	return s.page[0], s.err
}

func (s *primaryMessageStore) GetBySenderAndClientMessageID(string, string) (*model.Message, error) {
	return s.GetByUUID("")
}

type timelineRangeReader struct {
	mu       sync.Mutex
	records  []cassandraData.TimelineRecord
	err      error
	calls    int
	key      string
	firstSeq uint64
	lastSeq  uint64
}

type blockingTimelineRangeReader struct {
	started chan struct{}
	release chan struct{}
	record  cassandraData.TimelineRecord
}

func (r *blockingTimelineRangeReader) ListRange(context.Context, string, uint64, uint64) ([]cassandraData.TimelineRecord, error) {
	close(r.started)
	<-r.release
	return []cassandraData.TimelineRecord{r.record}, nil
}

func (r *timelineRangeReader) ListRange(_ context.Context, key string, firstSeq, lastSeq uint64) ([]cassandraData.TimelineRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	r.key, r.firstSeq, r.lastSeq = key, firstSeq, lastSeq
	return r.records, r.err
}

func (r *timelineRangeReader) snapshot() (int, string, uint64, uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls, r.key, r.firstSeq, r.lastSeq
}

func TestMessageStoreReturnsPrimaryPageAndComparesExactSequenceRange(t *testing.T) {
	primaryPage := []*model.Message{
		message(41, 8, "M8", "first"),
		message(42, 9, "M9", "second"),
	}
	primary := &primaryMessageStore{page: primaryPage}
	timeline := &timelineRangeReader{records: []cassandraData.TimelineRecord{
		record(primaryPage[0]),
		record(primaryPage[1]),
	}}
	comparisons := make(chan MessageComparison, 1)
	store := NewMessageStore(primary, timeline, func(comparison MessageComparison) { comparisons <- comparison })

	got, err := store.ListByConversationKey("group:G1", 90, 20)
	if err != nil || len(got) != 2 || got[0].ID != 41 || got[1].ID != 42 {
		t.Fatalf("primary page changed: messages=%+v err=%v", got, err)
	}
	store.Wait()
	comparison := <-comparisons
	if !comparison.Match || comparison.Skipped || comparison.FirstSeq != 8 || comparison.LastSeq != 9 {
		t.Fatalf("unexpected comparison: %+v", comparison)
	}
	calls, key, firstSeq, lastSeq := timeline.snapshot()
	if calls != 1 || key != "group:G1" || firstSeq != 8 || lastSeq != 9 {
		t.Fatalf("unexpected timeline query: calls=%d key=%s range=%d..%d", calls, key, firstSeq, lastSeq)
	}
}

func TestMessageStoreReportsMismatchWithoutChangingPrimaryResult(t *testing.T) {
	primaryPage := []*model.Message{message(7, 3, "M3", "primary")}
	shadowMessage := *primaryPage[0]
	shadowMessage.Content = "different"
	comparisons := make(chan MessageComparison, 1)
	store := NewMessageStore(
		&primaryMessageStore{page: primaryPage},
		&timelineRangeReader{records: []cassandraData.TimelineRecord{record(&shadowMessage)}},
		func(comparison MessageComparison) { comparisons <- comparison },
	)

	got, err := store.ListByConversationSeqAfter("group:G1", 2, 20)
	if err != nil || got[0].Content != "primary" {
		t.Fatalf("primary response changed: messages=%+v err=%v", got, err)
	}
	store.Wait()
	comparison := <-comparisons
	if comparison.Match || comparison.Skipped || comparison.Operation != "list_conversation_seq_after" {
		t.Fatalf("expected payload mismatch, got %+v", comparison)
	}
}

func TestMessageStoreSkipsEmptyAndFailedPrimaryPages(t *testing.T) {
	tests := []struct {
		name   string
		page   []*model.Message
		err    error
		reason string
	}{
		{name: "empty", page: []*model.Message{}, reason: "empty_primary_page"},
		{name: "failed", err: errors.New("mysql unavailable"), reason: "primary_query_failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			timeline := &timelineRangeReader{}
			comparisons := make(chan MessageComparison, 1)
			store := NewMessageStore(&primaryMessageStore{page: tt.page, err: tt.err}, timeline, func(comparison MessageComparison) { comparisons <- comparison })
			_, _ = store.ListByConversationSeqBefore("group:G1", 10, 20)
			store.Wait()
			comparison := <-comparisons
			if !comparison.Skipped || comparison.SkipReason != tt.reason || comparison.Match {
				t.Fatalf("unexpected skipped comparison: %+v", comparison)
			}
			if calls, _, _, _ := timeline.snapshot(); calls != 0 {
				t.Fatalf("timeline should not be queried, calls=%d", calls)
			}
		})
	}
}

func TestMessageStoreSkipsInvalidPrimarySequence(t *testing.T) {
	timeline := &timelineRangeReader{}
	comparisons := make(chan MessageComparison, 1)
	store := NewMessageStore(
		&primaryMessageStore{page: []*model.Message{{ConversationKey: "group:G1"}}},
		timeline,
		func(comparison MessageComparison) { comparisons <- comparison },
	)
	_, _ = store.ListByConversationKey("group:G1", 0, 20)
	comparison := <-comparisons
	if !comparison.Skipped || comparison.SkipReason != "invalid_primary_sequence" {
		t.Fatalf("unexpected invalid-sequence comparison: %+v", comparison)
	}
	if calls, _, _, _ := timeline.snapshot(); calls != 0 {
		t.Fatalf("timeline should not receive invalid sequence, calls=%d", calls)
	}
}

func TestMessageStoreDropsShadowWorkWhenCapacityIsExhausted(t *testing.T) {
	primaryPage := []*model.Message{message(1, 1, "M1", "one")}
	timeline := &blockingTimelineRangeReader{
		started: make(chan struct{}), release: make(chan struct{}), record: record(primaryPage[0]),
	}
	comparisons := make(chan MessageComparison, 2)
	store := newMessageStore(
		&primaryMessageStore{page: primaryPage}, timeline,
		func(comparison MessageComparison) { comparisons <- comparison }, 1,
	)

	_, _ = store.ListByConversationKey("group:G1", 0, 20)
	<-timeline.started
	_, _ = store.ListByConversationKey("group:G1", 0, 20)
	capacityComparison := <-comparisons
	if !capacityComparison.Skipped || capacityComparison.SkipReason != "shadow_capacity_exhausted" {
		t.Fatalf("expected capacity skip, got %+v", capacityComparison)
	}
	close(timeline.release)
	store.Wait()
	completedComparison := <-comparisons
	if !completedComparison.Match || completedComparison.Skipped {
		t.Fatalf("expected the admitted comparison to complete, got %+v", completedComparison)
	}
}

func TestMessageStoreKeepsOfflineAndWritesOnMySQLOnly(t *testing.T) {
	primary := &primaryMessageStore{page: []*model.Message{message(1, 1, "M1", "one")}}
	timeline := &timelineRangeReader{}
	store := NewMessageStore(primary, timeline, nil)

	if err := store.CreateWithSync(primary.page[0], []string{"U2"}); err != nil {
		t.Fatalf("forward create: %v", err)
	}
	if _, err := store.ListOfflineByUserUUID("U2", 0, 20); err != nil {
		t.Fatalf("forward offline query: %v", err)
	}
	store.Wait()
	if primary.writeCalls != 1 || primary.offlineCalls != 1 {
		t.Fatalf("unexpected primary calls: writes=%d offline=%d", primary.writeCalls, primary.offlineCalls)
	}
	if calls, _, _, _ := timeline.snapshot(); calls != 0 {
		t.Fatalf("timeline should not receive writes or Inbox queries, calls=%d", calls)
	}
}

func TestMessageStorePreservesMetadataContractThroughShadowDecorator(t *testing.T) {
	message := message(1, 7, "M7", "payload")
	store := NewMessageStore(&primaryMessageStore{page: []*model.Message{message}}, &timelineRangeReader{}, nil)

	byUUID, err := store.GetMetadataByUUID("M7")
	if err != nil || byUUID == nil || byUUID.MessageUUID != "M7" || byUUID.MessageSeq != 7 {
		t.Fatalf("metadata by UUID: metadata=%+v err=%v", byUUID, err)
	}
	byClient, err := store.GetMetadataBySenderAndClientMessageID(message.SenderUUID, message.ClientMessageID)
	if err != nil || byClient == nil || byClient.PayloadSHA256 != byUUID.PayloadSHA256 {
		t.Fatalf("metadata by client ID: metadata=%+v err=%v", byClient, err)
	}
}

func message(id uint, seq uint64, uuid, content string) *model.Message {
	return &model.Message{
		ID: id, UUID: uuid, ClientMessageID: "C-" + uuid,
		ConversationKey: "group:G1", Seq: seq, SenderUUID: "U1",
		TargetType: model.MessageTargetGroup, TargetUUID: "G1",
		MessageType: model.MessageTypeText, Content: content,
		SentAt: time.Date(2026, 8, 27, 10, int(seq), 0, 0, time.UTC),
	}
}

func record(message *model.Message) cassandraData.TimelineRecord {
	return cassandraData.TimelineRecord{Projection: cassandraData.TimelineProjection{
		ConversationKey: message.ConversationKey, MessageSeq: message.Seq,
		MessageUUID: message.UUID, ClientMessageID: message.ClientMessageID,
		SenderUUID: message.SenderUUID, TargetType: message.TargetType, TargetUUID: message.TargetUUID,
		MessageType: message.MessageType, Content: message.Content,
		FileID: message.FileID, FileName: message.FileName, FileSize: message.FileSize,
		FileURL: message.FileURL, FileContentType: message.FileContentType,
		FileExpiresAt: message.FileExpiresAt, SentAt: message.SentAt,
	}}
}
