package observability

import (
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestConversationProjectionCollectorUsesBoundedProjectionLabels(t *testing.T) {
	t.Parallel()

	collector := NewConversationProjectionCollector()
	collector.Observe("group_message", 15*time.Millisecond, nil)
	collector.Observe("group_message", 25*time.Millisecond, nil)
	collector.Observe("direct_message", 5*time.Millisecond, errors.New("write failed"))
	collector.Observe("unknown", 10*time.Millisecond, nil)
	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(collector)

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather conversation metrics: %v", err)
	}
	writes := map[string]float64{}
	histogramCounts := map[string]uint64{}
	for _, family := range families {
		for _, metric := range family.GetMetric() {
			labels := map[string]string{}
			for _, label := range metric.GetLabel() {
				labels[label.GetName()] = label.GetValue()
			}
			switch family.GetName() {
			case "dipole_conversation_projection_writes_total":
				writes[labels["projection"]] = metric.GetCounter().GetValue()
			case "dipole_conversation_projection_write_duration_seconds":
				histogramCounts[labels["projection"]+":"+labels["outcome"]] = metric.GetHistogram().GetSampleCount()
			}
		}
	}

	if len(writes) != 3 {
		t.Fatalf("projection label count = %d, want 3: %v", len(writes), writes)
	}
	if writes["group_message"] != 2 || writes["direct_message"] != 0 || writes["group_init"] != 0 {
		t.Fatalf("unexpected projection metrics: %v", writes)
	}
	if len(histogramCounts) != 6 {
		t.Fatalf("projection/outcome label count = %d, want 6: %v", len(histogramCounts), histogramCounts)
	}
	if histogramCounts["group_message:success"] != 2 || histogramCounts["direct_message:error"] != 1 {
		t.Fatalf("unexpected projection duration metrics: %v", histogramCounts)
	}
}
