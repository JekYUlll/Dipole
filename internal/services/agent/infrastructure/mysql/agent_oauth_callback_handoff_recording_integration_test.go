package agentmysql

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/application"
	mysqlData "github.com/JekYUlll/Dipole/internal/platform/mysql"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/migration"
	mysqlDriver "github.com/go-sql-driver/mysql"
)

func TestAgentOAuthCallbackHandoffRecordingRepositoryIsAtomic(t *testing.T) {
	db := openAgentOAuthCallbackTemporaryDatabase(t)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatalf("create migration runner: %v", err)
	}
	if err = runner.Up(context.Background()); err != nil {
		t.Fatalf("migrate temporary database: %v", err)
	}
	store, err := mysqlData.NewStore(db)
	if err != nil {
		t.Fatalf("create SQLC store: %v", err)
	}
	recorder, err := NewAgentOAuthCallbackHandoffRecordingRepository(store)
	if err != nil {
		t.Fatalf("create callback recorder: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	transactionID, handoffID := strings.Repeat("a", 22), strings.Repeat("b", 22)
	state, codeHash := strings.Repeat("c", 64), strings.Repeat("d", 64)
	insertOAuthAuthorizationTransaction(t, store, transactionID, "U100", state, now.Add(time.Hour))
	// A callback can only occur after Core persisted its authorization transaction.
	callbackAt := now.Add(time.Second)
	request := application.AgentOAuthCallbackHandoffRecordRequestV1{
		HandoffUUID: handoffID, TransactionUUID: transactionID, OwnerUserUUID: "U100", StateSHA256: state,
		AuthorizationCodeSHA256: codeHash, SealedAuthorizationCode: "v1.abc.def.ghi", RuntimeKeyID: "oauth-runtime-1",
	}
	recorded, created, err := recorder.RecordAgentOAuthCallbackHandoff(context.Background(), request, callbackAt)
	if err != nil || !created || recorded == nil || recorded.TransactionUUID != transactionID || recorded.ExpiresAt.Before(callbackAt) {
		t.Fatalf("record=%+v created=%v err=%v", recorded, created, err)
	}
	assertOAuthAuthorizationConsumed(t, db, transactionID, true)
	assertOAuthCallbackHandoffCount(t, db, 1)

	replayed, created, err := recorder.RecordAgentOAuthCallbackHandoff(context.Background(), request, callbackAt.Add(time.Second))
	if err != nil || created || replayed == nil || replayed.HandoffUUID != handoffID {
		t.Fatalf("replay=%+v created=%v err=%v", replayed, created, err)
	}
	assertOAuthCallbackHandoffCount(t, db, 1)

	conflictingTransaction := strings.Repeat("e", 22)
	insertOAuthAuthorizationTransaction(t, store, conflictingTransaction, "U100", strings.Repeat("f", 64), now.Add(time.Hour))
	conflict := request
	conflict.TransactionUUID, conflict.StateSHA256 = conflictingTransaction, strings.Repeat("f", 64)
	if _, _, err = recorder.RecordAgentOAuthCallbackHandoff(context.Background(), conflict, callbackAt.Add(2*time.Second)); err == nil {
		t.Fatal("expected unique handoff conflict")
	}
	assertOAuthAuthorizationConsumed(t, db, conflictingTransaction, false)
	assertOAuthCallbackHandoffCount(t, db, 1)
}

func openAgentOAuthCallbackTemporaryDatabase(t *testing.T) *sql.DB {
	t.Helper()
	adminDSN := os.Getenv("DIPOLE_TEST_MYSQL_ADMIN_DSN")
	if adminDSN == "" {
		t.Skip("DIPOLE_TEST_MYSQL_ADMIN_DSN is required for Agent OAuth callback integration tests")
	}
	config, err := mysqlDriver.ParseDSN(adminDSN)
	if err != nil {
		t.Fatalf("parse admin DSN: %v", err)
	}
	config.DBName = ""
	admin, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatalf("open admin database: %v", err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	database := fmt.Sprintf("dipole_agent_oauth_%d", time.Now().UnixNano())
	if _, err = admin.Exec("CREATE DATABASE `" + database + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		t.Fatalf("create temporary database: %v", err)
	}
	t.Cleanup(func() { _, _ = admin.Exec("DROP DATABASE IF EXISTS `" + database + "`") })
	config.DBName, config.MultiStatements = database, true
	db, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatalf("open temporary database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func insertOAuthAuthorizationTransaction(t *testing.T, store *mysqlData.Store, transactionID, owner, state string, expiresAt time.Time) {
	t.Helper()
	_, err := store.Queries().InsertAgentOAuthAuthorizationTransaction(context.Background(), generated.InsertAgentOAuthAuthorizationTransactionParams{
		TransactionUuid: transactionID, OwnerUserUuid: owner, Issuer: "https://auth.example.com", RedirectUri: "https://dipole.example.com/oauth/callback",
		StateSha256: state, SealedCodeVerifier: "v1.abc.def.ghi", ExpiresAt: expiresAt,
	})
	if err != nil {
		t.Fatalf("insert authorization transaction: %v", err)
	}
}

func assertOAuthAuthorizationConsumed(t *testing.T, db *sql.DB, transactionID string, want bool) {
	t.Helper()
	var consumed sql.NullTime
	if err := db.QueryRow("SELECT consumed_at FROM agent_oauth_authorization_transactions WHERE transaction_uuid = ?", transactionID).Scan(&consumed); err != nil {
		t.Fatalf("load authorization transaction: %v", err)
	}
	if consumed.Valid != want {
		t.Fatalf("transaction consumed=%v want=%v", consumed.Valid, want)
	}
}

func assertOAuthCallbackHandoffCount(t *testing.T, db *sql.DB, want int) {
	t.Helper()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM agent_oauth_callback_handoffs").Scan(&count); err != nil {
		t.Fatalf("count callback handoffs: %v", err)
	}
	if count != want {
		t.Fatalf("handoff count=%d want=%d", count, want)
	}
}
