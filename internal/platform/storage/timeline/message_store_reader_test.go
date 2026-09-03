package timeline

import (
	"context"
	"testing"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
)

type messageStore struct {
	application.MessageStore
	afterSeq uint64
	limit    int
	page     []*model.Message
}

func (s *messageStore) ListByConversationSeqAfter(_ string, afterSeq uint64, limit int) ([]*model.Message, error) {
	s.afterSeq, s.limit = afterSeq, limit
	return s.page, nil
}

func TestMessageStoreReaderUsesInclusiveSeqRange(t *testing.T) {
	store := &messageStore{page: []*model.Message{{Seq: 8}, {Seq: 9}}}
	reader, err := NewMessageStoreReader(store)
	if err != nil {
		t.Fatalf("new reader: %v", err)
	}

	page, err := reader.ListConversationRange(context.Background(), "group:G1", 8, 9)
	if err != nil || len(page) != 2 || store.afterSeq != 7 || store.limit != 2 {
		t.Fatalf("unexpected SQLC range mapping: page=%+v after=%d limit=%d err=%v", page, store.afterSeq, store.limit, err)
	}
}

func TestMessageStoreReaderRejectsInvalidRange(t *testing.T) {
	reader, err := NewMessageStoreReader(&messageStore{})
	if err != nil {
		t.Fatalf("new reader: %v", err)
	}
	if _, err := reader.ListConversationRange(context.Background(), "group:G1", 0, 1); err == nil {
		t.Fatal("expected invalid range error")
	}
}
