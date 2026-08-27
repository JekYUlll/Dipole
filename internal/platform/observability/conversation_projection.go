package observability

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var conversationProjectionNames = []string{"direct_message", "group_init", "group_message"}

type ConversationProjectionCollector struct {
	writes   *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

func NewConversationProjectionCollector() *ConversationProjectionCollector {
	collector := &ConversationProjectionCollector{
		writes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "dipole_conversation_projection_writes_total",
			Help: "Successful Conversation State SQL upserts by projection path.",
		}, []string{"projection"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "dipole_conversation_projection_write_duration_seconds",
			Help: "Conversation State repository call duration by projection path and outcome.",
			Buckets: []float64{
				0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1,
				0.25, 0.5, 1, 2.5, 5, 10,
			},
		}, []string{"projection", "outcome"}),
	}
	for _, projection := range conversationProjectionNames {
		collector.writes.WithLabelValues(projection).Add(0)
		collector.duration.WithLabelValues(projection, "success")
		collector.duration.WithLabelValues(projection, "error")
	}
	return collector
}

func (c *ConversationProjectionCollector) Observe(projection string, duration time.Duration, err error) {
	for _, allowed := range conversationProjectionNames {
		if projection == allowed {
			outcome := "success"
			if err != nil {
				outcome = "error"
			} else {
				c.writes.WithLabelValues(projection).Inc()
			}
			c.duration.WithLabelValues(projection, outcome).Observe(duration.Seconds())
			return
		}
	}
}

func (c *ConversationProjectionCollector) Describe(ch chan<- *prometheus.Desc) {
	c.writes.Describe(ch)
	c.duration.Describe(ch)
}

func (c *ConversationProjectionCollector) Collect(ch chan<- prometheus.Metric) {
	c.writes.Collect(ch)
	c.duration.Collect(ch)
}
