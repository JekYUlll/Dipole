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

func TestMetricsServerServesHealthAndRegistry(t *testing.T) {
	registry := prometheus.NewRegistry()
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{Name: "dipole_test_ready"})
	gauge.Set(1)
	registry.MustRegister(gauge)

	server, err := StartMetricsServer("127.0.0.1:0", registry)
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

	for path, expected := range map[string]string{
		"/health":  "ok",
		"/metrics": "dipole_test_ready 1",
	} {
		response, err := http.Get("http://" + server.Address() + path)
		if err != nil {
			t.Fatalf("get %s: %v", path, err)
		}
		body, readErr := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		if response.StatusCode != http.StatusOK || !strings.Contains(string(body), expected) {
			t.Fatalf("%s status=%d body=%q", path, response.StatusCode, body)
		}
	}
}

func TestMetricsServerRejectsInvalidAddress(t *testing.T) {
	if _, err := StartMetricsServer("missing-port", prometheus.NewRegistry()); err == nil {
		t.Fatal("invalid metrics listener must be rejected")
	}
}
