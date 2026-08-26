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
		assertCurrentVersion(t, runner, 2)
		if err := runner.ValidateCurrent(ctx); err != nil {
			t.Fatalf("validate current schema: %v", err)
		}
		assertTableCount(t, db, 14)

		if err := runner.Up(ctx); err != nil {
			t.Fatalf("repeat migration: %v", err)
		}
		assertMigrationCount(t, db, 2)
		if _, err := db.Exec("INSERT INTO schema_migrations (version, name) VALUES (3, 'future_expand')"); err != nil {
			t.Fatalf("insert future migration: %v", err)
		}
		if err := runner.ValidateCurrent(ctx); err != nil {
			t.Fatalf("expected rolling deployment to accept a future migration: %v", err)
		}
		if _, err := db.Exec("DELETE FROM schema_migrations WHERE version = 3"); err != nil {
			t.Fatalf("remove future migration: %v", err)
		}

		if err := runner.Down(ctx, 1); err != nil {
			t.Fatalf("roll back baseline: %v", err)
		}
		assertCurrentVersion(t, runner, 1)
		if err := runner.ValidateCurrent(ctx); err == nil {
			t.Fatal("expected rolled-back database validation to fail")
		}
		assertTableCount(t, db, 13)

		if err := runner.Down(ctx, 1); err != nil {
			t.Fatalf("roll back baseline: %v", err)
		}
		assertCurrentVersion(t, runner, 0)
		assertTableCount(t, db, 1)
	})

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
		uuid, clientID, conversationKey string
	}{
		{"MSEQ1", "CMSEQ1", "direct:U1:U2"},
		{"MSEQ2", "CMSEQ2", "direct:U1:U2"},
		{"MSEQ3", "CMSEQ3", "group:G1"},
	} {
		if _, err := db.Exec(`INSERT INTO messages
            (uuid, client_message_id, conversation_key, sender_uuid, target_uuid, content, sent_at)
            VALUES (?, ?, ?, 'U1', 'U2', 'seed', NOW(3))`, seed.uuid, seed.clientID, seed.conversationKey); err != nil {
			t.Fatalf("seed legacy message: %v", err)
		}
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
