package observability

import "github.com/prometheus/client_golang/prometheus"

var conversationProjectionNames = []string{"direct_message", "group_init", "group_message"}

type ConversationProjectionCollector struct {
	writes *prometheus.CounterVec
}

func NewConversationProjectionCollector() *ConversationProjectionCollector {
	collector := &ConversationProjectionCollector{writes: prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "dipole_conversation_projection_writes_total",
		Help: "Successful Conversation State SQL upserts by projection path.",
	}, []string{"projection"})}
	for _, projection := range conversationProjectionNames {
		collector.writes.WithLabelValues(projection).Add(0)
	}
	return collector
}

func (c *ConversationProjectionCollector) Observe(projection string) {
	for _, allowed := range conversationProjectionNames {
		if projection == allowed {
			c.writes.WithLabelValues(projection).Inc()
			return
		}
	}
}

func (c *ConversationProjectionCollector) Describe(ch chan<- *prometheus.Desc) {
	c.writes.Describe(ch)
}

func (c *ConversationProjectionCollector) Collect(ch chan<- prometheus.Metric) {
	c.writes.Collect(ch)
}
