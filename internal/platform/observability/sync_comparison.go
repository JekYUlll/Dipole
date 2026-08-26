package observability

import "github.com/prometheus/client_golang/prometheus"

type ClientSyncComparisonCollector struct {
	outcomes *prometheus.CounterVec
}

func NewClientSyncComparisonCollector() *ClientSyncComparisonCollector {
	return &ClientSyncComparisonCollector{outcomes: prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "dipole_web_sync_comparison_total",
		Help: "Web client Offline and Sync comparison observations by scope and terminal or sampled outcome.",
	}, []string{"scope", "outcome"})}
}

func (c *ClientSyncComparisonCollector) ObserveClientSyncComparison(outcome string, count int) {
	if c == nil || c.outcomes == nil || count <= 0 {
		return
	}
	c.outcomes.WithLabelValues("incoming_direct", outcome).Add(float64(count))
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
