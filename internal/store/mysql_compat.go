package store

import (
	"database/sql"

	"github.com/JekYUlll/Dipole/internal/config"
	platformmysql "github.com/JekYUlll/Dipole/internal/platform/mysql"
)

// SQLDB remains available for legacy tools. New services use platform/mysql
// directly so connection ownership stays outside the aggregate store package.
var SQLDB *sql.DB

func InitMySQL() error {
	return InitMySQLWithConfig(config.MySQLConfig())
}

func InitMySQLWithConfig(cfg config.MySQL) error {
	if err := platformmysql.InitMySQLWithConfig(cfg); err != nil {
		return err
	}
	SQLDB = platformmysql.SQLDB
	return nil
}
