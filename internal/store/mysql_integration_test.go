package store

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

func TestInitLegacyGORMReusesSQLConnectionPool(t *testing.T) {
	adminDSN := os.Getenv("DIPOLE_TEST_MYSQL_ADMIN_DSN")
	if adminDSN == "" {
		t.Skip("DIPOLE_TEST_MYSQL_ADMIN_DSN is required for MySQL integration tests")
	}
	config, err := mysqlDriver.ParseDSN(adminDSN)
	if err != nil {
		t.Fatalf("parse admin DSN: %v", err)
	}
	config.DBName = ""
	adminDB, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatalf("open admin database: %v", err)
	}
	defer adminDB.Close()
	databaseName := fmt.Sprintf("dipole_store_contract_%d", time.Now().UnixNano())
	if _, err := adminDB.Exec("CREATE DATABASE `" + databaseName + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		t.Fatalf("create contract database: %v", err)
	}
	defer func() { _, _ = adminDB.Exec("DROP DATABASE IF EXISTS `" + databaseName + "`") }()
	config.DBName = databaseName
	db, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatalf("open contract database: %v", err)
	}
	defer db.Close()

	oldSQLDB, oldDB := SQLDB, DB
	SQLDB, DB = db, nil
	defer func() { SQLDB, DB = oldSQLDB, oldDB }()
	if err := InitLegacyGORM(); err != nil {
		t.Fatalf("initialize legacy GORM: %v", err)
	}
	var value int
	if err := DB.Raw("SELECT 1").Scan(&value).Error; err != nil {
		t.Fatalf("query through legacy GORM: %v", err)
	}
	if value != 1 {
		t.Fatalf("unexpected query result: %d", value)
	}
	gormSQLDB, err := DB.DB()
	if err != nil {
		t.Fatalf("read GORM SQL pool: %v", err)
	}
	if gormSQLDB != SQLDB {
		t.Fatal("legacy GORM created a second connection pool")
	}
}
