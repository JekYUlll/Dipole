package cassandra

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
)

type syncTimelineLookupStub struct {
	records map[string]TimelineRecord
	err     error
}

func (s *syncTimelineLookupStub) Lookup(_ context.Context, conversationKey string, sequence uint64) (TimelineRecord, bool, error) {
	if s.err != nil {
		return TimelineRecord{}, false, s.err
	}
	record, ok := s.records[conversationKey]
	return record, ok && record.Projection.MessageSeq == sequence, nil
}

func TestCassandraSyncMessageHydratorUsesLocatorAndPreservesPayload(t *testing.T) {
	sentAt := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	projection := TimelineProjection{ConversationKey: "direct:U1:U2", MessageSeq: 7, MessageUUID: "M7", ClientMessageID: "C7", SenderUUID: "U1", TargetUUID: "U2", Content: "hello", SentAt: sentAt}
	hydrator, err := NewSyncMessageHydrator(&syncTimelineLookupStub{records: map[string]TimelineRecord{projection.ConversationKey: {Projection: projection}}})
	if err != nil {
		t.Fatalf("new Cassandra hydrator: %v", err)
	}
	messages, err := hydrator.Hydrate(context.Background(), []model.SyncMessageLocator{{MessageUUID: "M7", ConversationKey: projection.ConversationKey, MessageSeq: 7}})
	if err != nil {
		t.Fatalf("hydrate Cassandra message: %v", err)
	}
	message := messages["M7"]
	if message == nil || message.Content != "hello" || message.Seq != 7 || !message.SentAt.Equal(sentAt) {
		t.Fatalf("unexpected hydrated message: %+v", message)
	}
}

func TestCassandraSyncMessageHydratorRejectsMissingAndConflictingRecords(t *testing.T) {
	locator := model.SyncMessageLocator{MessageUUID: "M7", ConversationKey: "group:G1", MessageSeq: 7}
	missing, _ := NewSyncMessageHydrator(&syncTimelineLookupStub{records: map[string]TimelineRecord{}})
	if _, err := missing.Hydrate(context.Background(), []model.SyncMessageLocator{locator}); err == nil {
		t.Fatal("expected missing record error")
	}
	conflict, _ := NewSyncMessageHydrator(&syncTimelineLookupStub{records: map[string]TimelineRecord{"group:G1": {Projection: TimelineProjection{ConversationKey: "group:G1", MessageSeq: 7, MessageUUID: "M8"}}}})
	if _, err := conflict.Hydrate(context.Background(), []model.SyncMessageLocator{locator}); err == nil {
		t.Fatal("expected identity conflict")
	}
	failing, _ := NewSyncMessageHydrator(&syncTimelineLookupStub{err: errors.New("unavailable")})
	if _, err := failing.Hydrate(context.Background(), []model.SyncMessageLocator{locator}); err == nil {
		t.Fatal("expected lookup error")
	}
}
