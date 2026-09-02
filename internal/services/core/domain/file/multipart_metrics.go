package corefile

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// MultipartMetrics exposes bounded operational metrics without user, session,
// or object identifiers.
type MultipartMetrics struct {
	operations *prometheus.CounterVec
	duration   *prometheus.HistogramVec
}

func NewMultipartMetrics() *MultipartMetrics {
	return &MultipartMetrics{
		operations: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "dipole_multipart_operations_total",
			Help: "Multipart operations by operation and outcome.",
		}, []string{"operation", "outcome"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "dipole_multipart_operation_duration_seconds",
			Help: "Multipart operation duration by operation.",
		}, []string{"operation"}),
	}
}

func (m *MultipartMetrics) Observe(operation, outcome string, started time.Time) {
	if m == nil {
		return
	}
	m.ObserveOutcome(operation, outcome)
	m.duration.WithLabelValues(operation).Observe(time.Since(started).Seconds())
}

// ObserveOutcome records an additional bounded outcome without adding a second
// duration sample. It is used when one request has both retry and final result.
func (m *MultipartMetrics) ObserveOutcome(operation, outcome string) {
	if m == nil {
		return
	}
	m.operations.WithLabelValues(operation, outcome).Inc()
}

func (m *MultipartMetrics) Describe(ch chan<- *prometheus.Desc) {
	m.operations.Describe(ch)
	m.duration.Describe(ch)
}

func (m *MultipartMetrics) Collect(ch chan<- prometheus.Metric) {
	m.operations.Collect(ch)
	m.duration.Collect(ch)
}
