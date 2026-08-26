package mysql_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/JekYUlll/Dipole/db/migrations"
	syncbaseline "github.com/JekYUlll/Dipole/internal/baseline/sync"
	"github.com/JekYUlll/Dipole/internal/data/migration"
	mysqlStore "github.com/JekYUlll/Dipole/internal/data/mysql"
)

func TestSyncBaselineCaptureReconcileAndRestore(t *testing.T) {
	db := openTemporaryDatabase(t)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatalf("create migration runner: %v", err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatalf("migrate temporary database: %v", err)
	}
	seedSyncBaseline(t, db)

	store, err := mysqlStore.NewStore(db)
	if err != nil {
		t.Fatalf("create sqlc store: %v", err)
	}
	baseline, err := mysqlStore.NewSyncBaselineStore(store)
	if err != nil {
		t.Fatalf("create Sync baseline store: %v", err)
	}
	ctx := context.Background()
	manifest, err := baseline.Capture(ctx, "legacy-v1")
	if err != nil {
		t.Fatalf("capture Sync baseline: %v", err)
	}
	if manifest.HighWatermarkSyncSeq != 12 || manifest.EntryCount != 2 || len(manifest.EntriesSHA256) != 64 || manifest.FirstCreatedOutboxID == 0 {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	repeated, err := baseline.Capture(ctx, "legacy-v1")
	if err != nil || repeated != manifest {
		t.Fatalf("repeat capture changed immutable manifest: repeated=%+v err=%v", repeated, err)
	}
	results := make(chan struct {
		manifest syncbaseline.Manifest
		err      error
	}, 2)
	for range 2 {
		go func() {
			captured, captureErr := baseline.Capture(ctx, "legacy-concurrent")
			results <- struct {
				manifest syncbaseline.Manifest
				err      error
			}{captured, captureErr}
		}()
	}
	var concurrentManifest syncbaseline.Manifest
	for range 2 {
		result := <-results
		if result.err != nil {
			t.Fatalf("concurrent capture: %v", result.err)
		}
		if concurrentManifest.JobName == "" {
			concurrentManifest = result.manifest
		} else if result.manifest != concurrentManifest {
			t.Fatalf("concurrent captures diverged: first=%+v second=%+v", concurrentManifest, result.manifest)
		}
	}

	report, err := baseline.Reconcile(ctx, "legacy-v1", 10)
	if err != nil || !report.Consistent || report.ExpectedRows != 2 || report.ActualRows != 2 {
		t.Fatalf("unexpected initial reconciliation: report=%+v err=%v", report, err)
	}
	if _, err := db.Exec("DELETE FROM user_sync_inbox WHERE sync_seq = 11"); err != nil {
		t.Fatalf("delete archived Inbox row: %v", err)
	}
	report, err = baseline.Reconcile(ctx, "legacy-v1", 10)
	if err != nil || report.Missing != 1 || report.Consistent {
		t.Fatalf("expected one missing row: report=%+v err=%v", report, err)
	}
	report, err = baseline.Restore(ctx, "legacy-v1", 10)
	if err != nil || !report.Consistent {
		t.Fatalf("restore missing baseline row: report=%+v err=%v", report, err)
	}
	var restoredSeq uint64
	if err := db.QueryRow("SELECT sync_seq FROM user_sync_inbox WHERE user_uuid = 'U2' AND message_uuid = 'M-LEGACY'").Scan(&restoredSeq); err != nil {
		t.Fatalf("read restored sequence: %v", err)
	}
	if restoredSeq != 11 {
		t.Fatalf("expected original sequence 11, got %d", restoredSeq)
	}

	if _, err := db.Exec("DELETE FROM user_sync_inbox WHERE sync_seq = 11"); err != nil {
		t.Fatalf("delete row before conflict fixture: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO user_sync_inbox
		(sync_seq, user_uuid, message_uuid, conversation_key, message_seq, created_at)
		VALUES (13, 'U2', 'M-LEGACY', 'group:G1', 4, NOW(3))`); err != nil {
		t.Fatalf("insert moved baseline row: %v", err)
	}
	report, err = baseline.Restore(ctx, "legacy-v1", 10)
	if !errors.Is(err, mysqlStore.ErrUnsafeSyncBaselineRestore) || report.Conflicting != 1 {
		t.Fatalf("expected unsafe conflict refusal: report=%+v err=%v", report, err)
	}
	if _, err := db.Exec(`UPDATE sync_inbox_baseline_entries
		SET message_seq = message_seq + 1 WHERE job_name = 'legacy-v1' AND sync_seq = 10`); err != nil {
		t.Fatalf("corrupt baseline fixture: %v", err)
	}
	if _, err := baseline.Reconcile(ctx, "legacy-v1", 10); err == nil {
		t.Fatal("expected modified baseline archive to fail integrity validation")
	}
}

func seedSyncBaseline(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO user_sync_states (user_uuid, created_at, updated_at) VALUES
		('U1', NOW(3), NOW(3)), ('U2', NOW(3), NOW(3)), ('U3', NOW(3), NOW(3))`); err != nil {
		t.Fatalf("seed Sync states: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO user_sync_inbox
		(sync_seq, user_uuid, message_uuid, conversation_key, message_seq, created_at) VALUES
		(10, 'U1', 'M-LEGACY', 'group:G1', 4, NOW(3)),
		(11, 'U2', 'M-LEGACY', 'group:G1', 4, NOW(3)),
		(12, 'U3', 'M-EVENT', 'direct:U1:U3', 9, NOW(3))`); err != nil {
		t.Fatalf("seed Sync Inbox: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO outbox_events (
		aggregate_type, aggregate_id, event_type, topic, message_key, value,
		status, retry_count, created_at, updated_at
	) VALUES ('message', 'M-EVENT', 'message.direct.created', 'message.direct.created',
		'M-EVENT', '{}', 'published', 0, NOW(3), NOW(3))`); err != nil {
		t.Fatalf("seed created Outbox event: %v", err)
	}
}
