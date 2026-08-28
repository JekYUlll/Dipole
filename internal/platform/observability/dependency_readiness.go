package observability

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type DependencyProbe struct {
	Name                  string
	RequireInitialSuccess bool
	Check                 func(context.Context) error
}

type DependencyReadinessPolicy struct {
	Interval         time.Duration
	Timeout          time.Duration
	FailureThreshold int
	SuccessThreshold int
}

type dependencyProbeState struct {
	probe               DependencyProbe
	healthy             bool
	consecutiveFailures int
	consecutiveSuccess  int
}

type dependencyReadinessMonitor struct {
	server   *MetricsServer
	policy   DependencyReadinessPolicy
	states   []dependencyProbeState
	ready    *prometheus.GaugeVec
	failures *prometheus.CounterVec

	refreshMu sync.Mutex
	stopOnce  sync.Once
	stop      chan struct{}
	done      chan struct{}
}

var dependencyNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

func validateDependencyReadiness(probes []DependencyProbe, policy DependencyReadinessPolicy) error {
	if policy.Interval <= 0 || policy.Timeout <= 0 || policy.FailureThreshold < 1 || policy.SuccessThreshold < 1 {
		return fmt.Errorf("dependency readiness policy requires positive interval, timeout, and thresholds")
	}
	seen := make(map[string]struct{}, len(probes))
	for index := range probes {
		name := strings.TrimSpace(probes[index].Name)
		if !dependencyNamePattern.MatchString(name) {
			return fmt.Errorf("invalid dependency probe name %q", probes[index].Name)
		}
		if probes[index].Check == nil {
			return fmt.Errorf("dependency probe %s check is required", name)
		}
		if _, exists := seen[name]; exists {
			return fmt.Errorf("dependency probe %s is duplicated", name)
		}
		seen[name] = struct{}{}
	}
	return nil
}

func newDependencyReadinessMonitor(server *MetricsServer, probes []DependencyProbe, policy DependencyReadinessPolicy) *dependencyReadinessMonitor {
	states := make([]dependencyProbeState, 0, len(probes))
	for _, probe := range probes {
		probe.Name = strings.TrimSpace(probe.Name)
		states = append(states, dependencyProbeState{probe: probe, healthy: !probe.RequireInitialSuccess})
	}
	return &dependencyReadinessMonitor{
		server: server, policy: policy, states: states,
		ready: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "dipole_dependency_ready", Help: "Whether a critical runtime dependency is currently healthy after hysteresis.",
			ConstLabels: prometheus.Labels{"service": server.service},
		}, []string{"dependency"}),
		failures: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "dipole_dependency_probe_failures_total", Help: "Total failed critical dependency probes.",
			ConstLabels: prometheus.Labels{"service": server.service},
		}, []string{"dependency"}),
		stop: make(chan struct{}), done: make(chan struct{}),
	}
}

func (m *dependencyReadinessMonitor) start() {
	for index := range m.states {
		ready := 0.0
		if m.states[index].healthy {
			ready = 1
		}
		m.ready.WithLabelValues(m.states[index].probe.Name).Set(ready)
		m.failures.WithLabelValues(m.states[index].probe.Name)
	}
	m.refresh(context.Background())
	go func() {
		defer close(m.done)
		ticker := time.NewTicker(m.policy.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.refresh(context.Background())
			case <-m.stop:
				return
			}
		}
	}()
}

func (m *dependencyReadinessMonitor) refresh(ctx context.Context) {
	m.refreshMu.Lock()
	defer m.refreshMu.Unlock()
	allHealthy := true
	for index := range m.states {
		state := &m.states[index]
		probeCtx, cancel := context.WithTimeout(ctx, m.policy.Timeout)
		err := state.probe.Check(probeCtx)
		cancel()
		if err != nil {
			state.consecutiveFailures++
			state.consecutiveSuccess = 0
			m.failures.WithLabelValues(state.probe.Name).Inc()
			if state.consecutiveFailures >= m.policy.FailureThreshold {
				state.healthy = false
			}
		} else {
			state.consecutiveSuccess++
			state.consecutiveFailures = 0
			if state.consecutiveSuccess >= m.policy.SuccessThreshold {
				state.healthy = true
			}
		}
		if state.healthy {
			m.ready.WithLabelValues(state.probe.Name).Set(1)
		} else {
			m.ready.WithLabelValues(state.probe.Name).Set(0)
			allHealthy = false
		}
	}
	m.server.setDependenciesReady(allHealthy)
}

func (m *dependencyReadinessMonitor) close() {
	m.stopOnce.Do(func() { close(m.stop) })
	<-m.done
}
