package bootstrap

import (
	"context"
	"database/sql"

	"github.com/JekYUlll/Dipole/internal/application"
)

// These aliases keep embedded and rollback callers source-compatible while
// Message runtime ownership moves into its service bootstrap.
type OutboxRelay = outboxRelay
type QueryOnlyMessageApplication = queryOnlyMessageApplication

func NewOutboxRelay(repo application.OutboxRelayStore) *OutboxRelay {
	return newOutboxRelay(repo)
}

func NewQueryOnlyMessageApplication(queries application.MessageQuery) *QueryOnlyMessageApplication {
	return newQueryOnlyMessageApplication(queries)
}

func VerifyMessageDatabaseBoundary(ctx context.Context, database interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, inboxWrites bool) error {
	return verifyMessageDatabaseBoundary(ctx, database, inboxWrites)
}
