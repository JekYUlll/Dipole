package messagemysql_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	mysqlData "github.com/JekYUlll/Dipole/internal/platform/mysql"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/migration"
	messagemysql "github.com/JekYUlll/Dipole/internal/services/message/infrastructure/mysql"
)

func TestOutboxRelayRepositoryContract(t *testing.T) {
	db, _ := openContractDatabase(t)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatalf("create migration runner: %v", err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatalf("migrate contract database: %v", err)
	}
	mysqlStore, err := mysqlData.NewStore(db)
	if err != nil {
		t.Fatalf("create MySQL store: %v", err)
	}
	sqlcRepo, err := messagemysql.NewOutboxRepository(mysqlStore)
	if err != nil {
		t.Fatalf("create sqlc outbox repository: %v", err)
	}
	t.Run("sqlc", func(t *testing.T) {
		runOutboxRelayContract(t, db, sqlcRepo, "sqlc")
	})
}

func runOutboxRelayContract(t *testing.T, db *sql.DB, store application.OutboxRelayStore, prefix string) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Millisecond)
	lease := 5 * time.Second

	empty, err := store.ClaimPendingBatch(0, now, lease)
	if err != nil || len(empty) != 0 {
		t.Fatalf("zero-limit claim: events=%+v err=%v", empty, err)
	}

	dueID := seedOutboxEvent(t, db, prefix+"-due", model.OutboxStatusPending, nil, nil, nil)
	futureRetry := now.Add(time.Hour)
	seedOutboxEvent(t, db, prefix+"-future", model.OutboxStatusPending, &futureRetry, nil, nil)
	staleLock := now.Add(-10 * time.Minute)
	staleID := seedOutboxEvent(t, db, prefix+"-stale", model.OutboxStatusProcessing, nil, &staleLock, nil)
	activeLock := now.Add(time.Hour)
	seedOutboxEvent(t, db, prefix+"-active", model.OutboxStatusProcessing, nil, &activeLock, nil)
	publishedAt := now.Add(-time.Minute)
	seedOutboxEvent(t, db, prefix+"-published", model.OutboxStatusPublished, nil, nil, &publishedAt)

	claimed, err := store.ClaimPendingBatch(2, now, lease)
	if err != nil {
		t.Fatalf("claim due and stale events: %v", err)
	}
	if got := outboxEventIDs(claimed); len(got) != 2 || got[0] != dueID || got[1] != staleID {
		t.Fatalf("unexpected claimed order: got=%v want=[%d %d]", got, dueID, staleID)
	}
	for _, event := range claimed {
		if event.Status != model.OutboxStatusProcessing || event.LockedAt == nil || !event.LockedAt.Equal(now) {
			t.Fatalf("claim did not hydrate lease state: %+v", event)
		}
	}
	if err := store.MarkPublished(staleID, now); err != nil {
		t.Fatalf("publish reclaimed stale event: %v", err)
	}

	second, err := store.ClaimPendingBatch(10, now, lease)
	if err != nil || len(second) != 0 {
		t.Fatalf("active leases or future retries were reclaimed: events=%+v err=%v", second, err)
	}

	nextRetryAt := now.Add(30 * time.Second)
	longError := strings.Repeat("x", 600)
	if err := store.MarkRetry(dueID, 3, nextRetryAt, errors.New(longError)); err != nil {
		t.Fatalf("mark retry: %v", err)
	}
	state := readOutboxState(t, db, dueID)
	if state.status != model.OutboxStatusPending || state.retryCount != 3 || state.lockedAt.Valid || !state.nextRetryAt.Valid || !state.nextRetryAt.Time.Equal(nextRetryAt) || len(state.lastError.String) != 512 {
		t.Fatalf("unexpected retry state: %+v", state)
	}

	tooEarly, err := store.ClaimPendingBatch(10, now, lease)
	if err != nil || len(tooEarly) != 0 {
		t.Fatalf("retry claimed before backoff elapsed: events=%+v err=%v", tooEarly, err)
	}
	retried, err := store.ClaimPendingBatch(10, nextRetryAt, lease)
	if err != nil || len(retried) != 1 || retried[0].ID != dueID {
		t.Fatalf("retry not reclaimed when due: events=%+v err=%v", retried, err)
	}
	finalPublishedAt := nextRetryAt.Add(time.Second)
	if err := store.MarkPublished(dueID, finalPublishedAt); err != nil {
		t.Fatalf("mark published: %v", err)
	}
	state = readOutboxState(t, db, dueID)
	if state.status != model.OutboxStatusPublished || state.lockedAt.Valid || state.lastError.String != "" || !state.publishedAt.Valid || !state.publishedAt.Time.Equal(finalPublishedAt) {
		t.Fatalf("unexpected published state: %+v", state)
	}

	headers, err := store.DecodeHeaders(&model.OutboxEvent{HeadersJSON: []byte(`{"trace_id":"trace-1"}`)})
	if err != nil || headers["trace_id"] != "trace-1" {
		t.Fatalf("decode valid headers: headers=%v err=%v", headers, err)
	}
	headers, err = store.DecodeHeaders(nil)
	if err != nil || headers != nil {
		t.Fatalf("decode nil headers: headers=%v err=%v", headers, err)
	}
	if _, err := store.DecodeHeaders(&model.OutboxEvent{HeadersJSON: []byte("{")}); err == nil {
		t.Fatal("expected invalid headers to fail")
	}
}

func seedOutboxEvent(t *testing.T, db *sql.DB, aggregateID, status string, nextRetryAt, lockedAt, publishedAt *time.Time) uint {
	t.Helper()
	result, err := db.Exec(
		"INSERT INTO outbox_events (aggregate_type, aggregate_id, event_type, topic, message_key, value, headers_json, status, retry_count, next_retry_at, locked_at, published_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?, ?, ?, ?)",
		"message", aggregateID, "message.created", "message.created", aggregateID, []byte("payload"), []byte(`{"source":"contract"}`), status, nextRetryAt, lockedAt, publishedAt, time.Now().UTC(), time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("seed outbox event %s: %v", aggregateID, err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("read outbox id %s: %v", aggregateID, err)
	}
	return uint(id)
}

type outboxState struct {
	status      string
	retryCount  int
	lastError   sql.NullString
	nextRetryAt sql.NullTime
	lockedAt    sql.NullTime
	publishedAt sql.NullTime
}

func readOutboxState(t *testing.T, db *sql.DB, id uint) outboxState {
	t.Helper()
	var state outboxState
	if err := db.QueryRow(
		"SELECT status, retry_count, last_error, next_retry_at, locked_at, published_at FROM outbox_events WHERE id = ?",
		id,
	).Scan(&state.status, &state.retryCount, &state.lastError, &state.nextRetryAt, &state.lockedAt, &state.publishedAt); err != nil {
		t.Fatalf("read outbox state %d: %v", id, err)
	}
	return state
}

func outboxEventIDs(events []*model.OutboxEvent) []uint {
	ids := make([]uint, 0, len(events))
	for _, event := range events {
		ids = append(ids, event.ID)
	}
	return ids
}
