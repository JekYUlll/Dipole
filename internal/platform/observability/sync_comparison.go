package observability

import "github.com/prometheus/client_golang/prometheus"

const clientSyncComparisonScope = "incoming_direct"

var clientSyncComparisonOutcomes = []string{
	"baseline",
	"match",
	"pending",
	"legacy_only",
	"sync_only",
	"overflow",
}

var clientSyncErrorOutcomes = []string{"storage_full", "sync_error"}
var timelineNotifyShadowOutcomes = []string{"match", "missing", "mismatch", "error", "invalid"}

type ClientSyncComparisonCollector struct {
	outcomes *prometheus.CounterVec
	errors   *prometheus.CounterVec
	timeline *prometheus.CounterVec
}

func NewClientSyncComparisonCollector() *ClientSyncComparisonCollector {
	collector := &ClientSyncComparisonCollector{outcomes: prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "dipole_web_sync_comparison_total",
		Help: "Web client Offline and Sync comparison observations by scope and terminal or sampled outcome.",
	}, []string{"scope", "outcome"})}
	collector.errors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "dipole_web_sync_client_errors_total",
		Help: "Web Sync client recovery failures by bounded outcome.",
	}, []string{"outcome"})
	collector.timeline = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "dipole_web_timeline_notify_shadow_total",
		Help: "Web Timeline notification shadow verification observations by bounded outcome.",
	}, []string{"outcome"})
	for _, outcome := range clientSyncComparisonOutcomes {
		collector.outcomes.WithLabelValues(clientSyncComparisonScope, outcome).Add(0)
	}
	for _, outcome := range clientSyncErrorOutcomes {
		collector.errors.WithLabelValues(outcome).Add(0)
	}
	for _, outcome := range timelineNotifyShadowOutcomes {
		collector.timeline.WithLabelValues(outcome).Add(0)
	}
	return collector
}

func (c *ClientSyncComparisonCollector) ObserveClientSyncComparison(outcome string, count int) {
	if c == nil || count <= 0 {
		return
	}
	if isClientSyncComparisonOutcome(outcome) && c.outcomes != nil {
		c.outcomes.WithLabelValues(clientSyncComparisonScope, outcome).Add(float64(count))
		return
	}
	if isClientSyncErrorOutcome(outcome) && c.errors != nil {
		c.errors.WithLabelValues(outcome).Add(float64(count))
		return
	}
	if timelineOutcome, ok := timelineNotifyShadowOutcome(outcome); ok && c.timeline != nil {
		c.timeline.WithLabelValues(timelineOutcome).Add(float64(count))
	}
}

func timelineNotifyShadowOutcome(candidate string) (string, bool) {
	const prefix = "timeline_"
	if len(candidate) <= len(prefix) || candidate[:len(prefix)] != prefix {
		return "", false
	}
	outcome := candidate[len(prefix):]
	for _, allowed := range timelineNotifyShadowOutcomes {
		if outcome == allowed {
			return outcome, true
		}
	}
	return "", false
}

func isClientSyncErrorOutcome(candidate string) bool {
	for _, outcome := range clientSyncErrorOutcomes {
		if candidate == outcome {
			return true
		}
	}
	return false
}

func isClientSyncComparisonOutcome(candidate string) bool {
	for _, outcome := range clientSyncComparisonOutcomes {
		if candidate == outcome {
			return true
		}
	}
	return false
}

func (c *ClientSyncComparisonCollector) Describe(ch chan<- *prometheus.Desc) {
	if c != nil && c.outcomes != nil {
		c.outcomes.Describe(ch)
	}
	if c != nil && c.errors != nil {
		c.errors.Describe(ch)
	}
	if c != nil && c.timeline != nil {
		c.timeline.Describe(ch)
	}
}

func (c *ClientSyncComparisonCollector) Collect(ch chan<- prometheus.Metric) {
	if c != nil && c.outcomes != nil {
		c.outcomes.Collect(ch)
	}
	if c != nil && c.errors != nil {
		c.errors.Collect(ch)
	}
	if c != nil && c.timeline != nil {
		c.timeline.Collect(ch)
	}
}
