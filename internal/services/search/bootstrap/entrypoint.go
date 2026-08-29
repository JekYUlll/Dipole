package bootstrap

import (
	"context"
)

type Runtime = SearchRuntime

// InitializeService starts the Search-owned runtime.
func InitializeService(ctx context.Context) (*Runtime, error) {
	return Initialize(ctx)
}
