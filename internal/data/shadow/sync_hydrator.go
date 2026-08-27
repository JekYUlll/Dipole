package shadow

import (
	"context"
	"sync"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/prometheus/client_golang/prometheus"
)

type SyncHydrationComparison struct {
	Match        bool
	Skipped      bool
	SkipReason   string
	PrimaryCount int
	ShadowCount  int
	ShadowError  string
}

type SyncMessageHydrator struct {
	primary     application.SyncMessageHydrator
	shadow      application.SyncMessageHydrator
	observe     func(SyncHydrationComparison)
	slots       chan struct{}
	work        sync.WaitGroup
	comparisons *prometheus.CounterVec
	latency     *prometheus.HistogramVec
}

var _ application.SyncMessageHydrator = (*SyncMessageHydrator)(nil)

func NewSyncMessageHydrator(primary, shadow application.SyncMessageHydrator, observe func(SyncHydrationComparison)) *SyncMessageHydrator {
	return newSyncMessageHydrator(primary, shadow, observe, defaultMaxConcurrentComparisons)
}

func newSyncMessageHydrator(primary, shadow application.SyncMessageHydrator, observe func(SyncHydrationComparison), maxConcurrent int) *SyncMessageHydrator {
	if observe == nil {
		observe = func(SyncHydrationComparison) {}
	}
	if maxConcurrent <= 0 {
		maxConcurrent = defaultMaxConcurrentComparisons
	}
	return &SyncMessageHydrator{
		primary: primary, shadow: shadow, observe: observe, slots: make(chan struct{}, maxConcurrent),
		comparisons: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "dipole_sync_hydration_shadow_total", Help: "Sync message hydration shadow comparisons by outcome."}, []string{"outcome"}),
		latency:     prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "dipole_sync_hydration_shadow_duration_seconds", Help: "Cassandra Sync hydration shadow latency.", Buckets: prometheus.DefBuckets}, []string{"outcome"}),
	}
}

func (h *SyncMessageHydrator) Hydrate(ctx context.Context, locators []model.SyncMessageLocator) (map[string]*model.Message, error) {
	primary, err := h.primary.Hydrate(ctx, locators)
	if err != nil {
		return nil, err
	}
	comparison := SyncHydrationComparison{PrimaryCount: len(primary)}
	if len(locators) == 0 {
		comparison.Skipped = true
		comparison.SkipReason = "empty_locator_page"
		h.record(comparison, 0)
		return primary, nil
	}
	select {
	case h.slots <- struct{}{}:
	default:
		comparison.Skipped = true
		comparison.SkipReason = "shadow_capacity_exhausted"
		h.record(comparison, 0)
		return primary, nil
	}
	locatorSnapshot := append([]model.SyncMessageLocator(nil), locators...)
	primarySnapshot := cloneMessageMap(primary)
	h.work.Add(1)
	go func() {
		startedAt := time.Now()
		defer h.work.Done()
		defer func() { <-h.slots }()
		shadow, shadowErr := h.shadow.Hydrate(context.Background(), locatorSnapshot)
		comparison.ShadowCount = len(shadow)
		if shadowErr != nil {
			comparison.ShadowError = shadowErr.Error()
		} else {
			comparison.Match = equalHydratedMessages(locatorSnapshot, primarySnapshot, shadow)
		}
		h.record(comparison, time.Since(startedAt).Seconds())
	}()
	return primary, nil
}

func (h *SyncMessageHydrator) record(comparison SyncHydrationComparison, duration float64) {
	outcome := "mismatch"
	if comparison.Skipped {
		outcome = "skipped"
	} else if comparison.ShadowError != "" {
		outcome = "error"
	} else if comparison.Match {
		outcome = "match"
	}
	h.comparisons.WithLabelValues(outcome).Inc()
	if !comparison.Skipped {
		h.latency.WithLabelValues(outcome).Observe(duration)
	}
	h.observe(comparison)
}

func (h *SyncMessageHydrator) Describe(descriptions chan<- *prometheus.Desc) {
	h.comparisons.Describe(descriptions)
	h.latency.Describe(descriptions)
}

func (h *SyncMessageHydrator) Collect(metrics chan<- prometheus.Metric) {
	h.comparisons.Collect(metrics)
	h.latency.Collect(metrics)
}

func (h *SyncMessageHydrator) Wait() {
	if h != nil {
		h.work.Wait()
	}
}

func cloneMessageMap(messages map[string]*model.Message) map[string]*model.Message {
	result := make(map[string]*model.Message, len(messages))
	for id, message := range messages {
		if message == nil {
			continue
		}
		copy := *message
		if message.FileExpiresAt != nil {
			value := *message.FileExpiresAt
			copy.FileExpiresAt = &value
		}
		result[id] = &copy
	}
	return result
}

func equalHydratedMessages(locators []model.SyncMessageLocator, primary, shadow map[string]*model.Message) bool {
	if len(primary) != len(shadow) {
		return false
	}
	for _, locator := range locators {
		if !equalHydratedMessage(primary[locator.MessageUUID], shadow[locator.MessageUUID]) {
			return false
		}
	}
	return true
}

func equalHydratedMessage(left, right *model.Message) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.UUID == right.UUID && left.ClientMessageID == right.ClientMessageID &&
		left.ConversationKey == right.ConversationKey && left.Seq == right.Seq &&
		left.SenderUUID == right.SenderUUID && left.TargetType == right.TargetType &&
		left.TargetUUID == right.TargetUUID && left.MessageType == right.MessageType &&
		left.Content == right.Content && left.FileID == right.FileID && left.FileName == right.FileName &&
		left.FileSize == right.FileSize && left.FileURL == right.FileURL &&
		left.FileContentType == right.FileContentType && equalHydrationTime(left.FileExpiresAt, right.FileExpiresAt) &&
		left.SentAt.Equal(right.SentAt)
}

func equalHydrationTime(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.Equal(*right)
}
