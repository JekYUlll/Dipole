package bootstrap

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

func TestSyncDatabaseBoundaryWithMySQLAccount(t *testing.T) {
	dsn := os.Getenv("DIPOLE_TEST_SYNC_MYSQL_DSN")
	if dsn == "" {
		t.Skip("DIPOLE_TEST_SYNC_MYSQL_DSN is required for Sync permission integration tests")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open Sync MySQL account: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("ping Sync MySQL account: %v", err)
	}
	if err := verifySyncDatabaseBoundary(context.Background(), db); err != nil {
		t.Fatalf("verify real Sync MySQL boundary: %v", err)
	}
}
