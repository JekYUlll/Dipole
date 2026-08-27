package searchcutover

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSwitcherRequiresFreshVerifiedSnapshot(t *testing.T) {
	tests := []struct {
		name       string
		confirmed  bool
		watermarks []uint64
		consistent bool
		wantCalls  [][2]string
	}{
		{name: "maintenance confirmation", watermarks: []uint64{10}, consistent: true},
		{name: "stale snapshot", confirmed: true, watermarks: []uint64{11}, consistent: true},
		{name: "reconciliation mismatch", confirmed: true, watermarks: []uint64{10}, consistent: false},
		{name: "fresh snapshot", confirmed: true, watermarks: []uint64{10, 10, 10}, consistent: true, wantCalls: [][2]string{{"old", "new"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			aliases := &aliasStub{}
			switcher, err := New(&sourceStub{values: test.watermarks}, snapshotStub{watermark: 10}, verifierStub{consistent: test.consistent}, aliases, Config{
				Action: ActionSwitch, JobName: "build-1", FromIndex: "old", ToIndex: "new",
				MaintenanceConfirmed: test.confirmed, RollbackWindow: 24 * time.Hour,
			})
			if err != nil {
				if test.name == "maintenance confirmation" {
					return
				}
				t.Fatalf("create switcher: %v", err)
			}
			_, runErr := switcher.Run(context.Background())
			if len(test.wantCalls) == 0 {
				if runErr == nil || len(aliases.calls) != 0 {
					t.Fatalf("expected switch rejection: calls=%v err=%v", aliases.calls, runErr)
				}
				return
			}
			if runErr != nil || !equalCalls(aliases.calls, test.wantCalls) {
				t.Fatalf("fresh switch calls=%v err=%v", aliases.calls, runErr)
			}
		})
	}
}

func TestSwitcherCompensatesWhenSourceAdvancesDuringAliasOperation(t *testing.T) {
	aliases := &aliasStub{}
	switcher, err := New(&sourceStub{values: []uint64{10, 10, 11}}, snapshotStub{watermark: 10}, verifierStub{consistent: true}, aliases, Config{
		Action: ActionSwitch, JobName: "build-1", FromIndex: "old", ToIndex: "new",
		MaintenanceConfirmed: true, RollbackWindow: time.Hour,
	})
	if err != nil {
		t.Fatalf("create switcher: %v", err)
	}
	_, err = switcher.Run(context.Background())
	if err == nil || !equalCalls(aliases.calls, [][2]string{{"old", "new"}, {"new", "old"}}) {
		t.Fatalf("expected compensating switch: calls=%v err=%v", aliases.calls, err)
	}
}

func TestSwitcherReportsCompensationFailure(t *testing.T) {
	expected := errors.New("rollback unavailable")
	aliases := &aliasStub{failCall: 2, err: expected}
	switcher, _ := New(&sourceStub{values: []uint64{10, 10, 11}}, snapshotStub{watermark: 10}, verifierStub{consistent: true}, aliases, Config{
		Action: ActionRollback, JobName: "rollback-1", FromIndex: "new", ToIndex: "old",
		MaintenanceConfirmed: true, RollbackWindow: time.Hour,
	})
	_, err := switcher.Run(context.Background())
	if !errors.Is(err, expected) {
		t.Fatalf("expected joined compensation failure, got %v", err)
	}
}

type sourceStub struct {
	values []uint64
	call   int
}

func (s *sourceStub) HighWatermark(context.Context) (uint64, error) {
	index := s.call
	if index >= len(s.values) {
		index = len(s.values) - 1
	}
	value := s.values[index]
	s.call++
	return value, nil
}

type snapshotStub struct{ watermark uint64 }

func (s snapshotStub) CompletedHighWatermark(context.Context, string) (uint64, error) {
	return s.watermark, nil
}

type verifierStub struct{ consistent bool }

func (s verifierStub) Verify(context.Context, uint64) (Verification, error) {
	return Verification{Consistent: s.consistent, SourceCount: 3, TargetCount: 3}, nil
}

type aliasStub struct {
	calls    [][2]string
	failCall int
	err      error
}

func (s *aliasStub) Switch(_ context.Context, from, to string) error {
	s.calls = append(s.calls, [2]string{from, to})
	if s.failCall == len(s.calls) {
		return s.err
	}
	return nil
}

func equalCalls(left, right [][2]string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
