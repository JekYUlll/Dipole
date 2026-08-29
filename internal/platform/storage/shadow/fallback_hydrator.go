package shadow

import (
	"context"
	"fmt"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
)

// FallbackSyncMessageHydrator makes Cassandra the opt-in read path while keeping MySQL as an immediate rollback path.
type FallbackSyncMessageHydrator struct {
	primary  application.SyncMessageHydrator
	fallback application.SyncMessageHydrator
	observe  func(string)
	detailed func(SyncHydrationRouteObservation)
}

var _ application.SyncMessageHydrator = (*FallbackSyncMessageHydrator)(nil)

func NewFallbackSyncMessageHydrator(primary, fallback application.SyncMessageHydrator, observe func(string)) (*FallbackSyncMessageHydrator, error) {
	return newFallbackSyncMessageHydrator(primary, fallback, observe, nil)
}

func NewFallbackSyncMessageHydratorWithMetrics(primary, fallback application.SyncMessageHydrator, observe func(string), metrics *SyncHydrationMetrics) (*FallbackSyncMessageHydrator, error) {
	return newFallbackSyncMessageHydrator(primary, fallback, observe, metrics)
}

func newFallbackSyncMessageHydrator(primary, fallback application.SyncMessageHydrator, observe func(string), metrics *SyncHydrationMetrics) (*FallbackSyncMessageHydrator, error) {
	if primary == nil || fallback == nil {
		return nil, fmt.Errorf("Cassandra primary and MySQL fallback hydrators are required")
	}
	if observe == nil {
		observe = func(string) {}
	}
	detailed := func(observation SyncHydrationRouteObservation) {
		if metrics != nil {
			metrics.Observe(observation)
		}
	}
	return &FallbackSyncMessageHydrator{primary: primary, fallback: fallback, observe: observe, detailed: detailed}, nil
}

func (h *FallbackSyncMessageHydrator) Hydrate(ctx context.Context, locators []model.SyncMessageLocator) (map[string]*model.Message, error) {
	startedAt := time.Now()
	primary, primaryErr := h.primary.Hydrate(ctx, locators)
	if primaryErr == nil {
		h.observe("hit")
		h.detailed(SyncHydrationRouteObservation{Outcome: "hit", Duration: time.Since(startedAt)})
		return primary, nil
	}
	if err := ctx.Err(); err != nil {
		h.detailed(SyncHydrationRouteObservation{Outcome: "cancelled", Duration: time.Since(startedAt)})
		return nil, err
	}
	fallback, fallbackErr := h.fallback.Hydrate(ctx, locators)
	if fallbackErr != nil {
		h.detailed(SyncHydrationRouteObservation{Outcome: "error", Duration: time.Since(startedAt)})
		return nil, fmt.Errorf("Cassandra hydration failed and MySQL fallback failed: %w", fallbackErr)
	}
	h.observe("fallback")
	h.detailed(SyncHydrationRouteObservation{Outcome: "fallback", Duration: time.Since(startedAt)})
	return fallback, nil
}
