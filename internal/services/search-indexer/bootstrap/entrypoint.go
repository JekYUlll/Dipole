package bootstrap

import (
	"context"
)

// InitializeService starts the Search Indexer-owned runtime.
func InitializeService(ctx context.Context) (*SearchIndexerRuntime, error) {
	return Initialize(ctx)
}
