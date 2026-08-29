package mysqlconfig

import (
	"net"
	"strconv"
	"time"

	"github.com/JekYUlll/Dipole/internal/config"
	mysqlDriver "github.com/go-sql-driver/mysql"
)

func DSN(cfg config.MySQL, multiStatements bool) string {
	driverConfig := mysqlDriver.NewConfig()
	driverConfig.User = cfg.User
	driverConfig.Passwd = cfg.Password
	driverConfig.Net = "tcp"
	driverConfig.Addr = net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	driverConfig.DBName = cfg.DBName
	driverConfig.ParseTime = true
	driverConfig.Loc = time.Local
	driverConfig.Collation = "utf8mb4_unicode_ci"
	driverConfig.MultiStatements = multiStatements
	return driverConfig.FormatDSN()
}
