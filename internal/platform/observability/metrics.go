package observability

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MetricsServer struct {
	server            *http.Server
	listener          net.Listener
	service           string
	lifecycleReady    atomic.Bool
	dependenciesReady atomic.Bool
	readiness         prometheus.Gauge
	registry          *prometheus.Registry
	monitorMu         sync.Mutex
	monitor           *dependencyReadinessMonitor
	actualReady       atomic.Bool
	callbackMu        sync.Mutex
	callbacks         []func(bool)
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
	result := &MetricsServer{server: server, listener: listener, service: service, readiness: readiness, registry: lifecycleRegistry}
	result.dependenciesReady.Store(true)
	readinessHandler := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if !result.isReady() {
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
	s.lifecycleReady.Store(true)
	s.updateReadiness()
}

func (s *MetricsServer) MarkNotReady() {
	if s == nil {
		return
	}
	s.lifecycleReady.Store(false)
	s.updateReadiness()
}

func (s *MetricsServer) MonitorDependencies(probes []DependencyProbe, policy DependencyReadinessPolicy) error {
	if s == nil || len(probes) == 0 {
		return nil
	}
	if err := validateDependencyReadiness(probes, policy); err != nil {
		return err
	}
	s.monitorMu.Lock()
	defer s.monitorMu.Unlock()
	if s.monitor != nil {
		return fmt.Errorf("dependency readiness monitor is already configured")
	}
	monitor := newDependencyReadinessMonitor(s, probes, policy)
	if err := s.registry.Register(monitor.ready); err != nil {
		return fmt.Errorf("register dependency readiness metric: %w", err)
	}
	if err := s.registry.Register(monitor.failures); err != nil {
		s.registry.Unregister(monitor.ready)
		return fmt.Errorf("register dependency failure metric: %w", err)
	}
	s.monitor = monitor
	monitor.start()
	return nil
}

func (s *MetricsServer) RefreshDependencyReadiness(ctx context.Context) {
	if s == nil {
		return
	}
	s.monitorMu.Lock()
	monitor := s.monitor
	s.monitorMu.Unlock()
	if monitor != nil {
		monitor.refresh(ctx)
	}
}

func (s *MetricsServer) setDependenciesReady(ready bool) {
	s.dependenciesReady.Store(ready)
	s.updateReadiness()
}

func (s *MetricsServer) OnReadinessChange(callback func(bool)) {
	if s == nil || callback == nil {
		return
	}
	s.callbackMu.Lock()
	s.callbacks = append(s.callbacks, callback)
	s.callbackMu.Unlock()
	callback(s.isReady())
}

func (s *MetricsServer) isReady() bool {
	return s.lifecycleReady.Load() && s.dependenciesReady.Load()
}

func (s *MetricsServer) updateReadiness() {
	ready := s.isReady()
	if ready {
		s.readiness.Set(1)
	} else {
		s.readiness.Set(0)
	}
	previous := s.actualReady.Swap(ready)
	if previous == ready {
		return
	}
	s.callbackMu.Lock()
	callbacks := append([]func(bool){}, s.callbacks...)
	s.callbackMu.Unlock()
	for _, callback := range callbacks {
		callback(ready)
	}
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
	s.monitorMu.Lock()
	monitor := s.monitor
	s.monitor = nil
	s.monitorMu.Unlock()
	if monitor != nil {
		monitor.close()
	}
	return s.server.Shutdown(ctx)
}
