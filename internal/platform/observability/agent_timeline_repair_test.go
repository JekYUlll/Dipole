package observability

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestAgentTimelineRepairCollectorBoundsOutcomes(t *testing.T) {
	collector := NewAgentTimelineRepairCollector()
	collector.Observe("repaired", time.Millisecond)
	collector.Observe("projection_error", 0)
	collector.Observe("event_uuid_secret", time.Second)
	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(collector)
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	if len(families) != 2 {
		t.Fatalf("metric families=%d, want 2", len(families))
	}
}
