package observability

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestDependencyReadinessUsesFailureAndRecoveryHysteresis(t *testing.T) {
	server := newDependencyTestServer(t)
	var healthy atomic.Bool
	healthy.Store(true)
	probe := DependencyProbe{Name: "mysql", Check: func(context.Context) error {
		if healthy.Load() {
			return nil
		}
		return errors.New("mysql unavailable")
	}}
	if err := server.MonitorDependencies([]DependencyProbe{probe}, DependencyReadinessPolicy{
		Interval: time.Hour, Timeout: time.Second, FailureThreshold: 2, SuccessThreshold: 2,
	}); err != nil {
		t.Fatalf("monitor dependencies: %v", err)
	}
	server.MarkReady()
	assertProbe(t, server, "/readyz", http.StatusOK, "ready")

	healthy.Store(false)
	server.RefreshDependencyReadiness(t.Context())
	assertProbe(t, server, "/readyz", http.StatusOK, "ready")
	server.RefreshDependencyReadiness(t.Context())
	assertProbe(t, server, "/readyz", http.StatusServiceUnavailable, "not ready")
	assertProbe(t, server, "/metrics", http.StatusOK, `dipole_dependency_ready{dependency="mysql",service="dipole-test"} 0`)
	assertProbe(t, server, "/metrics", http.StatusOK, `dipole_dependency_probe_failures_total{dependency="mysql",service="dipole-test"} 2`)

	healthy.Store(true)
	server.RefreshDependencyReadiness(t.Context())
	assertProbe(t, server, "/readyz", http.StatusServiceUnavailable, "not ready")
	server.RefreshDependencyReadiness(t.Context())
	assertProbe(t, server, "/readyz", http.StatusOK, "ready")
}

func TestDependencyReadinessCachesTimeoutAndCannotOverrideDrain(t *testing.T) {
	server := newDependencyTestServer(t)
	blocking := atomic.Bool{}
	probe := DependencyProbe{Name: "core-rpc", Check: func(ctx context.Context) error {
		if !blocking.Load() {
			return nil
		}
		<-ctx.Done()
		return ctx.Err()
	}}
	if err := server.MonitorDependencies([]DependencyProbe{probe}, DependencyReadinessPolicy{
		Interval: time.Hour, Timeout: 10 * time.Millisecond, FailureThreshold: 1, SuccessThreshold: 1,
	}); err != nil {
		t.Fatalf("monitor dependencies: %v", err)
	}
	server.MarkReady()
	blocking.Store(true)
	server.RefreshDependencyReadiness(t.Context())

	started := time.Now()
	assertProbe(t, server, "/readyz", http.StatusServiceUnavailable, "not ready")
	if elapsed := time.Since(started); elapsed > 50*time.Millisecond {
		t.Fatalf("cached readiness took %s", elapsed)
	}

	blocking.Store(false)
	server.RefreshDependencyReadiness(t.Context())
	assertProbe(t, server, "/readyz", http.StatusOK, "ready")
	server.MarkNotReady()
	server.RefreshDependencyReadiness(t.Context())
	assertProbe(t, server, "/readyz", http.StatusServiceUnavailable, "not ready")
}

func TestDependencyReadinessCanRequireInitialSuccess(t *testing.T) {
	server := newDependencyTestServer(t)
	var healthy atomic.Bool
	probe := DependencyProbe{
		Name:                  "kafka-assignment",
		RequireInitialSuccess: true,
		Check: func(context.Context) error {
			if healthy.Load() {
				return nil
			}
			return errors.New("consumer group has no assignment")
		},
	}
	if err := server.MonitorDependencies([]DependencyProbe{probe}, DependencyReadinessPolicy{
		Interval: time.Hour, Timeout: time.Second, FailureThreshold: 3, SuccessThreshold: 2,
	}); err != nil {
		t.Fatalf("monitor dependencies: %v", err)
	}
	server.MarkReady()
	assertProbe(t, server, "/readyz", http.StatusServiceUnavailable, "not ready")

	healthy.Store(true)
	server.RefreshDependencyReadiness(t.Context())
	assertProbe(t, server, "/readyz", http.StatusServiceUnavailable, "not ready")
	server.RefreshDependencyReadiness(t.Context())
	assertProbe(t, server, "/readyz", http.StatusOK, "ready")
}

func TestDependencyReadinessRejectsInvalidPolicyAndProbeSet(t *testing.T) {
	server := newDependencyTestServer(t)
	check := func(context.Context) error { return nil }
	if err := server.MonitorDependencies([]DependencyProbe{{Name: "mysql", Check: check}, {Name: "mysql", Check: check}}, DependencyReadinessPolicy{
		Interval: time.Second, Timeout: time.Second, FailureThreshold: 1, SuccessThreshold: 1,
	}); err == nil {
		t.Fatal("duplicate dependency probe must fail")
	}
	if err := server.MonitorDependencies([]DependencyProbe{{Name: "BAD NAME", Check: check}}, DependencyReadinessPolicy{
		Interval: time.Second, Timeout: time.Second, FailureThreshold: 1, SuccessThreshold: 1,
	}); err == nil {
		t.Fatal("invalid dependency name must fail")
	}
	if err := server.MonitorDependencies([]DependencyProbe{{Name: "mysql", Check: nil}}, DependencyReadinessPolicy{}); err == nil {
		t.Fatal("invalid policy and nil check must fail")
	}
}

func TestReadinessChangeCallbackTracksLifecycleAndDependencies(t *testing.T) {
	server := newDependencyTestServer(t)
	var healthy atomic.Bool
	healthy.Store(true)
	if err := server.MonitorDependencies([]DependencyProbe{{Name: "mysql", Check: func(context.Context) error {
		if healthy.Load() {
			return nil
		}
		return errors.New("mysql unavailable")
	}}}, DependencyReadinessPolicy{
		Interval: time.Hour, Timeout: time.Second, FailureThreshold: 1, SuccessThreshold: 1,
	}); err != nil {
		t.Fatalf("monitor dependencies: %v", err)
	}

	var mu sync.Mutex
	states := make([]bool, 0, 4)
	server.OnReadinessChange(func(ready bool) {
		mu.Lock()
		defer mu.Unlock()
		states = append(states, ready)
	})
	server.MarkReady()
	healthy.Store(false)
	server.RefreshDependencyReadiness(t.Context())
	healthy.Store(true)
	server.RefreshDependencyReadiness(t.Context())
	server.MarkNotReady()

	mu.Lock()
	defer mu.Unlock()
	if expected := []bool{false, true, false, true, false}; !reflect.DeepEqual(states, expected) {
		t.Fatalf("readiness callback states = %v, want %v", states, expected)
	}
}

func newDependencyTestServer(t *testing.T) *MetricsServer {
	t.Helper()
	server, err := StartServiceMetricsServer("127.0.0.1:0", "dipole-test", prometheus.NewRegistry())
	if err != nil {
		t.Fatalf("start metrics server: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := server.Close(ctx); err != nil {
			t.Fatalf("close metrics server: %v", err)
		}
	})
	return server
}
