package model

import "testing"

func TestMessagePayloadSHA256IsDeterministicAndBoundarySafe(t *testing.T) {
	message := &Message{
		MessageType: MessageTypeText,
		Content:     "a\x00b",
		FileID:      "c",
	}

	first := MessagePayloadSHA256(message)
	second := MessagePayloadSHA256(message)
	if first != second {
		t.Fatalf("expected deterministic payload hash, got %q and %q", first, second)
	}
	if len(first) != 64 {
		t.Fatalf("expected SHA-256 hex digest, got length %d", len(first))
	}

	boundaryVariant := &Message{
		MessageType: MessageTypeText,
		Content:     "a",
		FileID:      "b\x00c",
	}
	if first == MessagePayloadSHA256(boundaryVariant) {
		t.Fatal("expected length-prefixed fields to prevent boundary collisions")
	}
}
