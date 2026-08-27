package observability

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestServiceMetricsServerExposesLifecycleAndRegistry(t *testing.T) {
	registry := prometheus.NewRegistry()
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{Name: "dipole_test_ready"})
	gauge.Set(1)
	registry.MustRegister(gauge)

	server, err := StartServiceMetricsServer("127.0.0.1:0", "dipole-message", registry)
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

	assertProbe(t, server, "/livez", http.StatusOK, "alive")
	assertProbe(t, server, "/readyz", http.StatusServiceUnavailable, "not ready")
	assertProbe(t, server, "/health", http.StatusServiceUnavailable, "not ready")
	assertProbe(t, server, "/metrics", http.StatusOK, `dipole_service_info{service="dipole-message"} 1`)
	assertProbe(t, server, "/metrics", http.StatusOK, `dipole_service_ready{service="dipole-message"} 0`)
	assertProbe(t, server, "/metrics", http.StatusOK, "dipole_test_ready 1")

	server.MarkReady()
	assertProbe(t, server, "/readyz", http.StatusOK, "ready")
	assertProbe(t, server, "/health", http.StatusOK, "ready")
	assertProbe(t, server, "/metrics", http.StatusOK, `dipole_service_ready{service="dipole-message"} 1`)

	server.MarkNotReady()
	assertProbe(t, server, "/readyz", http.StatusServiceUnavailable, "not ready")
}

func assertProbe(t *testing.T, server *MetricsServer, path string, wantStatus int, wantBody string) {
	t.Helper()
	response, err := http.Get("http://" + server.Address() + path)
	if err != nil {
		t.Fatalf("get %s: %v", path, err)
	}
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil {
		t.Fatalf("read %s: %v", path, readErr)
	}
	if response.StatusCode != wantStatus || !strings.Contains(string(body), wantBody) {
		t.Fatalf("%s status=%d body=%q, want status=%d body containing %q", path, response.StatusCode, body, wantStatus, wantBody)
	}
}

func TestMetricsServerRejectsInvalidAddress(t *testing.T) {
	if _, err := StartMetricsServer("missing-port", prometheus.NewRegistry()); err == nil {
		t.Fatal("invalid metrics listener must be rejected")
	}
	if _, err := StartServiceMetricsServer("127.0.0.1:0", "", prometheus.NewRegistry()); err == nil {
		t.Fatal("service metrics listener must require a service name")
	}
}
