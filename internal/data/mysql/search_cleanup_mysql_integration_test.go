package mysql_test

import (
	"context"
	"testing"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/data/migration"
	mysqldata "github.com/JekYUlll/Dipole/internal/data/mysql"
)

func TestSearchOutboxCleanupStoreContract(t *testing.T) {
	db := openTemporaryDatabase(t)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`INSERT INTO outbox_events (aggregate_type, aggregate_id, event_type, topic, message_key, value, status, retry_count, created_at, updated_at) VALUES ('message','M1','message.direct.created','message.direct.created','M1','{}','published',0,NOW(3),NOW(3))`,
		`INSERT INTO outbox_events (aggregate_type, aggregate_id, event_type, topic, message_key, value, status, retry_count, created_at, updated_at) VALUES ('user','U1','user.updated','user.updated','U1','{}','published',0,NOW(3),NOW(3))`,
		`INSERT INTO outbox_events (aggregate_type, aggregate_id, event_type, topic, message_key, value, status, retry_count, created_at, updated_at) VALUES ('message','M2','message.direct.edited','message.direct.edited','M2','{}','pending',0,NOW(3),NOW(3))`,
		`INSERT INTO outbox_events (aggregate_type, aggregate_id, event_type, topic, message_key, value, status, retry_count, created_at, updated_at) VALUES ('message','M3','message.group.deleted','message.group.deleted','M3','{}','published',0,NOW(3),NOW(3))`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	store, err := mysqldata.NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	cleanup, err := mysqldata.NewSearchOutboxCleanupStore(store)
	if err != nil {
		t.Fatal(err)
	}
	published, nonPublished, err := cleanup.Inspect(context.Background(), 3)
	if err != nil || published != 1 || nonPublished != 1 {
		t.Fatalf("unexpected cleanup inspection: published=%d non_published=%d err=%v", published, nonPublished, err)
	}
	deleted, err := cleanup.DeletePublishedBatch(context.Background(), 3, 10)
	if err != nil || deleted != 1 {
		t.Fatalf("unexpected cleanup delete: deleted=%d err=%v", deleted, err)
	}
	var remaining int
	if err := db.QueryRow(`SELECT COUNT(*) FROM outbox_events`).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 3 {
		t.Fatalf("expected unrelated, pending, and post-watermark rows to remain, got %d", remaining)
	}
}
