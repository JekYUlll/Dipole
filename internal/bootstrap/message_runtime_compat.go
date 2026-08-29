package bootstrap

import (
	"context"
	"database/sql"

	messagemysql "github.com/JekYUlll/Dipole/internal/services/message/infrastructure/mysql"
)

func VerifyMessageDatabaseBoundary(ctx context.Context, database interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, inboxWrites bool) error {
	return messagemysql.VerifyDatabaseBoundary(ctx, database, inboxWrites)
}
