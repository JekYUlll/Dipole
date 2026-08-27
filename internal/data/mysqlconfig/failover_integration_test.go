package mysqlconfig_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func TestMySQLRouterWriterFailover(t *testing.T) {
	dsn := os.Getenv("DIPOLE_TEST_MYSQL_FAILOVER_DSN")
	if dsn == "" {
		t.Skip("DIPOLE_TEST_MYSQL_FAILOVER_DSN is required for MySQL failover integration test")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open MySQL Router: %v", err)
	}
	defer db.Close()
	db.SetMaxIdleConns(2)
	db.SetMaxOpenConns(4)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS mysql_ha_probe (
        marker VARCHAR(64) NOT NULL PRIMARY KEY,
        writer_uuid VARCHAR(64) NOT NULL,
        created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ) ENGINE=InnoDB`); err != nil {
		t.Fatalf("create failover probe table: %v", err)
	}

	firstWriter := currentWriter(t, ctx, db)
	before := fmt.Sprintf("before-%d", time.Now().UnixNano())
	insertProbe(t, ctx, db, before, firstWriter)
	t.Logf("MYSQL_HA_PRIMARY_READY=%s", firstWriter)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("writer did not fail over from %s: %v", firstWriter, ctx.Err())
		case <-ticker.C:
			writer, queryErr := queryWriter(ctx, db)
			if queryErr != nil || writer == firstWriter {
				continue
			}
			after := fmt.Sprintf("after-%d", time.Now().UnixNano())
			if _, execErr := db.ExecContext(ctx,
				"INSERT INTO mysql_ha_probe (marker, writer_uuid) VALUES (?, ?)", after, writer,
			); execErr != nil {
				continue
			}
			var count int
			if err := db.QueryRowContext(ctx,
				"SELECT COUNT(*) FROM mysql_ha_probe WHERE marker IN (?, ?)", before, after,
			).Scan(&count); err != nil || count != 2 {
				continue
			}
			t.Logf("MYSQL_HA_FAILOVER_OK=%s", writer)
			return
		}
	}
}

func currentWriter(t *testing.T, ctx context.Context, db *sql.DB) string {
	t.Helper()
	writer, err := queryWriter(ctx, db)
	if err != nil {
		t.Fatalf("query writer UUID: %v", err)
	}
	return writer
}

func queryWriter(ctx context.Context, db *sql.DB) (string, error) {
	var writer string
	err := db.QueryRowContext(ctx, "SELECT @@server_uuid").Scan(&writer)
	return writer, err
}

func insertProbe(t *testing.T, ctx context.Context, db *sql.DB, marker, writer string) {
	t.Helper()
	if _, err := db.ExecContext(ctx,
		"INSERT INTO mysql_ha_probe (marker, writer_uuid) VALUES (?, ?)", marker, writer,
	); err != nil {
		t.Fatalf("insert failover probe: %v", err)
	}
}
