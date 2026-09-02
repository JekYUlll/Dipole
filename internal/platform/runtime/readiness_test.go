package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/JekYUlll/Dipole/internal/config"
	platformobservability "github.com/JekYUlll/Dipole/internal/platform/observability"
	realtimedelivery "github.com/JekYUlll/Dipole/internal/realtime/delivery"
)

func TestConfigureDependencyReadinessHonorsConfiguration(t *testing.T) {
	if err := ConfigureDependencyReadiness(nil, config.Metrics{}); err != nil {
		t.Fatalf("disabled readiness should be a no-op: %v", err)
	}
	err := ConfigureDependencyReadiness(nil, config.Metrics{DependencyProbesEnabled: true}, platformobservability.DependencyProbe{
		Name: "mysql", Check: func(context.Context) error { return nil },
	})
	if err == nil || !strings.Contains(err.Error(), "metrics.enabled") {
		t.Fatalf("enabled readiness error = %v", err)
	}
}

type readinessControllerStub struct{ values []bool }

func (s *readinessControllerStub) SetServing(ready bool) { s.values = append(s.values, ready) }

func TestBindRPCReadinessMirrorsMetricsLifecycle(t *testing.T) {
	metrics, err := platformobservability.StartServiceMetricsServer("127.0.0.1:0", "runtime-test", nil)
	if err != nil {
		t.Fatalf("start metrics: %v", err)
	}
	t.Cleanup(func() { _ = metrics.Close(context.Background()) })
	controller := &readinessControllerStub{}

	BindRPCReadiness(metrics, controller)
	metrics.MarkReady()
	metrics.MarkNotReady()

	want := []bool{false, true, false}
	if len(controller.values) != len(want) {
		t.Fatalf("serving callbacks = %v, want %v", controller.values, want)
	}
	for i := range want {
		if controller.values[i] != want[i] {
			t.Fatalf("serving callbacks = %v, want %v", controller.values, want)
		}
	}
}

type readinessFenceStub struct{ err error }

func (s readinessFenceStub) Assert(context.Context, realtimedelivery.Authority) error { return s.err }

func TestAuthorityFenceReadinessProbeFailsClosed(t *testing.T) {
	probe := AuthorityFenceReadinessProbe("delivery-authority", nil, realtimedelivery.AuthorityGo)
	if err := probe.Check(t.Context()); err == nil {
		t.Fatal("nil fence must fail readiness")
	}
	want := errors.New("denied")
	probe = AuthorityFenceReadinessProbe("delivery-authority", readinessFenceStub{err: want}, realtimedelivery.AuthorityGo)
	if err := probe.Check(t.Context()); !errors.Is(err, want) {
		t.Fatalf("readiness error = %v", err)
	}
}

func TestKafkaConsumerReadinessProbeRequiresInitialAssignment(t *testing.T) {
	probe := KafkaConsumerReadinessProbe("kafka-assignment", nil)
	if probe.Name != "kafka-assignment" || !probe.RequireInitialSuccess {
		t.Fatalf("unexpected kafka consumer readiness probe: %+v", probe)
	}
	if err := probe.Check(t.Context()); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("nil consumer readiness error = %v", err)
	}
}
