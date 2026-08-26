package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	gormMySQL "gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/JekYUlll/Dipole/internal/config"
)

var (
	SQLDB *sql.DB
	DB    *gorm.DB // Legacy rollback and AutoMigrate only.
)

func InitMySQL() error {
	cfg := config.MySQLConfig()
	driverConfig := mysqlDriver.NewConfig()
	driverConfig.User = cfg.User
	driverConfig.Passwd = cfg.Password
	driverConfig.Net = "tcp"
	driverConfig.Addr = fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	driverConfig.DBName = cfg.DBName
	driverConfig.ParseTime = true
	driverConfig.Loc = time.Local
	driverConfig.Params = map[string]string{"charset": "utf8mb4"}

	db, err := sql.Open("mysql", driverConfig.FormatDSN())
	if err != nil {
		return fmt.Errorf("open mysql: %w", err)
	}
	db.SetMaxIdleConns(5)
	db.SetMaxOpenConns(20)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return fmt.Errorf("ping mysql: %w", err)
	}

	SQLDB = db
	DB = nil
	return nil
}

func InitLegacyGORM() error {
	if SQLDB == nil {
		return fmt.Errorf("mysql not initialized")
	}
	db, err := gorm.Open(gormMySQL.New(gormMySQL.Config{Conn: SQLDB}), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("open legacy gorm adapter: %w", err)
	}
	DB = db
	return nil
}
