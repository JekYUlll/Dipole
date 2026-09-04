package observability

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestAgentTaskWaitingCollectorBoundsOutcomes(t *testing.T) {
	collector := NewAgentTaskWaitingCollector()
	collector.Observe("online")
	collector.Observe("offline")
	collector.Observe("invalid")
	collector.Observe("principal-U100")
	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(collector)
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	if len(families) != 1 || len(families[0].Metric) != len(agentTaskWaitingOutcomes) {
		t.Fatalf("unexpected bounded locator metrics: %+v", families)
	}
}
