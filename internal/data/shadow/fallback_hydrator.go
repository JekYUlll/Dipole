package shadow

import (
	"context"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
)

// FallbackSyncMessageHydrator makes Cassandra the opt-in read path while keeping MySQL as an immediate rollback path.
type FallbackSyncMessageHydrator struct {
	primary  application.SyncMessageHydrator
	fallback application.SyncMessageHydrator
	observe  func(string)
}

var _ application.SyncMessageHydrator = (*FallbackSyncMessageHydrator)(nil)

func NewFallbackSyncMessageHydrator(primary, fallback application.SyncMessageHydrator, observe func(string)) (*FallbackSyncMessageHydrator, error) {
	if primary == nil || fallback == nil {
		return nil, fmt.Errorf("Cassandra primary and MySQL fallback hydrators are required")
	}
	if observe == nil {
		observe = func(string) {}
	}
	return &FallbackSyncMessageHydrator{primary: primary, fallback: fallback, observe: observe}, nil
}

func (h *FallbackSyncMessageHydrator) Hydrate(ctx context.Context, locators []model.SyncMessageLocator) (map[string]*model.Message, error) {
	primary, primaryErr := h.primary.Hydrate(ctx, locators)
	if primaryErr == nil {
		h.observe("hit")
		return primary, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	fallback, fallbackErr := h.fallback.Hydrate(ctx, locators)
	if fallbackErr != nil {
		return nil, fmt.Errorf("Cassandra hydration failed and MySQL fallback failed: %w", fallbackErr)
	}
	h.observe("fallback")
	return fallback, nil
}
