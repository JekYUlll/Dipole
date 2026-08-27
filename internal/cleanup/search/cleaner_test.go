package searchcleanup

import (
	"context"
	"errors"
	"testing"
	"time"

	searchbackfill "github.com/JekYUlll/Dipole/internal/backfill/search"
	searchreconcile "github.com/JekYUlll/Dipole/internal/reconcile/search"
)

func TestCleanerDefaultsToDryRunAndRequiresConsistentEvidence(t *testing.T) {
	store := &cleanupStoreStub{published: 3}
	cleaner, err := New(store, cleanupReceipt(), cleanupReport(), Config{BatchSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	result, err := cleaner.Run(context.Background())
	if err != nil || !result.DryRun || result.EligibleCount != 3 || result.DeletedCount != 0 || store.deleteCalls != 0 {
		t.Fatalf("unexpected dry-run: %+v calls=%d err=%v", result, store.deleteCalls, err)
	}
	report := cleanupReport()
	report.HashMismatchCount = 1
	if _, err := New(store, cleanupReceipt(), report, Config{BatchSize: 2}); err == nil {
		t.Fatal("expected inconsistent evidence to fail")
	}
}

func TestCleanerExecutesPublishedBatchesAndBlocksUnsafeRange(t *testing.T) {
	store := &cleanupStoreStub{published: 3}
	cleaner, err := New(store, cleanupReceipt(), cleanupReport(), Config{BatchSize: 2, Execute: true, MaintenanceConfirmed: true, Operator: "operator-1"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := cleaner.Run(context.Background())
	if err != nil || result.DeletedCount != 3 || store.deleteCalls != 2 {
		t.Fatalf("unexpected cleanup: %+v calls=%d err=%v", result, store.deleteCalls, err)
	}
	store = &cleanupStoreStub{published: 2, nonPublished: 1}
	cleaner, _ = New(store, cleanupReceipt(), cleanupReport(), Config{BatchSize: 2, Execute: true, MaintenanceConfirmed: true, Operator: "operator-1"})
	if _, err := cleaner.Run(context.Background()); err == nil || store.deleteCalls != 0 {
		t.Fatalf("expected non-published range to block cleanup: calls=%d err=%v", store.deleteCalls, err)
	}
	if _, err := New(store, cleanupReceipt(), cleanupReport(), Config{BatchSize: 2, Execute: true}); err == nil {
		t.Fatal("expected missing maintenance confirmation to fail")
	}
	if _, err := New(store, cleanupReceipt(), cleanupReport(), Config{BatchSize: 2, Execute: true, MaintenanceConfirmed: true}); err == nil {
		t.Fatal("expected missing operator to fail")
	}
}

func TestCleanerResumesAfterPartialBatchFailure(t *testing.T) {
	store := &cleanupStoreStub{published: 3, failDeleteCall: 2}
	config := Config{BatchSize: 2, Execute: true, MaintenanceConfirmed: true, Operator: "operator-1"}
	cleaner, err := New(store, cleanupReceipt(), cleanupReport(), config)
	if err != nil {
		t.Fatal(err)
	}
	result, err := cleaner.Run(context.Background())
	if err == nil || result.DeletedCount != 2 || store.published != 1 {
		t.Fatalf("expected partial cleanup: result=%+v remaining=%d err=%v", result, store.published, err)
	}
	cleaner, err = New(store, cleanupReceipt(), cleanupReport(), config)
	if err != nil {
		t.Fatal(err)
	}
	result, err = cleaner.Run(context.Background())
	if err != nil || result.EligibleCount != 1 || result.DeletedCount != 1 || store.published != 0 {
		t.Fatalf("expected resumable cleanup: result=%+v remaining=%d err=%v", result, store.published, err)
	}
}

type cleanupStoreStub struct {
	published      uint64
	nonPublished   uint64
	deleteCalls    int
	failDeleteCall int
	failed         bool
}

func (s *cleanupStoreStub) Inspect(context.Context, uint64) (uint64, uint64, error) {
	return s.published, s.nonPublished, nil
}
func (s *cleanupStoreStub) DeletePublishedBatch(_ context.Context, _ uint64, limit int) (uint64, error) {
	s.deleteCalls++
	if s.deleteCalls == s.failDeleteCall && !s.failed {
		s.failed = true
		return 0, errors.New("injected batch failure")
	}
	if s.published == 0 {
		return 0, errors.New("unexpected delete")
	}
	deleted := uint64(limit)
	if deleted > s.published {
		deleted = s.published
	}
	s.published -= deleted
	return deleted, nil
}

func cleanupReceipt() searchbackfill.ArchiveReceipt {
	return searchbackfill.ArchiveReceipt{SchemaVersion: searchbackfill.ArchiveReceiptSchemaV1, SnapshotID: "snapshot-5", HighWatermarkID: 5,
		Manifest: searchbackfill.ArchiveObjectVersion{VersionID: "manifest-v1"}, Data: searchbackfill.ArchiveObjectVersion{VersionID: "data-v1"}}
}

func cleanupReport() searchreconcile.Report {
	return searchreconcile.Report{JobName: "search-v1", SourceHighWatermark: 5, CompletedAt: time.Now().UTC(), SourceCount: 3, TargetCount: 3, TargetFoundCount: 3, HashMatchedCount: 3, Consistent: true}
}
