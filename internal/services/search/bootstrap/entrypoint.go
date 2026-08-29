package bootstrap

import (
	"context"

	legacybootstrap "github.com/JekYUlll/Dipole/internal/bootstrap"
)

// Runtime aliases the compatibility runtime while Search bootstrap is being
// moved behind its service boundary.
type Runtime = legacybootstrap.SearchRuntime

// InitializeService keeps the Search entrypoint owned by the Search service.
// The underlying implementation remains in the compatibility bootstrap until
// shared RPC and readiness infrastructure has been extracted safely.
func InitializeService(ctx context.Context) (*Runtime, error) {
	return legacybootstrap.InitializeSearchService(ctx)
}
