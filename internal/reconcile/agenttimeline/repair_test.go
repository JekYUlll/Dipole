package agenttimeline

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

type repairStoreStub struct {
	items     []application.AgentTaskTimelineRepairV1
	completed []string
	retries   []uint32
}

func (s *repairStoreStub) EnqueueAgentTaskTimelineRepair(context.Context, application.AgentTaskTimelineEventV1, error) error {
	return nil
}
func (s *repairStoreStub) ClaimAgentTaskTimelineRepairs(int, time.Time, time.Duration) ([]application.AgentTaskTimelineRepairV1, error) {
	return s.items, nil
}
func (s *repairStoreStub) MarkAgentTaskTimelineRepairCompleted(id string) error {
	s.completed = append(s.completed, id)
	return nil
}
func (s *repairStoreStub) MarkAgentTaskTimelineRepairRetry(_ string, count uint32, _ time.Time, _ error) error {
	s.retries = append(s.retries, count)
	return nil
}

type timelineStub struct {
	appendErr error
	events    []application.AgentTaskTimelineEventV1
}

func (s *timelineStub) AppendAgentTaskTimelineEvent(_ context.Context, event application.AgentTaskTimelineEventV1) (uint64, error) {
	if s.appendErr != nil {
		return 0, s.appendErr
	}
	s.events = append(s.events, event)
	return uint64(len(s.events)), nil
}
func (s *timelineStub) ListAgentTaskTimelineEvents(context.Context, string, uint64, int) ([]application.AgentTaskTimelineEventV1, error) {
	return nil, nil
}

func TestRepairerReplaysAndCompletesClaimedIntent(t *testing.T) {
	repairs := &repairStoreStub{items: []application.AgentTaskTimelineRepairV1{{EventUUID: "EV-1", TaskUUID: "TASK-1", RunUUID: "RUN-1", Kind: application.AgentTaskTimelineEventModelCall, Status: "completed", OccurredAt: time.Now()}}}
	repairer, err := NewRepairer(repairs, &timelineStub{}, 10, time.Minute, time.Second, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	report, err := repairer.RunOnce(context.Background(), time.Now())
	if err != nil || report.Claimed != 1 || report.Repaired != 1 || len(repairs.completed) != 1 {
		t.Fatalf("report=%+v completed=%v err=%v", report, repairs.completed, err)
	}
}

func TestRepairerRetriesFailedProjection(t *testing.T) {
	repairs := &repairStoreStub{items: []application.AgentTaskTimelineRepairV1{{EventUUID: "EV-2", TaskUUID: "TASK-2", Kind: application.AgentTaskTimelineEventModelCall, Status: "running", OccurredAt: time.Now(), RetryCount: 2}}}
	repairer, err := NewRepairer(repairs, &timelineStub{appendErr: errors.New("projection unavailable")}, 10, time.Minute, time.Second, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	report, err := repairer.RunOnce(context.Background(), time.Now())
	if err != nil || report.Retried != 1 || len(repairs.retries) != 1 || repairs.retries[0] != 3 {
		t.Fatalf("report=%+v retries=%v err=%v", report, repairs.retries, err)
	}
}

func TestRepairerRejectsInvalidConfiguration(t *testing.T) {
	if _, err := NewRepairer(nil, &timelineStub{}, 10, time.Minute, time.Second, time.Second); err == nil {
		t.Fatal("expected missing repair store error")
	}
}
