package delivery

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeCutoverAttemptExecutor struct {
	now       time.Time
	fail      map[CutoverAttemptEventType]error
	actions   []CutoverAttemptAction
	artifacts map[string]string
}

func (f *fakeCutoverAttemptExecutor) Execute(_ context.Context, action CutoverAttemptAction) (CutoverAttemptActionResult, error) {
	f.actions = append(f.actions, action)
	if err := f.fail[action.EventType]; err != nil {
		return CutoverAttemptActionResult{}, err
	}
	if f.artifacts == nil {
		f.artifacts = make(map[string]string)
	}
	digest, exists := f.artifacts[action.ActionID]
	if !exists {
		digest = hashBytes([]byte(action.ActionID))
		f.artifacts[action.ActionID] = digest
	}
	return CutoverAttemptActionResult{ArtifactSHA256: digest, RecordedAtUnixMS: f.now.UnixMilli()}, nil
}

func TestCutoverAttemptOrchestratorCompletesOneDurableActionAtATime(t *testing.T) {
	now := time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)
	journal, err := CreateCutoverAttemptJournal(t.TempDir(), validCutoverAttemptManifest(now))
	if err != nil {
		t.Fatal(err)
	}
	executor := &fakeCutoverAttemptExecutor{now: now, fail: make(map[CutoverAttemptEventType]error)}
	orchestrator, err := NewCutoverAttemptOrchestrator(journal, executor, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	for index, want := range []CutoverAttemptEventType{
		CutoverEventSourceCheckpointed,
		CutoverEventFreezeApplied,
		CutoverEventFrozenConfirmed,
		CutoverEventTargetActivated,
		CutoverEventTargetCheckpointed,
		CutoverEventCompleted,
	} {
		now = now.Add(time.Second)
		executor.now = now
		result, err := orchestrator.Advance(context.Background())
		if err != nil {
			t.Fatalf("Advance(%d): %v", index, err)
		}
		if result.EventType != want || result.Sequence != uint64(index+1) {
			t.Fatalf("Advance(%d) = %+v, want %s", index, result, want)
		}
	}
	result, err := orchestrator.Advance(context.Background())
	if err != nil || !result.Terminal || result.State != CutoverAttemptCompleted || len(executor.actions) != 6 {
		t.Fatalf("terminal Advance() = %+v, err=%v, actions=%d", result, err, len(executor.actions))
	}
}

func TestCutoverAttemptOrchestratorResumesWithDeterministicActionID(t *testing.T) {
	now := time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)
	directory := t.TempDir()
	journal, err := CreateCutoverAttemptJournal(directory, validCutoverAttemptManifest(now))
	if err != nil {
		t.Fatal(err)
	}
	executor := &fakeCutoverAttemptExecutor{now: now, fail: map[CutoverAttemptEventType]error{
		CutoverEventSourceCheckpointed: errors.New("temporary checkpoint failure"),
	}}
	orchestrator, err := NewCutoverAttemptOrchestrator(journal, executor, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := orchestrator.Advance(context.Background()); err == nil {
		t.Fatal("executor failure must be returned")
	}
	delete(executor.fail, CutoverEventSourceCheckpointed)
	reloaded, err := LoadCutoverAttemptJournal(directory)
	if err != nil {
		t.Fatal(err)
	}
	orchestrator, err = NewCutoverAttemptOrchestrator(reloaded, executor, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := orchestrator.Advance(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(executor.actions) != 2 || executor.actions[0].ActionID != executor.actions[1].ActionID || reloaded.Projection.LastSequence != 1 {
		t.Fatalf("retry actions = %+v, projection = %+v", executor.actions, reloaded.Projection)
	}
}

func TestCutoverAttemptOrchestratorAutomaticallyRollsBackExpiredFreeze(t *testing.T) {
	created := time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)
	manifest := validCutoverAttemptManifest(created)
	manifest.MaxInterruptionMS = 5_000
	journal, err := CreateCutoverAttemptJournal(t.TempDir(), manifest)
	if err != nil {
		t.Fatal(err)
	}
	for index, eventType := range []CutoverAttemptEventType{
		CutoverEventSourceCheckpointed,
		CutoverEventFreezeApplied,
		CutoverEventFrozenConfirmed,
	} {
		if _, err := journal.Append(eventType, strings.Repeat("a", 64), created.Add(time.Duration(index)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	now := created.Add(7 * time.Second)
	executor := &fakeCutoverAttemptExecutor{now: now, fail: make(map[CutoverAttemptEventType]error)}
	orchestrator, err := NewCutoverAttemptOrchestrator(journal, executor, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	result, err := orchestrator.Advance(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.EventType != CutoverEventRollbackRequested || !result.RollbackTriggered || result.State != CutoverAttemptRollbackRequested {
		t.Fatalf("expired freeze result = %+v", result)
	}
	if len(executor.actions) != 1 || executor.actions[0].EventType != CutoverEventRollbackRequested {
		t.Fatalf("expired freeze actions = %+v", executor.actions)
	}
	result, err = orchestrator.Advance(context.Background())
	if err != nil || result.EventType != CutoverEventSourceReactivated {
		t.Fatalf("rollback continuation = %+v, err=%v", result, err)
	}
}

func TestCutoverAttemptOrchestratorRequestsSecondFreezeAfterTargetActivation(t *testing.T) {
	now := time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)
	journal, err := CreateCutoverAttemptJournal(t.TempDir(), validCutoverAttemptManifest(now))
	if err != nil {
		t.Fatal(err)
	}
	for index, eventType := range []CutoverAttemptEventType{
		CutoverEventSourceCheckpointed,
		CutoverEventFreezeApplied,
		CutoverEventFrozenConfirmed,
		CutoverEventTargetActivated,
	} {
		if _, err := journal.Append(eventType, strings.Repeat("a", 64), now.Add(time.Duration(index)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	executor := &fakeCutoverAttemptExecutor{now: now.Add(5 * time.Second), fail: make(map[CutoverAttemptEventType]error)}
	orchestrator, err := NewCutoverAttemptOrchestrator(journal, executor, func() time.Time { return executor.now })
	if err != nil {
		t.Fatal(err)
	}
	result, err := orchestrator.RequestRollback(context.Background())
	if err != nil || result.EventType != CutoverEventRollbackRequested {
		t.Fatalf("RequestRollback() = %+v, err=%v", result, err)
	}
	result, err = orchestrator.Advance(context.Background())
	if err != nil || result.EventType != CutoverEventRollbackFreezeApplied {
		t.Fatalf("rollback Advance() = %+v, err=%v", result, err)
	}
	for _, eventType := range []CutoverAttemptEventType{
		CutoverEventRollbackFrozenConfirmed,
		CutoverEventSourceReactivated,
		CutoverEventRollbackCheckpointed,
		CutoverEventRolledBack,
	} {
		executor.now = executor.now.Add(time.Second)
		result, err = orchestrator.Advance(context.Background())
		if err != nil || result.EventType != eventType {
			t.Fatalf("rollback Advance(%s) = %+v, err=%v", eventType, result, err)
		}
		if got := executor.actions[len(executor.actions)-1].ExpectedEpoch; got != 3 {
			t.Fatalf("rollback action %s epoch = %d, want 3", eventType, got)
		}
	}
}

func TestCutoverAttemptOrchestratorLeavesStateOnActionFailure(t *testing.T) {
	now := time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)
	journal, err := CreateCutoverAttemptJournal(t.TempDir(), validCutoverAttemptManifest(now))
	if err != nil {
		t.Fatal(err)
	}
	executor := &fakeCutoverAttemptExecutor{now: now, fail: map[CutoverAttemptEventType]error{
		CutoverEventSourceCheckpointed: errors.New("checkpoint unavailable"),
	}}
	orchestrator, err := NewCutoverAttemptOrchestrator(journal, executor, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := orchestrator.Advance(context.Background()); err == nil {
		t.Fatal("action failure must be returned")
	}
	if journal.Projection.State != CutoverAttemptCreated || journal.Projection.LastSequence != 0 {
		t.Fatalf("failed action mutated journal: %+v", journal.Projection)
	}
}
