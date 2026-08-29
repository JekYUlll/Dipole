package bootstrap

import (
	"context"
	"database/sql"

	"github.com/JekYUlll/Dipole/internal/application"
	messagemysql "github.com/JekYUlll/Dipole/internal/services/message/infrastructure/mysql"
)

// These aliases keep embedded and rollback callers source-compatible while
// Message runtime ownership moves into its service bootstrap.
type OutboxRelay = outboxRelay

func NewOutboxRelay(repo application.OutboxRelayStore) *OutboxRelay {
	return newOutboxRelay(repo)
}

func VerifyMessageDatabaseBoundary(ctx context.Context, database interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, inboxWrites bool) error {
	return messagemysql.VerifyDatabaseBoundary(ctx, database, inboxWrites)
}
