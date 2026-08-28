package delivery

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type cutoverArtifactFixture struct {
	ReceiptID string `json:"receipt_id"`
	Epoch     uint64 `json:"epoch"`
}

func TestCutoverActionArtifactStorePublishesAndReplaysIdempotently(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	directory := t.TempDir()
	action := validCutoverArtifactAction(CutoverEventFreezeApplied)
	payload := cutoverArtifactFixture{ReceiptID: "freeze-a", Epoch: 2}

	store, err := NewCutoverActionArtifactStore(directory)
	if err != nil {
		t.Fatal(err)
	}
	first, firstSHA, err := store.Publish(action, payload, now)
	if err != nil {
		t.Fatal(err)
	}
	second, secondSHA, err := store.Publish(action, payload, now)
	if err != nil {
		t.Fatal(err)
	}
	if firstSHA != secondSHA || first.ActionID != second.ActionID || string(first.Payload) != string(second.Payload) ||
		!validSHA256(firstSHA) || first.PayloadSHA256 == "" {
		t.Fatalf("idempotent publish mismatch: first=%+v second=%+v", first, second)
	}
	loaded, loadedSHA, err := store.Load(action)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeCutoverActionArtifactPayload[cutoverArtifactFixture](loaded)
	if err != nil {
		t.Fatal(err)
	}
	if loadedSHA != firstSHA || decoded != payload {
		t.Fatalf("loaded artifact mismatch: artifact=%+v payload=%+v", loaded, decoded)
	}
	info, err := os.Stat(filepath.Join(directory, action.ActionID+cutoverActionArtifactSuffix))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("artifact mode = %o, want 600", info.Mode().Perm())
	}
}

func TestCutoverActionArtifactStoreRejectsActionConflict(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	store, err := NewCutoverActionArtifactStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	action := validCutoverArtifactAction(CutoverEventFreezeApplied)
	if _, _, err := store.Publish(action, cutoverArtifactFixture{ReceiptID: "freeze-a", Epoch: 2}, now); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Publish(action, cutoverArtifactFixture{ReceiptID: "freeze-b", Epoch: 2}, now); err == nil {
		t.Fatal("same action ID with payload drift must fail")
	}
	drifted := action
	drifted.EventType = CutoverEventTargetActivated
	if _, _, err := store.Load(drifted); err == nil {
		t.Fatal("same action ID with action binding drift must fail")
	}
}

func TestCutoverActionArtifactStoreDetectsTampering(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	directory := t.TempDir()
	store, err := NewCutoverActionArtifactStore(directory)
	if err != nil {
		t.Fatal(err)
	}
	action := validCutoverArtifactAction(CutoverEventFreezeApplied)
	if _, _, err := store.Publish(action, cutoverArtifactFixture{ReceiptID: "freeze-a", Epoch: 2}, now); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, action.ActionID+cutoverActionArtifactSuffix)
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tampered := strings.Replace(string(payload), "\"epoch\": 2", "\"epoch\": 3", 1)
	if err := os.WriteFile(path, []byte(tampered), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Load(action); err == nil {
		t.Fatal("payload hash drift must fail")
	}
}

func TestCutoverActionArtifactStoreRejectsInvalidConfigurationAndPayload(t *testing.T) {
	if _, err := NewCutoverActionArtifactStore("."); err == nil {
		t.Fatal("current directory must be rejected")
	}
	store, err := NewCutoverActionArtifactStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	action := validCutoverArtifactAction(CutoverEventFreezeApplied)
	if _, _, err := store.Publish(action, func() {}, time.Now()); err == nil {
		t.Fatal("unsupported payload must fail")
	}
}

func validCutoverArtifactAction(eventType CutoverAttemptEventType) CutoverAttemptAction {
	action := CutoverAttemptAction{
		AttemptID: "attempt-a", Sequence: 2, EventType: eventType,
		CurrentState:    CutoverAttemptSourceCheckpointed,
		SourceAuthority: AuthorityGo, TargetAuthority: AuthorityCPP,
		ExpectedEpoch: 2, MaxInterruptionMS: 60_000,
	}
	action.ActionID, _ = cutoverAttemptActionID(action.AttemptID, action.Sequence, action.EventType)
	return action
}
