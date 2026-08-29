package bootstrap

import (
	"context"

	legacybootstrap "github.com/JekYUlll/Dipole/internal/bootstrap"
)

// Runtime aliases the compatibility runtime while Search Indexer bootstrap is
// being moved behind its service boundary.
type Runtime = legacybootstrap.SearchIndexerRuntime

// InitializeService keeps the Search Indexer entrypoint owned by its service
// while preserving Kafka and Elasticsearch rollback semantics.
func InitializeService(ctx context.Context) (*Runtime, error) {
	return legacybootstrap.InitializeSearchIndexer(ctx)
}
