package mysql

import (
	"context"
	"errors"
	"fmt"

	searchcleanup "github.com/JekYUlll/Dipole/internal/cleanup/search"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
)

var _ searchcleanup.Store = (*SearchOutboxCleanupStore)(nil)

type SearchOutboxCleanupStore struct{ queries *generated.Queries }

func NewSearchOutboxCleanupStore(store *Store) (*SearchOutboxCleanupStore, error) {
	if store == nil {
		return nil, errors.New("Search Outbox cleanup MySQL store is required")
	}
	return &SearchOutboxCleanupStore{queries: store.Queries()}, nil
}

func (s *SearchOutboxCleanupStore) Inspect(ctx context.Context, highWatermark uint64) (uint64, uint64, error) {
	published, err := s.queries.CountPublishedSearchOutboxThrough(ctx, highWatermark)
	if err != nil {
		return 0, 0, err
	}
	nonPublished, err := s.queries.CountNonPublishedSearchOutboxThrough(ctx, highWatermark)
	return uint64(published), uint64(nonPublished), err
}

func (s *SearchOutboxCleanupStore) DeletePublishedBatch(ctx context.Context, highWatermark uint64, limit int) (uint64, error) {
	result, err := s.queries.DeletePublishedSearchOutboxBatch(ctx, generated.DeletePublishedSearchOutboxBatchParams{ThroughID: highWatermark, Limit: int32(limit)})
	if err != nil {
		return 0, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read Search Outbox cleanup result: %w", err)
	}
	return uint64(rows), nil
}
