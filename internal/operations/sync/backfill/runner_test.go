package syncbackfill

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
)

type sourceStub struct {
	highWatermark uint64
	items         []SourceItem
}

func (s *sourceStub) HighWatermark(context.Context) (uint64, error) { return s.highWatermark, nil }
func (s *sourceStub) ListAfter(_ context.Context, afterID, throughID uint64, limit int) ([]SourceItem, error) {
	result := make([]SourceItem, 0, limit)
	for _, item := range s.items {
		if item.SourceID > afterID && item.SourceID <= throughID {
			result = append(result, item)
			if len(result) == limit {
				break
			}
		}
	}
	return result, nil
}

type checkpointStub struct {
	state      Checkpoint
	advances   []uint64
	failed     error
	completed  bool
	advanceErr error
}

func (s *checkpointStub) Acquire(ctx context.Context, jobName string, ownerID string, highWatermark uint64, lease time.Duration) (Checkpoint, error) {
	_, _, _, _ = ctx, jobName, ownerID, lease
	if s.state.HighWatermarkID == 0 {
		s.state.HighWatermarkID = highWatermark
	}
	s.state.Status = StatusRunning
	return s.state, nil
}
func (s *checkpointStub) Advance(ctx context.Context, jobName string, ownerID string, sourceID uint64, lease time.Duration) error {
	_, _, _, _ = ctx, jobName, ownerID, lease
	if s.advanceErr != nil {
		return s.advanceErr
	}
	s.advances = append(s.advances, sourceID)
	return nil
}
func (s *checkpointStub) Fail(ctx context.Context, jobName string, ownerID string, cause error) error {
	_, _, _ = ctx, jobName, ownerID
	s.failed = cause
	return nil
}
func (s *checkpointStub) Complete(ctx context.Context, jobName string, ownerID string) error {
	_, _, _ = ctx, jobName, ownerID
	s.completed = true
	return nil
}

type targetStub struct {
	items []*model.SyncProjection
	fail  string
}

func (s *targetStub) Apply(item *model.SyncProjection) error {
	if item.MessageUUID == s.fail {
		return errors.New("target unavailable")
	}
	s.items = append(s.items, item)
	return nil
}

func TestRunnerReplaysFixedSnapshotAndSkipsHotGroups(t *testing.T) {
	source := &sourceStub{highWatermark: 5, items: []SourceItem{
		item(1, true), item(3, false), item(5, true), item(7, true),
	}}
	checkpoints := &checkpointStub{}
	target := &targetStub{}
	runner := mustRunner(t, source, checkpoints, target, Config{JobName: "sync-v1", OwnerID: "worker-a", BatchSize: 2, LeaseDuration: time.Minute})

	result, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("run replay: %v", err)
	}
	if result.HighWatermarkID != 5 || result.LastProcessedID != 5 || result.Processed != 3 || result.Projected != 2 || result.Skipped != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if got := fmt.Sprint(checkpoints.advances); got != "[3 5]" || !checkpoints.completed {
		t.Fatalf("advances=%s completed=%v", got, checkpoints.completed)
	}
	if len(target.items) != 2 || target.items[0].EventID != "E-1" || target.items[1].MessageUUID != "M-5" {
		t.Fatalf("unexpected projections: %+v", target.items)
	}
}

func TestRunnerResumesAndDoesNotAdvanceFailedBatch(t *testing.T) {
	source := &sourceStub{highWatermark: 5, items: []SourceItem{item(1, true), item(3, true), item(5, true)}}
	checkpoints := &checkpointStub{state: Checkpoint{HighWatermarkID: 5, LastProcessedID: 1, Status: StatusFailed}}
	target := &targetStub{fail: "M-5"}
	runner := mustRunner(t, source, checkpoints, target, Config{JobName: "sync-v1", OwnerID: "worker-b", BatchSize: 2, LeaseDuration: time.Minute})

	result, err := runner.Run(context.Background())
	if err == nil || checkpoints.failed == nil {
		t.Fatalf("expected recorded replay failure, result=%+v err=%v", result, err)
	}
	if len(checkpoints.advances) != 0 || result.LastProcessedID != 1 {
		t.Fatalf("failed batch advanced: result=%+v advances=%v", result, checkpoints.advances)
	}
}

func TestRunnerRejectsInvalidSourceOrdering(t *testing.T) {
	source := &sourceStub{highWatermark: 3, items: []SourceItem{item(2, true), item(1, true)}}
	checkpoints := &checkpointStub{}
	runner := mustRunner(t, source, checkpoints, &targetStub{}, Config{JobName: "sync-v1", OwnerID: "worker", BatchSize: 3, LeaseDuration: time.Minute})
	if _, err := runner.Run(context.Background()); err == nil || checkpoints.failed == nil {
		t.Fatalf("expected ordering failure, err=%v", err)
	}
}

func item(id uint64, fanout bool) SourceItem {
	return SourceItem{SourceID: id, Fanout: fanout, Projection: &model.SyncProjection{
		EventID: fmt.Sprintf("E-%d", id), MessageUUID: fmt.Sprintf("M-%d", id),
		ConversationKey: "direct:U1:U2", MessageSeq: id, RecipientUUIDs: []string{"U1", "U2"},
	}}
}

func mustRunner(t *testing.T, source Source, checkpoints CheckpointStore, target Target, cfg Config) *Runner {
	t.Helper()
	runner, err := NewRunner(source, checkpoints, target, cfg)
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}
	return runner
}
