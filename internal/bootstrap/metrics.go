package bootstrap

import (
	"github.com/JekYUlll/Dipole/internal/config"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	platformObservability "github.com/JekYUlll/Dipole/internal/platform/observability"
	platformRuntime "github.com/JekYUlll/Dipole/internal/platform/runtime"
	"github.com/prometheus/client_golang/prometheus"
)

func startRuntimeMetrics(cfg config.Metrics, serviceName string, consumer *platformKafka.Consumer, collectors ...prometheus.Collector) (*platformObservability.MetricsServer, error) {
	return platformRuntime.StartMetrics(cfg, serviceName, consumer, collectors...)
}

func markRuntimeReady(server *platformObservability.MetricsServer) {
	platformRuntime.MarkReady(server)
}

func closeRuntimeMetrics(server *platformObservability.MetricsServer) error {
	return platformRuntime.CloseMetrics(server)
}
