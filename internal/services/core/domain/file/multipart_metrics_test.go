package corefile

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestMultipartMetricsExposeBoundedOperationLabels(t *testing.T) {
	metrics := NewMultipartMetrics()
	registry := prometheus.NewRegistry()
	registry.MustRegister(metrics)
	metrics.Observe("upload_part", "success", time.Now())
	metrics.ObserveOutcome("upload_part", "retry")

	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	if len(families) != 2 {
		t.Fatalf("unexpected metric family count: %d", len(families))
	}
	var retryFound bool
	for _, family := range families {
		if family.GetName() != "dipole_multipart_operations_total" && family.GetName() != "dipole_multipart_operation_duration_seconds" {
			t.Fatalf("unexpected metric family: %s", family.GetName())
		}
		for _, metric := range family.GetMetric() {
			if family.GetName() == "dipole_multipart_operations_total" {
				for _, label := range metric.GetLabel() {
					if label.GetName() == "outcome" && label.GetValue() == "retry" && metric.GetCounter().GetValue() == 1 {
						retryFound = true
					}
				}
			}
			for _, label := range metric.GetLabel() {
				if label.GetName() == "operation" && label.GetValue() != "upload_part" {
					t.Fatalf("unexpected operation label: %s", label.GetValue())
				}
			}
		}
	}
	if !retryFound {
		t.Fatal("retry outcome was not observed")
	}
}
