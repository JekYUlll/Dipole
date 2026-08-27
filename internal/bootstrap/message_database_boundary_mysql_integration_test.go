package bootstrap

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

func TestMessageProjectorDatabaseBoundaryWithMySQLAccount(t *testing.T) {
	verifyMessageDatabaseAccount(t, "DIPOLE_TEST_MESSAGE_PROJECTOR_MYSQL_DSN", false)
}

func TestMessageAtomicDatabaseBoundaryWithMySQLAccount(t *testing.T) {
	verifyMessageDatabaseAccount(t, "DIPOLE_TEST_MESSAGE_ATOMIC_MYSQL_DSN", true)
}

func verifyMessageDatabaseAccount(t *testing.T, environment string, inboxWrites bool) {
	t.Helper()
	dsn := os.Getenv(environment)
	if dsn == "" {
		t.Skip(environment + " is required for Message permission integration tests")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open Message projector MySQL account: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("ping Message projector MySQL account: %v", err)
	}
	if err := verifyMessageDatabaseBoundary(context.Background(), db, inboxWrites); err != nil {
		t.Fatalf("verify real Message MySQL boundary: %v", err)
	}
}
