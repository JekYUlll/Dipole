package bootstrap

import (
	"context"

	legacybootstrap "github.com/JekYUlll/Dipole/internal/bootstrap"
)

// Runtime aliases the compatibility runtime while Message bootstrap is being
// moved behind its service boundary.
type Runtime = legacybootstrap.MessageRuntime

// InitializeService keeps the Message entrypoint owned by the Message
// service while preserving the existing runtime and rollback semantics.
func InitializeService(ctx context.Context) (*Runtime, error) {
	return legacybootstrap.InitializeMessageService(ctx)
}
