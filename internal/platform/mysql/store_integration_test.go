package mysql_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/model"
	mysqlStore "github.com/JekYUlll/Dipole/internal/platform/mysql"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/mapper"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/migration"
	mysqlDriver "github.com/go-sql-driver/mysql"
)

func TestStoreWithinTxCommitAndRollback(t *testing.T) {
	db := openTemporaryDatabase(t)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatalf("create migration runner: %v", err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatalf("migrate temporary database: %v", err)
	}

	store, err := mysqlStore.NewStore(db)
	if err != nil {
		t.Fatalf("create sqlc store: %v", err)
	}
	params := mapper.AICallLogInsertParams(&model.AICallLog{
		TriggerMessageUUID: "M-sqlc-tx",
		ConversationKey:    "direct:U100:UAI",
		UserUUID:           "U100",
		AssistantUUID:      "UAI",
		Provider:           "test",
		Model:              "test-model",
		Status:             model.AICallStatusPending,
	})

	rollbackErr := errors.New("rollback requested")
	err = store.WithinTx(context.Background(), nil, func(queries *generated.Queries) error {
		if _, err := queries.InsertAICallLog(context.Background(), params); err != nil {
			return err
		}
		return rollbackErr
	})
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("expected rollback error, got %v", err)
	}
	assertAICallLogCount(t, db, 0)

	if err := store.WithinTx(context.Background(), nil, func(queries *generated.Queries) error {
		rows, err := queries.InsertAICallLog(context.Background(), params)
		if err != nil {
			return err
		}
		if rows != 1 {
			return fmt.Errorf("expected one inserted row, got %d", rows)
		}
		return nil
	}); err != nil {
		t.Fatalf("commit transaction: %v", err)
	}
	assertAICallLogCount(t, db, 1)

	rows, err := store.Queries().InsertAICallLog(context.Background(), params)
	if err != nil {
		t.Fatalf("repeat idempotent insert: %v", err)
	}
	if rows != 0 {
		t.Fatalf("expected duplicate insert to affect zero rows, got %d", rows)
	}
}

func openTemporaryDatabase(t *testing.T) *sql.DB {
	t.Helper()
	adminDSN := os.Getenv("DIPOLE_TEST_MYSQL_ADMIN_DSN")
	if adminDSN == "" {
		t.Skip("DIPOLE_TEST_MYSQL_ADMIN_DSN is required for sqlc store integration tests")
	}
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

	databaseName := fmt.Sprintf("dipole_sqlc_store_%d", time.Now().UnixNano())
	if _, err := adminDB.Exec("CREATE DATABASE `" + databaseName + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		t.Fatalf("create temporary database: %v", err)
	}
	t.Cleanup(func() {
		_, _ = adminDB.Exec("DROP DATABASE IF EXISTS `" + databaseName + "`")
	})

	targetConfig := adminConfig.Clone()
	targetConfig.DBName = databaseName
	targetConfig.MultiStatements = true
	db, err := sql.Open("mysql", targetConfig.FormatDSN())
	if err != nil {
		t.Fatalf("open temporary database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func assertAICallLogCount(t *testing.T, db *sql.DB, expected int) {
	t.Helper()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM ai_call_logs WHERE trigger_message_uuid = ?", "M-sqlc-tx").Scan(&count); err != nil {
		t.Fatalf("count AI call logs: %v", err)
	}
	if count != expected {
		t.Fatalf("expected %d AI call log rows, got %d", expected, count)
	}
}
