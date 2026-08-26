package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/JekYUlll/Dipole/internal/config"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	platformObservability "github.com/JekYUlll/Dipole/internal/platform/observability"
	"github.com/prometheus/client_golang/prometheus"
)

func startRuntimeMetrics(cfg config.Metrics, consumer *platformKafka.Consumer, collectors ...prometheus.Collector) (*platformObservability.MetricsServer, error) {
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
	return platformObservability.StartMetricsServer(cfg.Address, registry)
}

func closeRuntimeMetrics(server *platformObservability.MetricsServer) error {
	if server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return server.Close(ctx)
}
