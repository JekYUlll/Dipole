package searchreconcile

import (
	"context"
	"testing"
	"time"

	searchbackfill "github.com/JekYUlll/Dipole/internal/backfill/search"
	"github.com/JekYUlll/Dipole/internal/model"
)

func TestReconcilerReportsExactFixedSnapshot(t *testing.T) {
	first := state(t, upsert("M1", "one", 1))
	second := state(t, tombstone("M2", 3))
	source := reconcileSource{items: []searchbackfill.SourceMutation{
		{SourceID: 4, Mutation: upsert("M1", "one", 1)},
		{SourceID: 9, Mutation: tombstone("M2", 3)},
	}}
	target := reconcileTarget{states: map[string]model.MessageSearchState{"M1": first, "M2": second}}
	reconciler, err := New(source, target, Config{JobName: "search-v1", HighWatermarkID: 9, BatchSize: 1, MaxExamples: 10})
	if err != nil {
		t.Fatalf("create reconciler: %v", err)
	}
	report, err := reconciler.Run(context.Background())
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if !report.Consistent || report.SourceCount != 2 || report.TargetCount != 2 || report.HashMatchedCount != 2 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestReconcilerReportsMissingMismatchAndExtraDocuments(t *testing.T) {
	wrong := state(t, upsert("M2", "wrong", 3))
	source := reconcileSource{items: []searchbackfill.SourceMutation{
		{SourceID: 4, Mutation: upsert("M1", "one", 1)},
		{SourceID: 9, Mutation: upsert("M2", "two", 3)},
	}}
	target := reconcileTarget{states: map[string]model.MessageSearchState{
		"M2": wrong,
		"M3": state(t, tombstone("M3", 1)),
	}}
	reconciler, _ := New(source, target, Config{JobName: "search-v1", HighWatermarkID: 9, BatchSize: 10, MaxExamples: 10})
	report, err := reconciler.Run(context.Background())
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if report.Consistent || report.MissingCount != 1 || report.HashMismatchCount != 1 || report.ExtraCount != 1 {
		t.Fatalf("expected missing, mismatch, and extra document: %+v", report)
	}
}

func upsert(id, content string, revision uint64) *model.MessageSearchMutation {
	sentAt := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	return &model.MessageSearchMutation{Type: model.MessageSearchMutationUpsert, MessageUUID: id, Revision: revision, Document: &model.MessageSearchDocument{
		MessageUUID: id, ConversationKey: "direct:U1:U2", MessageSeq: revision, Revision: revision,
		SenderUUID: "U1", Content: content, SentAt: sentAt,
	}}
}

func tombstone(id string, revision uint64) *model.MessageSearchMutation {
	return &model.MessageSearchMutation{Type: model.MessageSearchMutationTombstone, MessageUUID: id, Revision: revision}
}

func state(t *testing.T, mutation *model.MessageSearchMutation) model.MessageSearchState {
	t.Helper()
	result, err := mutation.State()
	if err != nil {
		t.Fatalf("create state: %v", err)
	}
	return result
}

type reconcileSource struct {
	items []searchbackfill.SourceMutation
}

func (s reconcileSource) ListAfter(_ context.Context, after, through uint64, limit int) ([]searchbackfill.SourceMutation, error) {
	result := make([]searchbackfill.SourceMutation, 0, limit)
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

type reconcileTarget struct {
	states map[string]model.MessageSearchState
}

func (t reconcileTarget) Lookup(_ context.Context, messageUUID string) (model.MessageSearchState, bool, error) {
	value, ok := t.states[messageUUID]
	return value, ok, nil
}
func (t reconcileTarget) Count(context.Context) (uint64, error) { return uint64(len(t.states)), nil }
