package bootstrap

import (
	"context"

	legacybootstrap "github.com/JekYUlll/Dipole/internal/bootstrap"
)

// Runtime aliases the compatibility runtime while Sync bootstrap is being
// moved behind its service boundary.
type Runtime = legacybootstrap.SyncRuntime

// InitializeService keeps the Sync entrypoint owned by the Sync service while
// preserving the existing projector and hydration rollback semantics.
func InitializeService(ctx context.Context) (*Runtime, error) {
	return legacybootstrap.InitializeSyncService(ctx)
}
