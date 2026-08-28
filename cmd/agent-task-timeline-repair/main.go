package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/data/mysql"
	"github.com/JekYUlll/Dipole/internal/data/mysql/repository"
	platformobservability "github.com/JekYUlll/Dipole/internal/platform/observability"
	agenttimelinereconcile "github.com/JekYUlll/Dipole/internal/reconcile/agenttimeline"
	"github.com/JekYUlll/Dipole/internal/store"
	"github.com/prometheus/client_golang/prometheus"
)

func main() {
	batchSize := flag.Int("batch-size", 100, "maximum repair intents per poll")
	lease := flag.Duration("lease", 30*time.Second, "claim lease for a repair batch")
	backoff := flag.Duration("retry-backoff", time.Second, "delay before a failed repair is retried")
	interval := flag.Duration("interval", time.Second, "poll interval")
	metricsAddress := flag.String("metrics-address", "", "optional Prometheus metrics listen address")
	flag.Parse()

	if err := config.Load(); err != nil {
		fatal(fmt.Errorf("load config: %w", err))
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := store.InitMySQL(); err != nil {
		fatal(fmt.Errorf("initialize MySQL: %w", err))
	}
	defer store.SQLDB.Close()

	mysqlStore, err := mysql.NewStore(store.SQLDB)
	if err != nil {
		fatal(err)
	}
	policy, err := repository.NewAgentPolicyRepositoryWithTransactions(mysqlStore)
	if err != nil {
		fatal(err)
	}
	repairer, err := agenttimelinereconcile.NewRepairer(policy, policy, *batchSize, *lease, *backoff, *interval)
	if err != nil {
		fatal(err)
	}
	if *metricsAddress != "" {
		collector := platformobservability.NewAgentTimelineRepairCollector()
		registry := prometheus.NewRegistry()
		registry.MustRegister(collector)
		metrics, metricsErr := platformobservability.StartServiceMetricsServer(*metricsAddress, "dipole-agent-timeline-repair", registry)
		if metricsErr != nil {
			fatal(fmt.Errorf("start repair metrics: %w", metricsErr))
		}
		metrics.MarkReady()
		defer func() { _ = metrics.Close(context.Background()) }()
		repairer.WithObserver(collector)
	}
	if err := repairer.Run(ctx); err != nil && !signalContextError(err) {
		fatal(err)
	}
}

func signalContextError(err error) bool {
	return err == context.Canceled || err == context.DeadlineExceeded
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
