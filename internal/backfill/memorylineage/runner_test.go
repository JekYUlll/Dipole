package memorylineage

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRunnerResumesAndConvergesDuplicates(t *testing.T) {
	source := &sourceStub{highWatermark: 3, pages: [][]SourceItem{{
		{SourceID: 2, References: []Reference{{MemoryUUID: "M2", TaskUUID: "T2", Representation: "compact"}}},
		{SourceID: 3, References: []Reference{{MemoryUUID: "M3", TaskUUID: "T3", Representation: "full"}}},
	}}}
	checkpoint := &checkpointStub{state: Checkpoint{HighWatermarkID: 3, LastProcessedID: 1, Status: StatusFailed}}
	target := &targetStub{duplicates: map[string]bool{"M2/T2": true}}
	runner := mustRunner(t, source, checkpoint, target, Config{JobName: "memory-lineage", OwnerID: "worker-a", BatchSize: 2, LeaseDuration: time.Second})

	result, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result != (Result{HighWatermarkID: 3, LastProcessedID: 3, Processed: 2, References: 2, Inserted: 1, Duplicates: 1}) {
		t.Fatalf("result = %+v", result)
	}
	if checkpoint.state.LastProcessedID != 3 || checkpoint.state.Status != StatusCompleted {
		t.Fatalf("checkpoint = %+v", checkpoint.state)
	}
	if len(target.applied) != 2 {
		t.Fatalf("applied = %v", target.applied)
	}
}

func TestRunnerDoesNotAdvanceAfterTargetFailure(t *testing.T) {
	source := &sourceStub{highWatermark: 1, pages: [][]SourceItem{{{SourceID: 1, References: []Reference{{MemoryUUID: "M1", TaskUUID: "T1", Representation: "full"}}}}}}
	checkpoint := &checkpointStub{state: Checkpoint{HighWatermarkID: 1, Status: StatusRunning}}
	target := &targetStub{applyErr: errors.New("representation drift")}
	runner := mustRunner(t, source, checkpoint, target, Config{JobName: "memory-lineage", OwnerID: "worker-a", BatchSize: 1, LeaseDuration: time.Second})

	_, err := runner.Run(context.Background())
	if err == nil || checkpoint.state.LastProcessedID != 0 || checkpoint.state.Status != StatusFailed {
		t.Fatalf("err=%v checkpoint=%+v", err, checkpoint.state)
	}
	if checkpoint.advanced {
		t.Fatal("checkpoint advanced after target failure")
	}
}

func TestRunnerRejectsInvalidSourceAndConfiguration(t *testing.T) {
	if _, err := NewRunner(nil, &checkpointStub{}, &targetStub{}, Config{JobName: "j", OwnerID: "o", BatchSize: 1, LeaseDuration: time.Second}); err == nil {
		t.Fatal("expected nil source error")
	}
	source := &sourceStub{highWatermark: 1, pages: [][]SourceItem{{{SourceID: 1}}}}
	checkpoint := &checkpointStub{state: Checkpoint{HighWatermarkID: 1, Status: StatusRunning}}
	runner := mustRunner(t, source, checkpoint, &targetStub{}, Config{JobName: "memory-lineage", OwnerID: "worker-a", BatchSize: 1, LeaseDuration: time.Second})
	if _, err := runner.Run(context.Background()); err == nil {
		t.Fatal("expected empty references error")
	}
}

type sourceStub struct {
	highWatermark uint64
	pages         [][]SourceItem
	calls         int
}

func (s *sourceStub) HighWatermark(context.Context) (uint64, error) { return s.highWatermark, nil }
func (s *sourceStub) ListAfter(context.Context, uint64, uint64, int) ([]SourceItem, error) {
	page := s.pages[s.calls]
	s.calls++
	return page, nil
}

type checkpointStub struct {
	state    Checkpoint
	advanced bool
	failed   bool
}

func (s *checkpointStub) Acquire(context.Context, string, string, uint64, time.Duration) (Checkpoint, error) {
	return s.state, nil
}
func (s *checkpointStub) Advance(_ context.Context, _ string, _ string, id uint64, _ time.Duration) error {
	s.advanced = true
	s.state.LastProcessedID = id
	return nil
}
func (s *checkpointStub) Fail(context.Context, string, string, error) error {
	s.failed = true
	s.state.Status = StatusFailed
	return nil
}
func (s *checkpointStub) Complete(context.Context, string, string) error {
	s.state.Status = StatusCompleted
	return nil
}

type targetStub struct {
	duplicates map[string]bool
	applied    []string
	applyErr   error
}

func (s *targetStub) Apply(_ context.Context, reference Reference) (bool, bool, error) {
	if s.applyErr != nil {
		return false, false, s.applyErr
	}
	key := reference.MemoryUUID + "/" + reference.TaskUUID
	s.applied = append(s.applied, key)
	if s.duplicates[key] {
		return false, true, nil
	}
	return true, false, nil
}

func mustRunner(t *testing.T, source Source, checkpoint CheckpointStore, target Target, config Config) *Runner {
	t.Helper()
	runner, err := NewRunner(source, checkpoint, target, config)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}
	return runner
}
