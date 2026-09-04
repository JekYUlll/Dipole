package observability

import "github.com/prometheus/client_golang/prometheus"

var agentTaskWaitingOutcomes = []string{"online", "offline", "invalid"}

// AgentTaskWaitingCollector records low-cardinality Gateway delivery outcomes.
// A locator is advisory: the Task Inbox remains the authoritative pull source.
type AgentTaskWaitingCollector struct{ outcomes *prometheus.CounterVec }

func NewAgentTaskWaitingCollector() *AgentTaskWaitingCollector {
	c := &AgentTaskWaitingCollector{outcomes: prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "dipole_agent_task_waiting_locator_total",
		Help: "Agent Task waiting locator delivery attempts by bounded outcome.",
	}, []string{"outcome"})}
	for _, outcome := range agentTaskWaitingOutcomes {
		c.outcomes.WithLabelValues(outcome)
	}
	return c
}

func (c *AgentTaskWaitingCollector) Observe(outcome string) {
	if c == nil {
		return
	}
	for _, allowed := range agentTaskWaitingOutcomes {
		if outcome == allowed {
			c.outcomes.WithLabelValues(outcome).Inc()
			return
		}
	}
}

func (c *AgentTaskWaitingCollector) Describe(ch chan<- *prometheus.Desc) { c.outcomes.Describe(ch) }

func (c *AgentTaskWaitingCollector) Collect(ch chan<- prometheus.Metric) { c.outcomes.Collect(ch) }
