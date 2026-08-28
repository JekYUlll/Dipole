package repository_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/data/migration"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	sqlcRepository "github.com/JekYUlll/Dipole/internal/data/mysql/repository"
	"github.com/JekYUlll/Dipole/internal/model"
	mysqlDriver "github.com/go-sql-driver/mysql"
)

func TestAICallLogRepositoryContract(t *testing.T) {
	db, _ := openContractDatabase(t)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatalf("create migration runner: %v", err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatalf("migrate contract database: %v", err)
	}

	sqlcRepo, err := sqlcRepository.NewAICallLogRepository(generated.New(db))
	if err != nil {
		t.Fatalf("create sqlc repository: %v", err)
	}

	t.Run("sqlc", func(t *testing.T) {
		runAICallLogContract(t, db, sqlcRepo, "sqlc")
	})
}

func runAICallLogContract(t *testing.T, db *sql.DB, store application.AICallLogStore, prefix string) {
	t.Helper()
	if started, err := store.Begin(nil); err != nil || started {
		t.Fatalf("nil Begin: started=%v err=%v", started, err)
	}

	successID := prefix + "-success"
	log := &model.AICallLog{
		TriggerMessageUUID: successID,
		ConversationKey:    "direct:U100:UAI",
		UserUUID:           "U100",
		AssistantUUID:      "UAI",
		Provider:           "contract",
		Model:              "contract-model",
		Status:             model.AICallStatusPending,
	}
	started, err := store.Begin(log)
	if err != nil || !started {
		t.Fatalf("first Begin: started=%v err=%v", started, err)
	}
	started, err = store.Begin(log)
	if err != nil || started {
		t.Fatalf("duplicate Begin: started=%v err=%v", started, err)
	}
	if err := store.MarkSucceeded(successID, prefix+"-response", 11, 12, 23, 45); err != nil {
		t.Fatalf("mark succeeded: %v", err)
	}
	assertCallLog(t, db, successID, model.AICallStatusSucceeded, prefix+"-response", "", 11, 12, 23, 45)

	failedID := prefix + "-failed"
	failedLog := *log
	failedLog.ID = 0
	failedLog.TriggerMessageUUID = failedID
	if started, err := store.Begin(&failedLog); err != nil || !started {
		t.Fatalf("failed-case Begin: started=%v err=%v", started, err)
	}
	if err := store.MarkFailed(failedID, "contract failure", 67); err != nil {
		t.Fatalf("mark failed: %v", err)
	}
	assertCallLog(t, db, failedID, model.AICallStatusFailed, "", "contract failure", 0, 0, 0, 67)
}

func assertCallLog(t *testing.T, db *sql.DB, trigger string, status int8, response, errorMessage string, prompt, completion, total int, latency int64) {
	t.Helper()
	var gotStatus int8
	var gotResponse, gotError string
	var gotPrompt, gotCompletion, gotTotal int
	var gotLatency int64
	err := db.QueryRow(`SELECT status, response_message_uuid, error_message, prompt_tokens, completion_tokens, total_tokens, latency_ms
FROM ai_call_logs WHERE trigger_message_uuid = ?`, trigger).Scan(
		&gotStatus, &gotResponse, &gotError, &gotPrompt, &gotCompletion, &gotTotal, &gotLatency,
	)
	if err != nil {
		t.Fatalf("read call log %s: %v", trigger, err)
	}
	if gotStatus != status || gotResponse != response || gotError != errorMessage || gotPrompt != prompt || gotCompletion != completion || gotTotal != total || gotLatency != latency {
		t.Fatalf("unexpected call log %s: status=%d response=%q error=%q tokens=%d/%d/%d latency=%d", trigger, gotStatus, gotResponse, gotError, gotPrompt, gotCompletion, gotTotal, gotLatency)
	}
}

func openContractDatabase(t testing.TB) (*sql.DB, string) {
	t.Helper()
	adminDSN := os.Getenv("DIPOLE_TEST_MYSQL_ADMIN_DSN")
	if adminDSN == "" {
		t.Skip("DIPOLE_TEST_MYSQL_ADMIN_DSN is required for repository contract tests")
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

	databaseName := fmt.Sprintf("dipole_ai_contract_%d", time.Now().UnixNano())
	if _, err := adminDB.Exec("CREATE DATABASE `" + databaseName + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		t.Fatalf("create contract database: %v", err)
	}
	t.Cleanup(func() { _, _ = adminDB.Exec("DROP DATABASE IF EXISTS `" + databaseName + "`") })

	targetConfig := adminConfig.Clone()
	targetConfig.DBName = databaseName
	targetConfig.MultiStatements = true
	dsn := targetConfig.FormatDSN()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open contract database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, dsn
}
