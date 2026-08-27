package observability

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestDuplicateHydrationCollectorBoundsAndPreRegistersOutcomes(t *testing.T) {
	collector := NewDuplicateHydrationCollector()
	collector.Observe("hit")
	collector.Observe("unbounded")
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	if len(families) != 1 || len(families[0].Metric) != 3 {
		t.Fatalf("unexpected bounded metrics: %+v", families)
	}
}
