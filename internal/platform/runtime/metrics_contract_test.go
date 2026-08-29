package runtime

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/prometheus/client_golang/prometheus"
)

type typedNilCollector struct{}

var typedNilCollectorCalls atomic.Int32

func (c *typedNilCollector) Describe(chan<- *prometheus.Desc) {
	if c == nil {
		typedNilCollectorCalls.Add(1)
	}
}

func (c *typedNilCollector) Collect(chan<- prometheus.Metric) {
	if c == nil {
		typedNilCollectorCalls.Add(1)
	}
}

func TestRuntimeMetricsDisabledDoesNotListen(t *testing.T) {
	server, err := StartMetrics(config.Metrics{Enabled: false}, "dipole-test", nil)
	if err != nil || server != nil {
		t.Fatalf("disabled metrics server = %v, err = %v", server, err)
	}
}

func TestRuntimeMetricsRequiresAddress(t *testing.T) {
	if _, err := StartMetrics(config.Metrics{Enabled: true}, "dipole-test", nil); err == nil {
		t.Fatal("enabled metrics must require an address")
	}
}

func TestRuntimeMetricsStartsOnConfiguredAddress(t *testing.T) {
	server, err := StartMetrics(config.Metrics{Enabled: true, Address: "127.0.0.1:0"}, "dipole-test", nil)
	if err != nil {
		t.Fatalf("start runtime metrics: %v", err)
	}
	if server.Address() == "" {
		t.Fatal("metrics server address is empty")
	}
	if err := server.Close(context.Background()); err != nil {
		t.Fatalf("close runtime metrics: %v", err)
	}
}

func TestRuntimeMetricsRequiresServiceName(t *testing.T) {
	if _, err := StartMetrics(config.Metrics{Enabled: true, Address: "127.0.0.1:0"}, "", nil); err == nil {
		t.Fatal("enabled metrics must require a service name")
	}
}

func TestRuntimeMetricsSkipsTypedNilOptionalCollector(t *testing.T) {
	typedNilCollectorCalls.Store(0)
	var collector *typedNilCollector
	server, err := StartMetrics(config.Metrics{Enabled: true, Address: "127.0.0.1:0"}, "dipole-test", nil, collector)
	if err != nil {
		t.Fatalf("start runtime metrics: %v", err)
	}
	t.Cleanup(func() { _ = server.Close(context.Background()) })
	if calls := typedNilCollectorCalls.Load(); calls != 0 {
		t.Fatalf("typed nil collector was invoked %d times", calls)
	}
}
