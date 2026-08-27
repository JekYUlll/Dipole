package model

import (
	"testing"
	"time"
)

func TestMessageSearchMutationStateNormalizesReplayPayload(t *testing.T) {
	document := &MessageSearchDocument{
		MessageUUID: " M1 ", ConversationKey: " direct:U1:U2 ", MessageSeq: 7,
		SenderUUID: " U1 ", Content: "approved", SentAt: time.Date(2026, 8, 27, 8, 0, 0, 123456789, time.FixedZone("test", 8*60*60)),
	}
	mutation := &MessageSearchMutation{Type: MessageSearchMutationUpsert, MessageUUID: "M1", Revision: 2, Document: document}
	first, err := mutation.State()
	if err != nil {
		t.Fatalf("normalize mutation: %v", err)
	}
	second, err := mutation.State()
	if err != nil {
		t.Fatalf("normalize replay: %v", err)
	}
	if first.PayloadHash != second.PayloadHash || !first.Searchable || first.SentAt == nil {
		t.Fatalf("unexpected normalized state: %+v", first)
	}
	if first.SentAt.Location() != time.UTC || first.SentAt.Nanosecond() != 123000000 {
		t.Fatalf("expected millisecond UTC time, got %s", first.SentAt)
	}

	document.Content = "changed"
	changed, err := mutation.State()
	if err != nil {
		t.Fatalf("normalize changed mutation: %v", err)
	}
	if changed.PayloadHash == first.PayloadHash {
		t.Fatal("expected payload change to alter hash")
	}
}

func TestMessageSearchTombstoneCarriesNoSearchableFields(t *testing.T) {
	state, err := (&MessageSearchMutation{
		Type: MessageSearchMutationTombstone, MessageUUID: "M1", Revision: 3,
	}).State()
	if err != nil {
		t.Fatalf("normalize tombstone: %v", err)
	}
	if state.Searchable || state.SentAt != nil || state.ConversationKey != "" || state.Content != "" || state.PayloadHash == "" {
		t.Fatalf("unexpected tombstone: %+v", state)
	}
}
