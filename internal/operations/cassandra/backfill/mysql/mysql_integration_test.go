package mysql_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	cassandrabackfill "github.com/JekYUlll/Dipole/internal/operations/cassandra/backfill"
	cassandramysql "github.com/JekYUlll/Dipole/internal/operations/cassandra/backfill/mysql"
	platformmysql "github.com/JekYUlll/Dipole/internal/platform/mysql"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/migration"
	mysqltestutil "github.com/JekYUlll/Dipole/internal/platform/mysql/testutil"
)

func TestCassandraBackfillSourceAndCheckpointContract(t *testing.T) {
	db := mysqltestutil.OpenTemporaryDatabase(t)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatalf("create migration runner: %v", err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatalf("migrate temporary database: %v", err)
	}
	for _, values := range []struct {
		uuid, clientID, conversation string
		seq                          uint64
	}{
		{"M-backfill-1", "C-backfill-1", "direct:U1:U2", 1},
		{"M-backfill-2", "C-backfill-2", "group:G1", 1},
		{"M-backfill-3", "C-backfill-3", "direct:U1:U2", 2},
	} {
		if _, err := db.Exec(`INSERT INTO messages (
            uuid, client_message_id, conversation_key, seq, sender_uuid,
            target_type, target_uuid, message_type, content, sent_at
        ) VALUES (?, ?, ?, ?, 'U1', 0, 'U2', 0, 'payload', NOW(3))`,
			values.uuid, values.clientID, values.conversation, values.seq); err != nil {
			t.Fatalf("insert source message: %v", err)
		}
	}

	store, err := platformmysql.NewStore(db)
	if err != nil {
		t.Fatalf("create MySQL store: %v", err)
	}
	source, err := cassandramysql.NewCassandraBackfillSource(store)
	if err != nil {
		t.Fatalf("create backfill source: %v", err)
	}
	highWatermark, err := source.HighWatermark(context.Background())
	if err != nil || highWatermark != 3 {
		t.Fatalf("high watermark=%d err=%v", highWatermark, err)
	}
	descriptor, err := source.Descriptor(context.Background(), highWatermark)
	if err != nil {
		t.Fatalf("describe source: %v", err)
	}
	messages, err := source.ListAfter(context.Background(), 1, highWatermark, 10)
	if err != nil || len(messages) != 2 || messages[0].SourceID != 2 || messages[1].SourceID != 3 {
		t.Fatalf("source page=%+v err=%v", messages, err)
	}

	checkpoints, err := cassandramysql.NewCassandraBackfillCheckpointStore(store)
	if err != nil {
		t.Fatalf("create checkpoint store: %v", err)
	}
	checkpoint, err := checkpoints.Acquire(context.Background(), "timeline-v1", "owner-a", descriptor, highWatermark, time.Minute)
	if err != nil || checkpoint.HighWatermarkID != 3 || checkpoint.LastProcessedID != 0 || checkpoint.Status != cassandrabackfill.StatusRunning {
		t.Fatalf("initial checkpoint=%+v err=%v", checkpoint, err)
	}
	if _, err := checkpoints.CompletedHighWatermark(context.Background(), "timeline-v1"); !errors.Is(err, cassandramysql.ErrCassandraBackfillIncomplete) {
		t.Fatalf("running job reconciliation watermark error=%v", err)
	}
	if _, err := checkpoints.Acquire(context.Background(), "timeline-v1", "owner-b", descriptor, 99, time.Minute); !errors.Is(err, cassandramysql.ErrCassandraBackfillLeaseHeld) {
		t.Fatalf("second owner error=%v", err)
	}
	otherSource := cassandrabackfill.SourceDescriptor{Kind: cassandrabackfill.SourceKindMessageArchive, SnapshotID: "other", SHA256: "abc"}
	if _, err := checkpoints.Acquire(context.Background(), "timeline-v1", "owner-a", otherSource, highWatermark, time.Minute); !errors.Is(err, cassandramysql.ErrCassandraBackfillSourceMismatch) {
		t.Fatalf("changed source error=%v", err)
	}
	if err := checkpoints.Advance(context.Background(), "timeline-v1", "owner-a", 2, time.Minute); err != nil {
		t.Fatalf("advance checkpoint: %v", err)
	}
	if err := checkpoints.Fail(context.Background(), "timeline-v1", "owner-a", errors.New("planned retry")); err != nil {
		t.Fatalf("fail checkpoint: %v", err)
	}

	checkpoint, err = checkpoints.Acquire(context.Background(), "timeline-v1", "owner-b", descriptor, 99, time.Minute)
	if err != nil || checkpoint.HighWatermarkID != 3 || checkpoint.LastProcessedID != 2 {
		t.Fatalf("resumed checkpoint=%+v err=%v", checkpoint, err)
	}
	if err := checkpoints.Advance(context.Background(), "timeline-v1", "owner-b", 3, time.Minute); err != nil {
		t.Fatalf("finish checkpoint: %v", err)
	}
	if err := checkpoints.Complete(context.Background(), "timeline-v1", "owner-b"); err != nil {
		t.Fatalf("complete checkpoint: %v", err)
	}
	completedHighWatermark, err := checkpoints.CompletedHighWatermark(context.Background(), "timeline-v1")
	if err != nil || completedHighWatermark != 3 {
		t.Fatalf("completed reconciliation watermark=%d err=%v", completedHighWatermark, err)
	}
	if _, err := checkpoints.CompletedHighWatermarkForSource(context.Background(), "timeline-v1", otherSource); !errors.Is(err, cassandramysql.ErrCassandraBackfillSourceMismatch) {
		t.Fatalf("reconciliation changed source error=%v", err)
	}
	checkpoint, err = checkpoints.Acquire(context.Background(), "timeline-v1", "owner-c", descriptor, 99, time.Minute)
	if err != nil || checkpoint.Status != cassandrabackfill.StatusCompleted || checkpoint.HighWatermarkID != 3 || checkpoint.LastProcessedID != 3 {
		t.Fatalf("completed checkpoint=%+v err=%v", checkpoint, err)
	}
	if err := checkpoints.Advance(context.Background(), "timeline-v1", "owner-c", 3, time.Minute); !errors.Is(err, cassandramysql.ErrCassandraBackfillLeaseLost) {
		t.Fatalf("completed job advance error=%v", err)
	}
}
