package observability

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var agentTimelineRepairOutcomes = []string{"repaired", "projection_error", "complete_error", "claim_error", "invalid", "empty"}

type AgentTimelineRepairCollector struct {
	outcomes *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

func NewAgentTimelineRepairCollector() *AgentTimelineRepairCollector {
	c := &AgentTimelineRepairCollector{
		outcomes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "dipole_agent_task_timeline_repair_total",
			Help: "Agent Task Timeline repair attempts by bounded outcome.",
		}, []string{"outcome"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "dipole_agent_task_timeline_repair_duration_seconds",
			Help: "Agent Task Timeline repair operation duration by bounded outcome.",
		}, []string{"outcome"}),
	}
	for _, outcome := range agentTimelineRepairOutcomes {
		c.outcomes.WithLabelValues(outcome)
		c.duration.WithLabelValues(outcome)
	}
	return c
}

func (c *AgentTimelineRepairCollector) Observe(outcome string, duration time.Duration) {
	if c == nil {
		return
	}
	for _, allowed := range agentTimelineRepairOutcomes {
		if outcome == allowed {
			c.outcomes.WithLabelValues(outcome).Inc()
			if duration > 0 {
				c.duration.WithLabelValues(outcome).Observe(duration.Seconds())
			}
			return
		}
	}
}

func (c *AgentTimelineRepairCollector) Describe(ch chan<- *prometheus.Desc) {
	c.outcomes.Describe(ch)
	c.duration.Describe(ch)
}

func (c *AgentTimelineRepairCollector) Collect(ch chan<- prometheus.Metric) {
	c.outcomes.Collect(ch)
	c.duration.Collect(ch)
}
