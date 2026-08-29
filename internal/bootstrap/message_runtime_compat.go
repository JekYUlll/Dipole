package bootstrap

import (
	"context"
	"database/sql"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	platformobservability "github.com/JekYUlll/Dipole/internal/platform/observability"
)

// These aliases keep embedded and rollback callers source-compatible while
// Message runtime ownership moves into its service bootstrap.
type LazyCoreCapability = lazyCoreCapability
type OutboxRelay = outboxRelay
type QueryOnlyMessageApplication = queryOnlyMessageApplication

func NewLazyCoreCapability(cfg config.InternalRPC) *LazyCoreCapability {
	return newLazyCoreCapability(cfg)
}

func LazyCoreCapabilityReadinessProbe(name string, capability *LazyCoreCapability) platformobservability.DependencyProbe {
	return lazyCoreCapabilityReadinessProbe(name, capability)
}

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
