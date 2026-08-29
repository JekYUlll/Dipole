package syncreconcile

import (
	"context"
	"testing"

	"github.com/JekYUlll/Dipole/internal/model"
	syncbackfill "github.com/JekYUlll/Dipole/internal/operations/sync/backfill"
)

type sourceStub struct{ items []syncbackfill.SourceItem }

func (s *sourceStub) ListAfter(_ context.Context, afterID, throughID uint64, limit int) ([]syncbackfill.SourceItem, error) {
	result := make([]syncbackfill.SourceItem, 0, limit)
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

type snapshotStub struct{ highWatermark uint64 }

func (s snapshotStub) CompletedHighWatermark(ctx context.Context, jobName string) (uint64, error) {
	_, _ = ctx, jobName
	return s.highWatermark, nil
}

type targetStub struct {
	rows map[string][]model.SyncInboxLocator
}

func (s targetStub) ListByMessageUUID(_ context.Context, messageUUID string) ([]model.SyncInboxLocator, error) {
	return s.rows[messageUUID], nil
}

func TestReconcilerReportsConsistentSnapshot(t *testing.T) {
	source := &sourceStub{items: []syncbackfill.SourceItem{
		reconcileItem(1, true, "U1", "U2"), reconcileItem(2, false, "U1", "U2"),
	}}
	target := targetStub{rows: map[string][]model.SyncInboxLocator{
		"M-1": {locator("U1", "M-1", 1), locator("U2", "M-1", 1)},
	}}
	reconciler := mustReconciler(t, source, snapshotStub{highWatermark: 2}, target, Config{JobName: "sync-v1", BatchSize: 10, MaxExamples: 10})
	report, err := reconciler.Run(context.Background())
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if !report.Consistent || report.Events != 2 || report.ExpectedRows != 2 || report.ActualRows != 2 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestReconcilerFindsMissingExtraAndLocatorMismatch(t *testing.T) {
	source := &sourceStub{items: []syncbackfill.SourceItem{
		reconcileItem(1, true, "U1", "U2"), reconcileItem(2, false, "U3"),
	}}
	target := targetStub{rows: map[string][]model.SyncInboxLocator{
		"M-1": {locator("U1", "M-1", 99), locator("U9", "M-1", 1)},
		"M-2": {locator("U3", "M-2", 2)},
	}}
	reconciler := mustReconciler(t, source, snapshotStub{highWatermark: 2}, target, Config{JobName: "sync-v1", BatchSize: 1, MaxExamples: 10})
	report, err := reconciler.Run(context.Background())
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if report.Consistent || report.MissingRows != 1 || report.ExtraRows != 2 || report.LocatorMismatches != 1 || len(report.Examples) != 4 {
		t.Fatalf("unexpected mismatch report: %+v", report)
	}
}

func reconcileItem(id uint64, fanout bool, recipients ...string) syncbackfill.SourceItem {
	return syncbackfill.SourceItem{SourceID: id, Fanout: fanout, Projection: &model.SyncProjection{
		EventID: "E", MessageUUID: "M-" + string(rune('0'+id)), ConversationKey: "direct:U1:U2",
		MessageSeq: id, RecipientUUIDs: recipients,
	}}
}

func locator(userUUID, messageUUID string, seq uint64) model.SyncInboxLocator {
	return model.SyncInboxLocator{UserUUID: userUUID, MessageUUID: messageUUID, ConversationKey: "direct:U1:U2", MessageSeq: seq}
}

func mustReconciler(t *testing.T, source Source, snapshot Snapshot, target Target, cfg Config) *Reconciler {
	t.Helper()
	reconciler, err := NewReconciler(source, snapshot, target, cfg)
	if err != nil {
		t.Fatalf("new reconciler: %v", err)
	}
	return reconciler
}
