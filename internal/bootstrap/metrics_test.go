package bootstrap

import (
	"context"
	"testing"

	"github.com/JekYUlll/Dipole/internal/config"
)

func TestRuntimeMetricsDisabledDoesNotListen(t *testing.T) {
	server, err := startRuntimeMetrics(config.Metrics{Enabled: false}, "dipole-test", nil)
	if err != nil || server != nil {
		t.Fatalf("disabled metrics server = %v, err = %v", server, err)
	}
}

func TestRuntimeMetricsRequiresAddress(t *testing.T) {
	if _, err := startRuntimeMetrics(config.Metrics{Enabled: true}, "dipole-test", nil); err == nil {
		t.Fatal("enabled metrics must require an address")
	}
}

func TestRuntimeMetricsStartsOnConfiguredAddress(t *testing.T) {
	server, err := startRuntimeMetrics(config.Metrics{Enabled: true, Address: "127.0.0.1:0"}, "dipole-test", nil)
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
	if _, err := startRuntimeMetrics(config.Metrics{Enabled: true, Address: "127.0.0.1:0"}, "", nil); err == nil {
		t.Fatal("enabled metrics must require a service name")
	}
}
