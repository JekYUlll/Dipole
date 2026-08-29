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
	agenttimelinereconcile "github.com/JekYUlll/Dipole/internal/operations/agent/reconcile"
	platformmysql "github.com/JekYUlll/Dipole/internal/platform/mysql"
	platformobservability "github.com/JekYUlll/Dipole/internal/platform/observability"
	agentmysql "github.com/JekYUlll/Dipole/internal/services/agent/infrastructure/mysql"
	"github.com/prometheus/client_golang/prometheus"
)

func main() {
	batchSize := flag.Int("batch-size", 100, "maximum repair intents per poll")
	lease := flag.Duration("lease", 30*time.Second, "claim lease for a repair batch")
	backoff := flag.Duration("retry-backoff", time.Second, "delay before a failed repair is retried")
	interval := flag.Duration("interval", time.Second, "poll interval")
	metricsAddress := flag.String("metrics-address", "", "optional Prometheus metrics listen address")
	once := flag.Bool("once", false, "process one repair batch and exit")
	flag.Parse()

	if err := config.Load(); err != nil {
		fatal(fmt.Errorf("load config: %w", err))
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := platformmysql.InitMySQL(); err != nil {
		fatal(fmt.Errorf("initialize MySQL: %w", err))
	}
	defer platformmysql.SQLDB.Close()

	mysqlStore, err := mysql.NewStore(platformmysql.SQLDB)
	if err != nil {
		fatal(err)
	}
	policy, err := agentmysql.NewAgentPolicyRepositoryWithTransactions(mysqlStore)
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
	if *once {
		report, err := repairer.RunOnce(ctx, time.Now().UTC())
		if err != nil {
			fatal(err)
		}
		fmt.Printf("agent timeline repair once: claimed=%d repaired=%d retried=%d\n", report.Claimed, report.Repaired, report.Retried)
		return
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
