package bootstrap

import (
	"context"

	embeddedbootstrap "github.com/JekYUlll/Dipole/internal/bootstrap/embedded/runtime"
)

// EmbeddedRuntime aliases the compatibility aggregate runtime.
type EmbeddedRuntime = embeddedbootstrap.Runtime

// InitializeEmbedded preserves the local aggregate runtime as a rollback path.
func InitializeEmbedded(ctx context.Context) (*EmbeddedRuntime, error) {
	return embeddedbootstrap.Initialize(ctx)
}
