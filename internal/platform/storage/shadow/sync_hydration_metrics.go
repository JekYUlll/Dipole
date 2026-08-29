package shadow

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// SyncHydrationRouteObservation is the low-cardinality outcome of one hydration attempt.
type SyncHydrationRouteObservation struct {
	Outcome  string
	Duration time.Duration
}

// SyncHydrationMetrics exposes runtime evidence for the Cassandra primary path.
type SyncHydrationMetrics struct {
	routes  *prometheus.CounterVec
	latency *prometheus.HistogramVec
}

func NewSyncHydrationMetrics() *SyncHydrationMetrics {
	return &SyncHydrationMetrics{
		routes:  prometheus.NewCounterVec(prometheus.CounterOpts{Name: "dipole_sync_hydration_route_total", Help: "Sync message hydration requests by selected route outcome."}, []string{"outcome"}),
		latency: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "dipole_sync_hydration_route_duration_seconds", Help: "Sync message hydration request duration by route outcome.", Buckets: prometheus.DefBuckets}, []string{"outcome"}),
	}
}

func (m *SyncHydrationMetrics) Observe(observation SyncHydrationRouteObservation) {
	if m == nil {
		return
	}
	outcome := observation.Outcome
	switch outcome {
	case "hit", "fallback", "error", "cancelled":
	default:
		outcome = "error"
	}
	m.routes.WithLabelValues(outcome).Inc()
	m.latency.WithLabelValues(outcome).Observe(observation.Duration.Seconds())
}

func (m *SyncHydrationMetrics) Describe(descriptions chan<- *prometheus.Desc) {
	if m == nil {
		return
	}
	m.routes.Describe(descriptions)
	m.latency.Describe(descriptions)
}

func (m *SyncHydrationMetrics) Collect(metrics chan<- prometheus.Metric) {
	if m == nil {
		return
	}
	m.routes.Collect(metrics)
	m.latency.Collect(metrics)
}
