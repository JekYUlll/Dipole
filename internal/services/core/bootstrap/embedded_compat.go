package bootstrap

import (
	"context"

	legacybootstrap "github.com/JekYUlll/Dipole/internal/bootstrap"
)

// EmbeddedRuntime aliases the compatibility aggregate runtime.
type EmbeddedRuntime = legacybootstrap.Runtime

// InitializeEmbedded preserves the local aggregate runtime as a rollback path.
func InitializeEmbedded(ctx context.Context) (*EmbeddedRuntime, error) {
	return legacybootstrap.Initialize(ctx)
}
