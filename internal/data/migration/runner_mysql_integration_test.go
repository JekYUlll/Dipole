package migration_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/data/migration"
	mysqlDriver "github.com/go-sql-driver/mysql"
)

func TestMySQLBaselineMigration(t *testing.T) {
	adminDSN := os.Getenv("DIPOLE_TEST_MYSQL_ADMIN_DSN")
	if adminDSN == "" {
		t.Skip("DIPOLE_TEST_MYSQL_ADMIN_DSN is required for migration integration tests")
	}

	t.Run("empty database", func(t *testing.T) {
		db := openTemporaryDatabase(t, adminDSN, "empty")
		runner, err := migration.NewRunner(db, migrations.Files)
		if err != nil {
			t.Fatalf("create migration runner: %v", err)
		}

		ctx := context.Background()
		if err := runner.ValidateCurrent(ctx); err == nil {
			t.Fatal("expected empty database validation to fail")
		}
		if err := runner.Up(ctx); err != nil {
			t.Fatalf("migrate empty database: %v", err)
		}
		assertCurrentVersion(t, runner, 24)
		if err := runner.ValidateCurrent(ctx); err != nil {
			t.Fatalf("validate current schema: %v", err)
		}
		assertTableCount(t, db, 33)

		if err := runner.Up(ctx); err != nil {
			t.Fatalf("repeat migration: %v", err)
		}
		assertMigrationCount(t, db, 24)
		if _, err := db.Exec("INSERT INTO schema_migrations (version, name) VALUES (25, 'future_expand')"); err != nil {
			t.Fatalf("insert future migration: %v", err)
		}
		if err := runner.ValidateCurrent(ctx); err != nil {
			t.Fatalf("expected rolling deployment to accept a future migration: %v", err)
		}
		if _, err := db.Exec("DELETE FROM schema_migrations WHERE version = 25"); err != nil {
			t.Fatalf("remove future migration: %v", err)
		}
		if err := runner.Down(ctx, 1); err != nil {
			t.Fatalf("roll back Agent Task Workflow projection migration: %v", err)
		}
		assertCurrentVersion(t, runner, 23)
		assertTableCount(t, db, 33)

		if err := runner.Down(ctx, 1); err != nil {
			t.Fatalf("roll back Agent Model output replay migration: %v", err)
		}
		assertCurrentVersion(t, runner, 22)
		assertTableCount(t, db, 33)

		if err := runner.Down(ctx, 1); err != nil {
			t.Fatalf("roll back Agent Context manifest migration: %v", err)
		}
		assertCurrentVersion(t, runner, 21)
		assertTableCount(t, db, 33)

		if err := runner.Down(ctx, 1); err != nil {
			t.Fatalf("roll back Agent Run migration: %v", err)
		}
		assertCurrentVersion(t, runner, 20)
		assertTableCount(t, db, 32)

		if err := runner.Down(ctx, 1); err != nil {
			t.Fatalf("roll back Agent Shadow trajectory migration: %v", err)
		}
		assertCurrentVersion(t, runner, 19)
		assertTableCount(t, db, 30)

		if err := runner.Down(ctx, 1); err != nil {
			t.Fatalf("roll back Agent Model Audit migration: %v", err)
		}
		assertCurrentVersion(t, runner, 18)
		assertTableCount(t, db, 28)

		if err := runner.Down(ctx, 1); err != nil {
			t.Fatalf("roll back Agent Event Ledger migration: %v", err)
		}
		assertCurrentVersion(t, runner, 17)
		assertTableCount(t, db, 27)

		if err := runner.Down(ctx, 1); err != nil {
			t.Fatalf("roll back Agent Policy identity width migration: %v", err)
		}
		assertCurrentVersion(t, runner, 16)
		assertTableCount(t, db, 27)

		if err := runner.Down(ctx, 1); err != nil {
			t.Fatalf("roll back Agent Policy persistence migration: %v", err)
		}
		assertCurrentVersion(t, runner, 15)
		assertTableCount(t, db, 24)

		if err := runner.Down(ctx, 1); err != nil {
			t.Fatalf("roll back Cassandra source identity migration: %v", err)
		}
		assertCurrentVersion(t, runner, 14)
		assertTableCount(t, db, 24)

		if err := runner.Down(ctx, 1); err != nil {
			t.Fatalf("roll back Message Metadata legacy ID migration: %v", err)
		}
		assertCurrentVersion(t, runner, 13)
		assertTableCount(t, db, 24)

		if err := runner.Down(ctx, 1); err != nil {
			t.Fatalf("roll back Search source identity migration: %v", err)
		}
		assertCurrentVersion(t, runner, 12)
		assertTableCount(t, db, 24)

		if err := runner.Down(ctx, 1); err != nil {
			t.Fatalf("roll back Message Metadata migration: %v", err)
		}
		assertCurrentVersion(t, runner, 11)
		assertTableCount(t, db, 23)

		if err := runner.Down(ctx, 1); err != nil {
			t.Fatalf("roll back Sync Inbox baseline migration: %v", err)
		}
		assertCurrentVersion(t, runner, 10)
		assertTableCount(t, db, 21)

		if err := runner.Down(ctx, 1); err != nil {
			t.Fatalf("roll back Sync replay job migration: %v", err)
		}
		assertCurrentVersion(t, runner, 9)
		assertTableCount(t, db, 20)

		if err := runner.Down(ctx, 1); err != nil {
			t.Fatalf("roll back Sync locator migration: %v", err)
		}
		assertCurrentVersion(t, runner, 8)
		assertTableCount(t, db, 20)

		if err := runner.Down(ctx, 1); err != nil {
			t.Fatalf("roll back Search backfill migration: %v", err)
		}
		assertCurrentVersion(t, runner, 7)
		if err := runner.ValidateCurrent(ctx); err == nil {
			t.Fatal("expected rolled-back database validation to fail")
		}
		assertTableCount(t, db, 19)

		if err := runner.Down(ctx, 1); err != nil {
			t.Fatalf("roll back versioned search mutation migration: %v", err)
		}
		assertCurrentVersion(t, runner, 6)
		assertTableCount(t, db, 19)

		if err := runner.Down(ctx, 1); err != nil {
			t.Fatalf("roll back Cassandra backfill migration: %v", err)
		}
		assertCurrentVersion(t, runner, 5)
		assertTableCount(t, db, 18)

		if err := runner.Down(ctx, 1); err != nil {
			t.Fatalf("roll back hot group migration: %v", err)
		}
		assertCurrentVersion(t, runner, 4)
		assertTableCount(t, db, 16)

		if err := runner.Down(ctx, 1); err != nil {
			t.Fatalf("roll back search index migration: %v", err)
		}
		assertCurrentVersion(t, runner, 3)
		assertTableCount(t, db, 15)

		if err := runner.Down(ctx, 1); err != nil {
			t.Fatalf("roll back read checkpoint migration: %v", err)
		}
		assertCurrentVersion(t, runner, 2)
		assertTableCount(t, db, 14)

		if err := runner.Down(ctx, 1); err != nil {
			t.Fatalf("roll back conversation sequence migration: %v", err)
		}
		assertCurrentVersion(t, runner, 1)
		assertTableCount(t, db, 13)

		if err := runner.Down(ctx, 1); err != nil {
			t.Fatalf("roll back baseline: %v", err)
		}
		assertCurrentVersion(t, runner, 0)
		assertTableCount(t, db, 1)
	})

}

func TestMessageMetadataMigrationBackfillsExistingMessages(t *testing.T) {
	adminDSN := os.Getenv("DIPOLE_TEST_MYSQL_ADMIN_DSN")
	if adminDSN == "" {
		t.Skip("DIPOLE_TEST_MYSQL_ADMIN_DSN is required for migration integration tests")
	}
	db := openTemporaryDatabase(t, adminDSN, "message_metadata")
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatalf("create migration runner: %v", err)
	}
	ctx := context.Background()
	if err := runner.Up(ctx); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	if err := runner.Down(ctx, 3); err != nil {
		t.Fatalf("roll back Metadata legacy ID, Search source, and Metadata migrations: %v", err)
	}
	inserted, err := db.Exec(`INSERT INTO messages (
		uuid, client_message_id, conversation_key, seq, sender_uuid, target_type,
		target_uuid, message_type, content, file_id, file_expires_at, sent_at
	) VALUES ('M-meta-legacy', 'CM-meta-legacy', 'direct:U1:U2', 7, 'U1', 0,
		'U2', 1, '', 'F-meta-legacy', '2026-08-28 12:00:00.000', '2026-08-27 12:00:00.000')`)
	if err != nil {
		t.Fatalf("seed legacy message: %v", err)
	}
	legacyID, err := inserted.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Up(ctx); err != nil {
		t.Fatalf("apply metadata migration: %v", err)
	}

	var messageUUID, clientMessageID, fileID, payloadSHA string
	var messageSeq, metadataLegacyID uint64
	var expiresAt time.Time
	if err := db.QueryRow(`SELECT message_uuid, legacy_message_id, client_message_id, message_seq, file_id,
		file_expires_at, payload_sha256 FROM message_metadata WHERE message_uuid = 'M-meta-legacy'`).
		Scan(&messageUUID, &metadataLegacyID, &clientMessageID, &messageSeq, &fileID, &expiresAt, &payloadSHA); err != nil {
		t.Fatalf("read backfilled metadata: %v", err)
	}
	if messageUUID != "M-meta-legacy" || metadataLegacyID != uint64(legacyID) || clientMessageID != "CM-meta-legacy" || messageSeq != 7 ||
		fileID != "F-meta-legacy" || expiresAt.IsZero() || payloadSHA != "" {
		t.Fatalf("unexpected backfilled metadata: uuid=%s client=%s seq=%d file=%s expires=%s hash=%q",
			messageUUID, clientMessageID, messageSeq, fileID, expiresAt, payloadSHA)
	}
}

func TestSearchSourceIdentityMigrationBackfillsExistingJobs(t *testing.T) {
	adminDSN := os.Getenv("DIPOLE_TEST_MYSQL_ADMIN_DSN")
	if adminDSN == "" {
		t.Skip("DIPOLE_TEST_MYSQL_ADMIN_DSN is required for migration integration tests")
	}
	db := openTemporaryDatabase(t, adminDSN, "search_source_identity")
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatalf("create migration runner: %v", err)
	}
	ctx := context.Background()
	if err := runner.Up(ctx); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	if err := runner.Down(ctx, 1); err != nil {
		t.Fatalf("roll back source identity migration: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO search_backfill_jobs (
		job_name, target_index, status, source_high_watermark_id, last_processed_id, last_error
	) VALUES ('legacy-search-v1', 'dipole-messages-v1-build-a', 'completed', 42, 42, '')`); err != nil {
		t.Fatalf("insert legacy Search job: %v", err)
	}
	if err := runner.Up(ctx); err != nil {
		t.Fatalf("apply source identity migration: %v", err)
	}
	var kind, snapshotID, hash string
	if err := db.QueryRow(`SELECT source_kind, source_snapshot_id, source_sha256
		FROM search_backfill_jobs WHERE job_name = 'legacy-search-v1'`).Scan(&kind, &snapshotID, &hash); err != nil {
		t.Fatalf("read migrated Search source identity: %v", err)
	}
	if kind != "mysql_outbox" || snapshotID != "mysql-outbox:42" || hash != "" {
		t.Fatalf("unexpected migrated Search source identity: kind=%s snapshot=%s hash=%s", kind, snapshotID, hash)
	}
}

func TestMySQLMigrationRunnerSerializesConcurrentOwners(t *testing.T) {
	adminDSN := os.Getenv("DIPOLE_TEST_MYSQL_ADMIN_DSN")
	if adminDSN == "" {
		t.Skip("DIPOLE_TEST_MYSQL_ADMIN_DSN is required for migration integration tests")
	}
	db, dsn := openTemporaryDatabaseWithDSN(t, adminDSN, "concurrent_owner")
	secondDB, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open second migration connection: %v", err)
	}
	t.Cleanup(func() { _ = secondDB.Close() })

	runners := make([]*migration.Runner, 2)
	for index, connection := range []*sql.DB{db, secondDB} {
		runners[index], err = migration.NewRunner(connection, migrations.Files)
		if err != nil {
			t.Fatalf("create migration runner %d: %v", index, err)
		}
	}

	start := make(chan struct{})
	errors := make(chan error, len(runners))
	for _, runner := range runners {
		go func() {
			<-start
			errors <- runner.Up(context.Background())
		}()
	}
	close(start)
	for range runners {
		if err := <-errors; err != nil {
			t.Fatalf("concurrent migration failed: %v", err)
		}
	}
	assertMigrationCount(t, db, 13)
}

func TestConversationSequenceMigrationBackfillsPerConversation(t *testing.T) {
	adminDSN := os.Getenv("DIPOLE_TEST_MYSQL_ADMIN_DSN")
	if adminDSN == "" {
		t.Skip("DIPOLE_TEST_MYSQL_ADMIN_DSN is required for migration integration tests")
	}
	db := openTemporaryDatabase(t, adminDSN, "conversation_sequence")
	baseline, err := migrations.Files.ReadFile("000001_baseline.up.sql")
	if err != nil {
		t.Fatalf("read baseline migration: %v", err)
	}
	if _, err := db.Exec(string(baseline)); err != nil {
		t.Fatalf("apply baseline migration: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE schema_migrations (
        version BIGINT NOT NULL PRIMARY KEY,
        name VARCHAR(255) NOT NULL,
        applied_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    )`); err != nil {
		t.Fatalf("create migration ledger: %v", err)
	}
	if _, err := db.Exec("INSERT INTO schema_migrations (version, name) VALUES (1, 'baseline')"); err != nil {
		t.Fatalf("record baseline migration: %v", err)
	}
	for _, seed := range []struct {
		uuid, clientID, conversationKey, targetUUID string
		targetType                                  int8
	}{
		{"MSEQ1", "CMSEQ1", "direct:U1:U2", "U2", 0},
		{"MSEQ2", "CMSEQ2", "direct:U1:U2", "U2", 0},
		{"MSEQ3", "CMSEQ3", "group:G1", "G1", 1},
	} {
		if _, err := db.Exec(`INSERT INTO messages
	            (uuid, client_message_id, conversation_key, sender_uuid, target_type, target_uuid, content, sent_at)
	            VALUES (?, ?, ?, 'U1', ?, ?, 'seed', NOW(3))`, seed.uuid, seed.clientID, seed.conversationKey, seed.targetType, seed.targetUUID); err != nil {
			t.Fatalf("seed legacy message: %v", err)
		}
	}
	if _, err := db.Exec(`INSERT INTO user_sync_inbox (user_uuid, message_uuid, conversation_key, created_at)
        VALUES ('U2', 'MSEQ2', 'direct:U1:U2', NOW(3))`); err != nil {
		t.Fatalf("seed legacy Sync Inbox: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO conversations
        (user_uuid, target_uuid, conversation_key, last_message_uuid, last_message_at, unread_count)
        VALUES
        ('U2', 'U1', 'direct:U1:U2', 'MSEQ2', NOW(3), 1),
        ('U3', 'G1', 'group:G1', '', NOW(3), 0)`); err != nil {
		t.Fatalf("seed legacy conversations: %v", err)
	}
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatalf("create migration runner: %v", err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatalf("apply conversation sequence migration: %v", err)
	}

	rows, err := db.Query("SELECT conversation_key, seq FROM messages ORDER BY id")
	if err != nil {
		t.Fatalf("query backfilled messages: %v", err)
	}
	defer rows.Close()
	want := []struct {
		key string
		seq uint64
	}{{"direct:U1:U2", 1}, {"direct:U1:U2", 2}, {"group:G1", 1}}
	rowCount := 0
	for rows.Next() {
		index := rowCount
		if index >= len(want) {
			t.Fatal("backfill returned too many messages")
		}
		var key string
		var seq uint64
		if err := rows.Scan(&key, &seq); err != nil {
			t.Fatalf("scan backfilled message: %v", err)
		}
		if key != want[index].key || seq != want[index].seq {
			t.Fatalf("backfill[%d] = (%s,%d), want (%s,%d)", index, key, seq, want[index].key, want[index].seq)
		}
		rowCount++
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate backfilled messages: %v", err)
	}
	if rowCount != len(want) {
		t.Fatalf("backfill returned %d messages, want %d", rowCount, len(want))
	}
	for key, expected := range map[string]uint64{"direct:U1:U2": 2, "group:G1": 1} {
		var lastSeq uint64
		if err := db.QueryRow("SELECT last_seq FROM conversation_sequences WHERE conversation_key = ?", key).Scan(&lastSeq); err != nil {
			t.Fatalf("read allocator %s: %v", key, err)
		}
		if lastSeq != expected {
			t.Fatalf("allocator %s = %d, want %d", key, lastSeq, expected)
		}
	}
	var lastMessageSeq, readSeq uint64
	if err := db.QueryRow("SELECT last_message_seq, read_seq FROM conversations WHERE user_uuid = 'U2'").Scan(&lastMessageSeq, &readSeq); err != nil {
		t.Fatalf("read conversation positions: %v", err)
	}
	if lastMessageSeq != 2 || readSeq != 1 {
		t.Fatalf("conversation positions = last:%d read:%d, want last:2 read:1", lastMessageSeq, readSeq)
	}
	var groupLatestSeq uint64
	var groupLatestMessageUUID string
	if err := db.QueryRow("SELECT latest_message_seq, latest_message_uuid FROM group_sync_states WHERE group_uuid = 'G1'").Scan(&groupLatestSeq, &groupLatestMessageUUID); err != nil {
		t.Fatalf("read backfilled group sync state: %v", err)
	}
	if groupLatestSeq != 1 || groupLatestMessageUUID != "MSEQ3" {
		t.Fatalf("group sync state = seq:%d message:%s, want seq:1 message:MSEQ3", groupLatestSeq, groupLatestMessageUUID)
	}
	var inboxMessageSeq uint64
	if err := db.QueryRow("SELECT message_seq FROM user_sync_inbox WHERE user_uuid = 'U2' AND message_uuid = 'MSEQ2'").Scan(&inboxMessageSeq); err != nil {
		t.Fatalf("read backfilled Sync locator: %v", err)
	}
	if inboxMessageSeq != 2 {
		t.Fatalf("backfilled Sync message sequence = %d, want 2", inboxMessageSeq)
	}
}

func openTemporaryDatabase(t *testing.T, adminDSN, suffix string) *sql.DB {
	t.Helper()
	db, _ := openTemporaryDatabaseWithDSN(t, adminDSN, suffix)
	return db
}

func openTemporaryDatabaseWithDSN(t *testing.T, adminDSN, suffix string) (*sql.DB, string) {
	t.Helper()

	adminConfig, err := mysqlDriver.ParseDSN(adminDSN)
	if err != nil {
		t.Fatalf("parse admin DSN: %v", err)
	}
	adminConfig.DBName = ""
	adminDB, err := sql.Open("mysql", adminConfig.FormatDSN())
	if err != nil {
		t.Fatalf("open admin database: %v", err)
	}
	t.Cleanup(func() { _ = adminDB.Close() })

	databaseName := fmt.Sprintf("dipole_migration_%s_%d", suffix, time.Now().UnixNano())
	if _, err := adminDB.Exec("CREATE DATABASE `" + databaseName + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		t.Fatalf("create temporary database: %v", err)
	}
	t.Cleanup(func() {
		_, _ = adminDB.Exec("DROP DATABASE IF EXISTS `" + databaseName + "`")
	})

	targetConfig := adminConfig.Clone()
	targetConfig.DBName = databaseName
	targetConfig.MultiStatements = true
	dsn := targetConfig.FormatDSN()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open temporary database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatalf("ping temporary database: %v", err)
	}
	return db, dsn
}

func assertCurrentVersion(t *testing.T, runner *migration.Runner, expected int64) {
	t.Helper()
	version, err := runner.CurrentVersion(context.Background())
	if err != nil {
		t.Fatalf("read current version: %v", err)
	}
	if version != expected {
		t.Fatalf("expected migration version %d, got %d", expected, version)
	}
}

func assertTableCount(t *testing.T, db *sql.DB, expected int) {
	t.Helper()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE()").Scan(&count); err != nil {
		t.Fatalf("count tables: %v", err)
	}
	if count != expected {
		t.Fatalf("expected %d tables, got %d", expected, count)
	}
}

func assertMigrationCount(t *testing.T, db *sql.DB, expected int) {
	t.Helper()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if count != expected {
		t.Fatalf("expected %d migration rows, got %d", expected, count)
	}
}
