package observability

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestConversationProjectionCollectorUsesBoundedProjectionLabels(t *testing.T) {
	t.Parallel()

	collector := NewConversationProjectionCollector()
	collector.Observe("group_message")
	collector.Observe("group_message")
	collector.Observe("unknown")
	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(collector)

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather conversation metrics: %v", err)
	}
	writes := map[string]float64{}
	for _, family := range families {
		if family.GetName() != "dipole_conversation_projection_writes_total" {
			continue
		}
		for _, metric := range family.GetMetric() {
			projection := metric.GetLabel()[0].GetValue()
			writes[projection] = metric.GetCounter().GetValue()
		}
	}

	if len(writes) != 3 {
		t.Fatalf("projection label count = %d, want 3: %v", len(writes), writes)
	}
	if writes["group_message"] != 2 || writes["direct_message"] != 0 || writes["group_init"] != 0 {
		t.Fatalf("unexpected projection metrics: %v", writes)
	}
}
