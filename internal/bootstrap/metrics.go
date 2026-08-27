package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/JekYUlll/Dipole/internal/config"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	platformObservability "github.com/JekYUlll/Dipole/internal/platform/observability"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/JekYUlll/Dipole/internal/logger"
)

func startRuntimeMetrics(cfg config.Metrics, serviceName string, consumer *platformKafka.Consumer, collectors ...prometheus.Collector) (*platformObservability.MetricsServer, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if cfg.Address == "" {
		return nil, fmt.Errorf("metrics address is required when metrics are enabled")
	}
	registry := prometheus.NewRegistry()
	if consumer != nil {
		registry.MustRegister(platformKafka.NewConsumerCollector(consumer))
	}
	for _, collector := range collectors {
		if collector != nil {
			registry.MustRegister(collector)
		}
	}
	return platformObservability.StartServiceMetricsServer(cfg.Address, serviceName, registry)
}

func markRuntimeReady(server *platformObservability.MetricsServer) {
	if server == nil {
		return
	}
	server.MarkReady()
	logger.Info("service runtime ready",
		zap.String("service", server.Service()),
		zap.String("health_address", server.Address()),
	)
}

func closeRuntimeMetrics(server *platformObservability.MetricsServer) error {
	if server == nil {
		return nil
	}
	server.MarkNotReady()
	logger.Info("service runtime draining", zap.String("service", server.Service()))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return server.Close(ctx)
}
