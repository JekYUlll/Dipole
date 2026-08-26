package observability

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestClientSyncComparisonCollector(t *testing.T) {
	collector := NewClientSyncComparisonCollector()
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)
	collector.ObserveClientSyncComparison("match", 3)
	collector.ObserveClientSyncComparison("legacy_only", 1)

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, family := range families {
		if family.GetName() != "dipole_web_sync_comparison_total" {
			continue
		}
		if len(family.Metric) != 2 {
			t.Fatalf("comparison metric count = %d", len(family.Metric))
		}
		return
	}
	t.Fatal("comparison metric family is missing")
}
