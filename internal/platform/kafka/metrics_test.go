package kafka

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestConsumerCollectorExportsCumulativeOutcomes(t *testing.T) {
	consumer := &Consumer{clientID: "dipole-message", groupID: "dipole-message-consumer"}
	consumer.fetched.Add(7)
	consumer.handled.Add(6)
	consumer.committed.Add(5)
	consumer.fetchErrors.Add(1)
	consumer.commitErrors.Add(2)
	consumer.retryPublished.Add(3)
	consumer.deadPublished.Add(4)

	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(NewConsumerCollector(consumer))

	expected := map[string]float64{
		"dipole_kafka_consumer_fetched_total":               7,
		"dipole_kafka_consumer_handled_total":               6,
		"dipole_kafka_consumer_committed_total":             5,
		"dipole_kafka_consumer_fetch_errors_total":          1,
		"dipole_kafka_consumer_commit_errors_total":         2,
		"dipole_kafka_consumer_retry_published_total":       3,
		"dipole_kafka_consumer_dead_letter_published_total": 4,
	}
	metrics, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, family := range metrics {
		want, ok := expected[family.GetName()]
		if !ok {
			continue
		}
		if len(family.Metric) != 1 {
			t.Fatalf("metric %s count = %d, want 1", family.GetName(), len(family.Metric))
		}
		metric := family.Metric[0]
		if got := metric.GetCounter().GetValue(); got != want {
			t.Fatalf("metric %s = %v, want %v", family.GetName(), got, want)
		}
		labels := make(map[string]string, len(metric.Label))
		for _, pair := range metric.Label {
			labels[pair.GetName()] = pair.GetValue()
		}
		if labels["client_id"] != consumer.clientID || labels["group_id"] != consumer.groupID {
			t.Fatalf("metric %s labels = %+v", family.GetName(), labels)
		}
		delete(expected, family.GetName())
	}
	if len(expected) != 0 {
		t.Fatalf("missing metrics: %+v", expected)
	}
}
