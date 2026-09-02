package bootstrap

import (
	"context"
)

type Runtime = SyncRuntime

// InitializeService starts the Sync-owned runtime.
func InitializeService(ctx context.Context) (*Runtime, error) {
	return Initialize(ctx)
}
