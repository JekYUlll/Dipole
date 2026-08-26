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

type ClientSyncComparisonCollector struct {
	outcomes *prometheus.CounterVec
}

func NewClientSyncComparisonCollector() *ClientSyncComparisonCollector {
	collector := &ClientSyncComparisonCollector{outcomes: prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "dipole_web_sync_comparison_total",
		Help: "Web client Offline and Sync comparison observations by scope and terminal or sampled outcome.",
	}, []string{"scope", "outcome"})}
	for _, outcome := range clientSyncComparisonOutcomes {
		collector.outcomes.WithLabelValues(clientSyncComparisonScope, outcome).Add(0)
	}
	return collector
}

func (c *ClientSyncComparisonCollector) ObserveClientSyncComparison(outcome string, count int) {
	if c == nil || c.outcomes == nil || count <= 0 || !isClientSyncComparisonOutcome(outcome) {
		return
	}
	c.outcomes.WithLabelValues(clientSyncComparisonScope, outcome).Add(float64(count))
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
}

func (c *ClientSyncComparisonCollector) Collect(ch chan<- prometheus.Metric) {
	if c != nil && c.outcomes != nil {
		c.outcomes.Collect(ch)
	}
}
