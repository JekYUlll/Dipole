package ai

import (
	"testing"
	"time"
)

func TestGroupReplyAllowed(t *testing.T) {
	t.Parallel()

	if !groupReplyAllowed(nil, "G100") {
		t.Fatal("empty allowlist must admit every group")
	}
	if !groupReplyAllowed([]string{"", " G100 "}, "G100") {
		t.Fatal("allowlist should match after trim")
	}
	if groupReplyAllowed([]string{"G200"}, "G100") {
		t.Fatal("foreign group must be rejected")
	}
}

func TestGroupReplyGateRejectsInflightAndRate(t *testing.T) {
	t.Parallel()

	fixed := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	gate := newGroupReplyGate(1)
	gate.now = func() time.Time { return fixed }

	release, ok := gate.TryEnter("G100")
	if !ok {
		t.Fatal("first enter should succeed")
	}
	if _, ok := gate.TryEnter("G100"); ok {
		t.Fatal("inflight group must be rejected")
	}
	release()

	if _, ok := gate.TryEnter("G100"); ok {
		t.Fatal("rate limit 1/min must reject the second attempt")
	}
	if _, ok := gate.TryEnter("G200"); !ok {
		t.Fatal("a different group should still enter")
	}

	gate.now = func() time.Time { return fixed.Add(61 * time.Second) }
	if _, ok := gate.TryEnter("G100"); !ok {
		t.Fatal("window expiry should admit the group again")
	}
}

func TestResolveGroupReplyFallback(t *testing.T) {
	t.Parallel()

	if got := resolveGroupReplyFallback("  "); got != defaultGroupReplyFallback {
		t.Fatalf("empty config should use default, got %q", got)
	}
	if got := resolveGroupReplyFallback("稍后再试"); got != "稍后再试" {
		t.Fatalf("configured fallback = %q", got)
	}
}
