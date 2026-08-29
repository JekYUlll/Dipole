package testutil

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

// OpenTemporaryDatabase creates an isolated database for MySQL contract tests.
func OpenTemporaryDatabase(t testing.TB) *sql.DB {
	t.Helper()
	adminDSN := os.Getenv("DIPOLE_TEST_MYSQL_ADMIN_DSN")
	if adminDSN == "" {
		t.Skip("DIPOLE_TEST_MYSQL_ADMIN_DSN is required for MySQL integration tests")
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

	databaseName := fmt.Sprintf("dipole_operation_contract_%d", time.Now().UnixNano())
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
