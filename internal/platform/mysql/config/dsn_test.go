package mysqlconfig

import (
	"testing"

	"github.com/JekYUlll/Dipole/internal/config"
	mysqlDriver "github.com/go-sql-driver/mysql"
)

func TestDSNConfiguresMigrationStatements(t *testing.T) {
	t.Parallel()

	dsn := DSN(config.MySQL{
		Host:     "db.internal",
		Port:     3307,
		User:     "dipole",
		Password: "secret",
		DBName:   "dipole",
	}, true)
	parsed, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("parse DSN: %v", err)
	}
	if parsed.Addr != "db.internal:3307" || parsed.DBName != "dipole" {
		t.Fatalf("unexpected DSN target: %+v", parsed)
	}
	if !parsed.ParseTime || !parsed.MultiStatements {
		t.Fatalf("expected parseTime and multiStatements, got %+v", parsed)
	}
}
