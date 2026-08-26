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
	collector.ObserveClientSyncComparison("unexpected", 1)

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, family := range families {
		if family.GetName() != "dipole_web_sync_comparison_total" {
			continue
		}
		if len(family.Metric) != 6 {
			t.Fatalf("comparison metric count = %d", len(family.Metric))
		}
		outcomes := make(map[string]float64, len(family.Metric))
		for _, metric := range family.Metric {
			for _, label := range metric.Label {
				if label.GetName() == "outcome" {
					outcomes[label.GetValue()] = metric.GetCounter().GetValue()
				}
			}
		}
		for outcome, expected := range map[string]float64{
			"baseline":    0,
			"match":       3,
			"pending":     0,
			"legacy_only": 1,
			"sync_only":   0,
			"overflow":    0,
		} {
			if outcomes[outcome] != expected {
				t.Fatalf("comparison outcome %s = %v, want %v", outcome, outcomes[outcome], expected)
			}
		}
		return
	}
	t.Fatal("comparison metric family is missing")
}
