package cassandrabackfill

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	cassandradata "github.com/JekYUlll/Dipole/internal/data/cassandra"
	"github.com/JekYUlll/Dipole/internal/model"
)

type sourceStub struct {
	highWatermark uint64
	messages      []SourceMessage
	err           error
}

func (s *sourceStub) HighWatermark(context.Context) (uint64, error) {
	return s.highWatermark, s.err
}

func (s *sourceStub) ListAfter(_ context.Context, afterID, throughID uint64, limit int) ([]SourceMessage, error) {
	if s.err != nil {
		return nil, s.err
	}
	result := make([]SourceMessage, 0, limit)
	for _, message := range s.messages {
		if message.SourceID > afterID && message.SourceID <= throughID {
			result = append(result, message)
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
	failure    error
	completed  bool
	advanceErr error
	acquireErr error
}

func (s *checkpointStub) Acquire(_ context.Context, _ string, _ string, highWatermark uint64, _ time.Duration) (Checkpoint, error) {
	if s.acquireErr != nil {
		return Checkpoint{}, s.acquireErr
	}
	if s.state.HighWatermarkID == 0 {
		s.state.HighWatermarkID = highWatermark
	}
	s.state.Status = StatusRunning
	return s.state, nil
}

func (s *checkpointStub) Advance(_ context.Context, _ string, _ string, sourceID uint64, _ time.Duration) error {
	if s.advanceErr != nil {
		return s.advanceErr
	}
	s.advances = append(s.advances, sourceID)
	s.state.LastProcessedID = sourceID
	return nil
}

func (s *checkpointStub) Fail(_ context.Context, _ string, _ string, cause error) error {
	s.failure = cause
	return nil
}

func (s *checkpointStub) Complete(context.Context, string, string) error {
	s.completed = true
	s.state.Status = StatusCompleted
	return nil
}

type timelineStub struct {
	projections []cassandradata.TimelineProjection
	failUUID    string
}

func (s *timelineStub) Append(_ context.Context, projection cassandradata.TimelineProjection) (cassandradata.AppendResult, error) {
	if projection.MessageUUID == s.failUUID {
		return cassandradata.AppendResult{}, errors.New("Cassandra unavailable")
	}
	s.projections = append(s.projections, projection)
	return cassandradata.AppendResult{Inserted: true}, nil
}

func TestRunnerCopiesFixedSnapshotAndAdvancesPerBatch(t *testing.T) {
	source := &sourceStub{highWatermark: 5, messages: []SourceMessage{
		message(1), message(3), message(5), message(7),
	}}
	checkpoints := &checkpointStub{}
	timeline := &timelineStub{}
	runner := mustRunner(t, source, checkpoints, timeline, Config{JobName: "timeline-v1", OwnerID: "worker-a", BatchSize: 2, LeaseDuration: time.Minute})

	result, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("run backfill: %v", err)
	}
	if result.HighWatermarkID != 5 || result.LastProcessedID != 5 || result.Processed != 3 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if got := fmt.Sprint(checkpoints.advances); got != "[3 5]" {
		t.Fatalf("checkpoint advances = %s", got)
	}
	if !checkpoints.completed || len(timeline.projections) != 3 {
		t.Fatalf("completion=%v projections=%d", checkpoints.completed, len(timeline.projections))
	}
	if timeline.projections[0].EventID != "backfill:M-1" || timeline.projections[0].EventVersion != "v1" {
		t.Fatalf("backfill identity: %+v", timeline.projections[0])
	}
}

func TestRunnerResumesFromCheckpoint(t *testing.T) {
	source := &sourceStub{highWatermark: 9, messages: []SourceMessage{message(1), message(3), message(5), message(7)}}
	checkpoints := &checkpointStub{state: Checkpoint{HighWatermarkID: 5, LastProcessedID: 3, Status: StatusFailed}}
	timeline := &timelineStub{}
	runner := mustRunner(t, source, checkpoints, timeline, Config{JobName: "timeline-v1", OwnerID: "worker-b", BatchSize: 10, LeaseDuration: time.Minute})

	result, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("resume backfill: %v", err)
	}
	if result.HighWatermarkID != 5 || result.Processed != 1 || len(timeline.projections) != 1 || timeline.projections[0].MessageUUID != "M-5" {
		t.Fatalf("resume result=%+v projections=%+v", result, timeline.projections)
	}
}

func TestRunnerDoesNotAdvanceFailedBatch(t *testing.T) {
	source := &sourceStub{highWatermark: 3, messages: []SourceMessage{message(1), message(2), message(3)}}
	checkpoints := &checkpointStub{}
	timeline := &timelineStub{failUUID: "M-2"}
	runner := mustRunner(t, source, checkpoints, timeline, Config{JobName: "timeline-v1", OwnerID: "worker-a", BatchSize: 3, LeaseDuration: time.Minute})

	_, err := runner.Run(context.Background())
	if err == nil || checkpoints.failure == nil {
		t.Fatalf("expected recorded projection failure, err=%v recorded=%v", err, checkpoints.failure)
	}
	if len(checkpoints.advances) != 0 || checkpoints.completed {
		t.Fatalf("failed batch advanced: %+v", checkpoints.advances)
	}
}

func TestRunnerRejectsNonIncreasingSource(t *testing.T) {
	source := &sourceStub{highWatermark: 3, messages: []SourceMessage{message(2), message(1)}}
	checkpoints := &checkpointStub{}
	runner := mustRunner(t, source, checkpoints, &timelineStub{}, Config{JobName: "timeline-v1", OwnerID: "worker-a", BatchSize: 3, LeaseDuration: time.Minute})

	if _, err := runner.Run(context.Background()); err == nil || checkpoints.failure == nil {
		t.Fatalf("expected source ordering failure, err=%v", err)
	}
}

func TestRunnerCompletesEmptySnapshot(t *testing.T) {
	checkpoints := &checkpointStub{}
	runner := mustRunner(t, &sourceStub{}, checkpoints, &timelineStub{}, Config{JobName: "timeline-v1", OwnerID: "worker-a", BatchSize: 3, LeaseDuration: time.Minute})

	result, err := runner.Run(context.Background())
	if err != nil || !checkpoints.completed || result.Processed != 0 {
		t.Fatalf("empty snapshot result=%+v completed=%v err=%v", result, checkpoints.completed, err)
	}
}

func TestRunnerReportsLeaseLossWithoutCheckpointAdvance(t *testing.T) {
	leaseLost := errors.New("lease lost")
	checkpoints := &checkpointStub{advanceErr: leaseLost}
	runner := mustRunner(t, &sourceStub{highWatermark: 1, messages: []SourceMessage{message(1)}}, checkpoints, &timelineStub{}, Config{JobName: "timeline-v1", OwnerID: "worker-a", BatchSize: 1, LeaseDuration: time.Minute})

	if _, err := runner.Run(context.Background()); !errors.Is(err, leaseLost) {
		t.Fatalf("run error = %v", err)
	}
}

func TestRunnerRejectsOversizedBatch(t *testing.T) {
	_, err := NewRunner(&sourceStub{}, &checkpointStub{}, &timelineStub{}, Config{
		JobName: "timeline-v1", OwnerID: "worker-a", BatchSize: MaxBatchSize + 1, LeaseDuration: time.Minute,
	})
	if err == nil {
		t.Fatal("expected oversized backfill batch to fail")
	}
}

func message(id uint64) SourceMessage {
	return SourceMessage{SourceID: id, Message: model.Message{
		UUID: fmt.Sprintf("M-%d", id), ClientMessageID: fmt.Sprintf("C-%d", id),
		ConversationKey: "direct:U-1:U-2", Seq: id, SenderUUID: "U-1",
		TargetType: model.MessageTargetDirect, TargetUUID: "U-2", Content: "payload",
		SentAt: time.Unix(int64(id), 0).UTC(),
	}}
}

func mustRunner(t *testing.T, source Source, checkpoints CheckpointStore, timeline TimelineAppender, cfg Config) *Runner {
	t.Helper()
	runner, err := NewRunner(source, checkpoints, timeline, cfg)
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}
	return runner
}
