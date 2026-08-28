package bootstrap

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/JekYUlll/Dipole/internal/config"
	platformobservability "github.com/JekYUlll/Dipole/internal/platform/observability"
	realtimedelivery "github.com/JekYUlll/Dipole/internal/realtime/delivery"
	"google.golang.org/grpc/health"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
)

func TestBindRPCReadinessMirrorsRuntimeReadiness(t *testing.T) {
	metrics, err := startRuntimeMetrics(config.Metrics{Enabled: true, Address: "127.0.0.1:0"}, "dipole-test", nil)
	if err != nil {
		t.Fatalf("start runtime metrics: %v", err)
	}
	t.Cleanup(func() { _ = closeRuntimeMetrics(metrics) })
	rpc := &InternalRPCServer{health: health.NewServer()}

	bindRPCReadiness(metrics, rpc)
	assertRPCHealthStatus(t, rpc, healthv1.HealthCheckResponse_NOT_SERVING)
	markRuntimeReady(metrics)
	assertRPCHealthStatus(t, rpc, healthv1.HealthCheckResponse_SERVING)
	metrics.MarkNotReady()
	assertRPCHealthStatus(t, rpc, healthv1.HealthCheckResponse_NOT_SERVING)
}

func TestConfigureRuntimeDependencyReadinessRequiresMetrics(t *testing.T) {
	err := configureRuntimeDependencyReadiness(nil, config.Metrics{DependencyProbesEnabled: true}, platformobservability.DependencyProbe{
		Name: "mysql", Check: func(context.Context) error { return nil },
	})
	if err == nil {
		t.Fatal("enabled dependency readiness must require metrics")
	}
}

func TestKafkaConsumerReadinessProbeRequiresInitialAssignment(t *testing.T) {
	probe := kafkaConsumerReadinessProbe("kafka-assignment", nil)
	if probe.Name != "kafka-assignment" || !probe.RequireInitialSuccess {
		t.Fatalf("unexpected kafka consumer readiness probe: %+v", probe)
	}
	if err := probe.Check(t.Context()); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("nil consumer readiness error = %v", err)
	}
}

type readinessFenceStub struct {
	err error
}

func (f readinessFenceStub) Assert(context.Context, realtimedelivery.Authority) error {
	return f.err
}

func TestAuthorityFenceReadinessProbeFailsClosed(t *testing.T) {
	probe := authorityFenceReadinessProbe("delivery-authority", nil, realtimedelivery.AuthorityGo)
	if err := probe.Check(t.Context()); err == nil {
		t.Fatal("nil fence must fail readiness")
	}
	want := errors.New("denied")
	probe = authorityFenceReadinessProbe("delivery-authority", readinessFenceStub{err: want}, realtimedelivery.AuthorityGo)
	if err := probe.Check(t.Context()); !errors.Is(err, want) {
		t.Fatalf("readiness error = %v", err)
	}
}

func assertRPCHealthStatus(t *testing.T, rpc *InternalRPCServer, expected healthv1.HealthCheckResponse_ServingStatus) {
	t.Helper()
	response, err := rpc.health.Check(t.Context(), &healthv1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("check rpc health: %v", err)
	}
	if response.GetStatus() != expected {
		t.Fatalf("rpc health = %s, want %s", response.GetStatus(), expected)
	}
}
