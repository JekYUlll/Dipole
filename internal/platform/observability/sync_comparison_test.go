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
	collector.ObserveClientSyncComparison("storage_full", 2)
	collector.ObserveClientSyncComparison("timeline_match", 4)
	collector.ObserveClientSyncComparison("timeline_mismatch", 1)
	collector.ObserveClientSyncComparison("unexpected", 1)

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	found := make(map[string]bool)
	for _, family := range families {
		switch family.GetName() {
		case "dipole_web_sync_comparison_total":
			found[family.GetName()] = true
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
		case "dipole_web_sync_client_errors_total":
			found[family.GetName()] = true
			if len(family.Metric) != 2 {
				t.Fatalf("client error metric count = %d", len(family.Metric))
			}
			errors := make(map[string]float64, len(family.Metric))
			for _, metric := range family.Metric {
				for _, label := range metric.Label {
					if label.GetName() == "outcome" {
						errors[label.GetValue()] = metric.GetCounter().GetValue()
					}
				}
			}
			if errors["storage_full"] != 2 || errors["sync_error"] != 0 {
				t.Fatalf("unexpected client error metrics: %+v", errors)
			}
		case "dipole_web_timeline_notify_shadow_total":
			found[family.GetName()] = true
			if len(family.Metric) != 5 {
				t.Fatalf("timeline metric count = %d", len(family.Metric))
			}
			outcomes := make(map[string]float64, len(family.Metric))
			for _, metric := range family.Metric {
				for _, label := range metric.Label {
					if label.GetName() == "outcome" {
						outcomes[label.GetValue()] = metric.GetCounter().GetValue()
					}
				}
			}
			if outcomes["match"] != 4 || outcomes["mismatch"] != 1 || outcomes["missing"] != 0 {
				t.Fatalf("unexpected timeline metrics: %+v", outcomes)
			}
		}
	}
	if !found["dipole_web_sync_comparison_total"] || !found["dipole_web_sync_client_errors_total"] || !found["dipole_web_timeline_notify_shadow_total"] {
		t.Fatalf("missing Sync metric families: %+v", found)
	}
}
