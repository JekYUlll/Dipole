package bootstrap

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

func TestMessageProjectorDatabaseBoundaryWithMySQLAccount(t *testing.T) {
	dsn := os.Getenv("DIPOLE_TEST_MESSAGE_PROJECTOR_MYSQL_DSN")
	if dsn == "" {
		t.Skip("DIPOLE_TEST_MESSAGE_PROJECTOR_MYSQL_DSN is required for Message permission integration tests")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open Message projector MySQL account: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("ping Message projector MySQL account: %v", err)
	}
	if err := verifyMessageDatabaseBoundary(context.Background(), db, false); err != nil {
		t.Fatalf("verify real Message projector MySQL boundary: %v", err)
	}
}
