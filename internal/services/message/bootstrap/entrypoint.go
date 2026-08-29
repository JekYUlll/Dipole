package bootstrap

import (
	"context"
)

type Runtime = MessageRuntime

// InitializeService starts the Message-owned runtime.
func InitializeService(ctx context.Context) (*Runtime, error) {
	return Initialize(ctx)
}
