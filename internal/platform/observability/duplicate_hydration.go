package observability

import "github.com/prometheus/client_golang/prometheus"

var duplicateHydrationOutcomes = []string{"hit", "fallback", "skipped_no_seq"}

type DuplicateHydrationCollector struct{ outcomes *prometheus.CounterVec }

func NewDuplicateHydrationCollector() *DuplicateHydrationCollector {
	collector := &DuplicateHydrationCollector{outcomes: prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "dipole_message_duplicate_hydration_total",
		Help: "Duplicate message body hydration attempts by bounded outcome.",
	}, []string{"outcome"})}
	for _, outcome := range duplicateHydrationOutcomes {
		collector.outcomes.WithLabelValues(outcome)
	}
	return collector
}

func (c *DuplicateHydrationCollector) Observe(outcome string) {
	if c == nil {
		return
	}
	for _, allowed := range duplicateHydrationOutcomes {
		if outcome == allowed {
			c.outcomes.WithLabelValues(outcome).Inc()
			return
		}
	}
}

func (c *DuplicateHydrationCollector) Describe(ch chan<- *prometheus.Desc) { c.outcomes.Describe(ch) }
func (c *DuplicateHydrationCollector) Collect(ch chan<- prometheus.Metric) { c.outcomes.Collect(ch) }
