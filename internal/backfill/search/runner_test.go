package searchbackfill

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
)

func TestRunnerUsesFixedSnapshotAndAdvancesAfterCompleteBatch(t *testing.T) {
	source := &sourceStub{highWatermark: 3, items: []SourceMutation{
		{SourceID: 1, Mutation: mutation("M1", 1)},
		{SourceID: 2, Mutation: mutation("M2", 1)},
		{SourceID: 3, Mutation: mutation("M3", 2)},
	}}
	checkpoints := &checkpointStub{}
	target := &targetStub{}
	runner, err := NewRunner(source, checkpoints, target, Config{
		JobName: "search-v1", OwnerID: "worker-1", BatchSize: 2, LeaseDuration: time.Minute,
	})
	if err != nil {
		t.Fatalf("create runner: %v", err)
	}

	result, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("run backfill: %v", err)
	}
	if result.HighWatermarkID != 3 || result.LastProcessedID != 3 || result.Processed != 3 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if got := checkpoints.advances; len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Fatalf("expected complete-batch checkpoints [2 3], got %v", got)
	}
	if !checkpoints.completed {
		t.Fatal("expected completed checkpoint")
	}
	if source.highWatermarkCalls != 1 {
		t.Fatalf("expected one fixed high-watermark read, got %d", source.highWatermarkCalls)
	}
}

func TestRunnerDoesNotAdvanceFailedBatchAndCanResume(t *testing.T) {
	source := &sourceStub{highWatermark: 3, items: []SourceMutation{
		{SourceID: 1, Mutation: mutation("M1", 1)},
		{SourceID: 2, Mutation: mutation("M2", 1)},
		{SourceID: 3, Mutation: mutation("M3", 1)},
	}}
	checkpoints := &checkpointStub{}
	target := &targetStub{failMessage: "M2"}
	runner, _ := NewRunner(source, checkpoints, target, Config{
		JobName: "search-v1", OwnerID: "worker-1", BatchSize: 2, LeaseDuration: time.Minute,
	})
	if _, err := runner.Run(context.Background()); err == nil {
		t.Fatal("expected target failure")
	}
	if len(checkpoints.advances) != 0 || checkpoints.lastProcessed != 0 {
		t.Fatalf("failed batch advanced checkpoint: %+v", checkpoints.advances)
	}

	target.failMessage = ""
	result, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("resume backfill: %v", err)
	}
	if result.LastProcessedID != 3 || !checkpoints.completed {
		t.Fatalf("resume did not complete: %+v", result)
	}
}

func mutation(id string, revision uint64) *model.MessageSearchMutation {
	return &model.MessageSearchMutation{Type: model.MessageSearchMutationTombstone, MessageUUID: id, Revision: revision}
}

type sourceStub struct {
	highWatermark      uint64
	highWatermarkCalls int
	items              []SourceMutation
}

func (s *sourceStub) HighWatermark(context.Context) (uint64, error) {
	s.highWatermarkCalls++
	return s.highWatermark, nil
}

func (s *sourceStub) ListAfter(_ context.Context, after, through uint64, limit int) ([]SourceMutation, error) {
	result := make([]SourceMutation, 0, limit)
	for _, item := range s.items {
		if item.SourceID > after && item.SourceID <= through {
			result = append(result, item)
			if len(result) == limit {
				break
			}
		}
	}
	return result, nil
}

type checkpointStub struct {
	highWatermark uint64
	lastProcessed uint64
	status        string
	advances      []uint64
	completed     bool
}

func (s *checkpointStub) Acquire(_ context.Context, _, _ string, highWatermark uint64, _ time.Duration) (Checkpoint, error) {
	if s.highWatermark == 0 {
		s.highWatermark = highWatermark
	}
	return Checkpoint{HighWatermarkID: s.highWatermark, LastProcessedID: s.lastProcessed, Status: s.status}, nil
}
func (s *checkpointStub) Advance(_ context.Context, _, _ string, sourceID uint64, _ time.Duration) error {
	s.lastProcessed = sourceID
	s.advances = append(s.advances, sourceID)
	return nil
}
func (s *checkpointStub) Fail(context.Context, string, string, error) error {
	s.status = StatusFailed
	return nil
}
func (s *checkpointStub) Complete(context.Context, string, string) error {
	s.status, s.completed = StatusCompleted, true
	return nil
}

type targetStub struct{ failMessage string }

func (s *targetStub) Apply(mutation *model.MessageSearchMutation) error {
	if mutation.MessageUUID == s.failMessage {
		return errors.New("injected target failure")
	}
	return nil
}
