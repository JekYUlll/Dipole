package mysql_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/compat/service"
	"github.com/JekYUlll/Dipole/internal/data/migration"
	mysqlStore "github.com/JekYUlll/Dipole/internal/data/mysql"
	"github.com/JekYUlll/Dipole/internal/model"
	searchbackfill "github.com/JekYUlll/Dipole/internal/operations/search/backfill"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
)

func TestSearchBackfillSourceAndCheckpointContract(t *testing.T) {
	db := openTemporaryDatabase(t)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatalf("create migration runner: %v", err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatalf("migrate temporary database: %v", err)
	}
	insertSearchOutboxEvent(t, db, "M1", "message.direct.created", searchPayload("M1", service.MessageMutationCreated, 1, "first"))
	insertSearchOutboxEvent(t, db, "M2", "message.direct.created", searchPayload("M2", service.MessageMutationCreated, 1, "second"))
	insertSearchOutboxEvent(t, db, "M1@r2", "message.direct.edited", searchPayload("M1", service.MessageMutationEdited, 2, "edited"))
	if _, err := db.Exec(`INSERT INTO outbox_events (
		aggregate_type, aggregate_id, event_type, topic, message_key, value, status, retry_count, created_at, updated_at
	) VALUES ('user', 'U1', 'user.updated', 'user.updated', 'U1', '{}', 'published', 0, NOW(3), NOW(3))`); err != nil {
		t.Fatalf("insert unrelated outbox event: %v", err)
	}

	store, err := mysqlStore.NewStore(db)
	if err != nil {
		t.Fatalf("create MySQL store: %v", err)
	}
	source, err := mysqlStore.NewSearchBackfillSource(store)
	if err != nil {
		t.Fatalf("create Search source: %v", err)
	}
	highWatermark, err := source.HighWatermark(context.Background())
	if err != nil || highWatermark != 3 {
		t.Fatalf("high watermark=%d err=%v", highWatermark, err)
	}
	items, err := source.ListAfter(context.Background(), 0, highWatermark, 10)
	if err != nil {
		t.Fatalf("list Search mutations: %v", err)
	}
	if len(items) != 2 || items[0].SourceID != 2 || items[0].Mutation.MessageUUID != "M2" ||
		items[1].SourceID != 3 || items[1].Mutation.MessageUUID != "M1" || items[1].Mutation.Revision != 2 {
		t.Fatalf("unexpected final-state source page: %+v", items)
	}

	checkpoints, err := mysqlStore.NewSearchBackfillCheckpointStore(store, "dipole-messages-v1-build-a")
	if err != nil {
		t.Fatalf("create Search checkpoints: %v", err)
	}
	descriptor, err := source.Descriptor(context.Background(), highWatermark)
	if err != nil {
		t.Fatalf("describe Search source: %v", err)
	}
	checkpoint, err := checkpoints.Acquire(context.Background(), "search-v1-build-a", "owner-a", descriptor, highWatermark, time.Minute)
	if err != nil || checkpoint.HighWatermarkID != 3 || checkpoint.Status != searchbackfill.StatusRunning {
		t.Fatalf("initial checkpoint=%+v err=%v", checkpoint, err)
	}
	if _, err := checkpoints.Acquire(context.Background(), "search-v1-build-a", "owner-b", descriptor, 99, time.Minute); !errors.Is(err, mysqlStore.ErrSearchBackfillLeaseHeld) {
		t.Fatalf("second owner error=%v", err)
	}
	if err := checkpoints.Advance(context.Background(), "search-v1-build-a", "owner-a", 2, time.Minute); err != nil {
		t.Fatalf("advance checkpoint: %v", err)
	}
	if err := checkpoints.Fail(context.Background(), "search-v1-build-a", "owner-a", errors.New("planned retry")); err != nil {
		t.Fatalf("fail checkpoint: %v", err)
	}
	checkpoint, err = checkpoints.Acquire(context.Background(), "search-v1-build-a", "owner-b", descriptor, 99, time.Minute)
	if err != nil || checkpoint.HighWatermarkID != 3 || checkpoint.LastProcessedID != 2 {
		t.Fatalf("resumed checkpoint=%+v err=%v", checkpoint, err)
	}
	wrongSource := descriptor
	wrongSource.SnapshotID = "archive:other"
	if _, err := checkpoints.Acquire(context.Background(), "search-v1-build-a", "owner-b", wrongSource, highWatermark, time.Minute); !errors.Is(err, mysqlStore.ErrSearchBackfillSourceMismatch) {
		t.Fatalf("source mismatch error=%v", err)
	}
	if err := checkpoints.Advance(context.Background(), "search-v1-build-a", "owner-b", 3, time.Minute); err != nil {
		t.Fatalf("finish checkpoint: %v", err)
	}
	if err := checkpoints.Complete(context.Background(), "search-v1-build-a", "owner-b"); err != nil {
		t.Fatalf("complete checkpoint: %v", err)
	}
	completed, err := checkpoints.CompletedHighWatermark(context.Background(), "search-v1-build-a")
	if err != nil || completed != 3 {
		t.Fatalf("completed watermark=%d err=%v", completed, err)
	}
	if _, err := checkpoints.CompletedHighWatermarkForSource(context.Background(), "search-v1-build-a", wrongSource); !errors.Is(err, mysqlStore.ErrSearchBackfillSourceMismatch) {
		t.Fatalf("completed source mismatch error=%v", err)
	}
	wrongTarget, _ := mysqlStore.NewSearchBackfillCheckpointStore(store, "dipole-messages-v1-build-b")
	if _, err := wrongTarget.CompletedHighWatermark(context.Background(), "search-v1-build-a"); !errors.Is(err, mysqlStore.ErrSearchBackfillTargetMismatch) {
		t.Fatalf("target mismatch error=%v", err)
	}
}

func searchPayload(messageID string, mutation service.MessageMutationType, revision uint64, content string) service.MessageEventPayload {
	return service.MessageEventPayload{
		MessageID: messageID, MutationType: mutation, Revision: revision, ActorUUID: "U1",
		ConversationKey: "direct:U1:U2", MessageSeq: 1, SenderUUID: "U1", TargetUUID: "U2",
		TargetType: model.MessageTargetDirect, Content: content, SentAt: time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC),
	}
}

func insertSearchOutboxEvent(t *testing.T, db *sql.DB, aggregateID, eventType string, payload service.MessageEventPayload) {
	t.Helper()
	envelope, err := platformKafka.NewEnvelope(eventType, payload)
	if err != nil {
		t.Fatalf("create envelope: %v", err)
	}
	value, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO outbox_events (
		aggregate_type, aggregate_id, event_type, topic, message_key, value, status, retry_count, created_at, updated_at
	) VALUES ('message', ?, ?, ?, ?, ?, 'published', 0, NOW(3), NOW(3))`,
		aggregateID, eventType, eventType, payload.MessageID, value); err != nil {
		t.Fatalf("insert Search outbox event: %v", err)
	}
}
