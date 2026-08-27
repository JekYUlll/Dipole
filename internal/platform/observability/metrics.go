package observability

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MetricsServer struct {
	server    *http.Server
	listener  net.Listener
	service   string
	ready     atomic.Bool
	readiness prometheus.Gauge
}

var serviceNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

func StartMetricsServer(address string, gatherer prometheus.Gatherer) (*MetricsServer, error) {
	server, err := StartServiceMetricsServer(address, "dipole", gatherer)
	if err != nil {
		return nil, err
	}
	server.MarkReady()
	return server, nil
}

func StartServiceMetricsServer(address, service string, gatherer prometheus.Gatherer) (*MetricsServer, error) {
	service = strings.TrimSpace(service)
	if !serviceNamePattern.MatchString(service) {
		return nil, fmt.Errorf("invalid metrics service name %q", service)
	}
	if _, _, err := net.SplitHostPort(address); err != nil {
		return nil, fmt.Errorf("invalid metrics address %q: %w", address, err)
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("listen for metrics on %q: %w", address, err)
	}
	info := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "dipole_service_info", Help: "Static identity of a running Dipole service.",
		ConstLabels: prometheus.Labels{"service": service},
	})
	info.Set(1)
	readiness := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "dipole_service_ready", Help: "Whether the Dipole service currently accepts traffic.",
		ConstLabels: prometheus.Labels{"service": service},
	})
	lifecycleRegistry := prometheus.NewRegistry()
	lifecycleRegistry.MustRegister(info, readiness)
	combinedGatherer := prometheus.Gatherers{gatherer, lifecycleRegistry}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(combinedGatherer, promhttp.HandlerOpts{}))
	mux.HandleFunc("/livez", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("alive\n"))
	})
	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	result := &MetricsServer{server: server, listener: listener, service: service, readiness: readiness}
	readinessHandler := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if !result.ready.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("not ready\n"))
			return
		}
		_, _ = w.Write([]byte("ready\n"))
	}
	mux.HandleFunc("/readyz", readinessHandler)
	mux.HandleFunc("/health", readinessHandler)
	go func() { _ = server.Serve(listener) }()
	return result, nil
}

func (s *MetricsServer) Service() string {
	if s == nil {
		return ""
	}
	return s.service
}

func (s *MetricsServer) MarkReady() {
	if s == nil {
		return
	}
	s.ready.Store(true)
	s.readiness.Set(1)
}

func (s *MetricsServer) MarkNotReady() {
	if s == nil {
		return
	}
	s.ready.Store(false)
	s.readiness.Set(0)
}

func (s *MetricsServer) Address() string {
	if s == nil || s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

func (s *MetricsServer) Close(ctx context.Context) error {
	if s == nil || s.server == nil {
		return nil
	}
	s.MarkNotReady()
	return s.server.Shutdown(ctx)
}
