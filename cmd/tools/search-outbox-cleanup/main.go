package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/JekYUlll/Dipole/internal/bootstrap"
	"github.com/JekYUlll/Dipole/internal/config"
)

func main() {
	receipt := flag.String("receipt", "", "verified object archive receipt")
	reconcileReport := flag.String("reconcile-report", "", "consistent Search reconciliation JSON report")
	targetIndex := flag.String("target-index", "", "physical index bound to the completed Backfill Job")
	batchSize := flag.Int("batch-size", 500, "published Outbox rows deleted per batch")
	execute := flag.Bool("execute", false, "delete eligible rows; omitted means dry-run")
	confirmed := flag.Bool("confirm-maintenance-window", false, "confirm Message mutation producers are paused")
	operator := flag.String("operator", "", "responsible operator identity; required with --execute")
	flag.Parse()
	if strings.TrimSpace(*receipt) == "" || strings.TrimSpace(*reconcileReport) == "" || strings.TrimSpace(*targetIndex) == "" {
		fmt.Fprintln(os.Stderr, "-receipt, -reconcile-report, and -target-index are required")
		os.Exit(1)
	}
	if err := config.Load(); err != nil {
		fmt.Fprintln(os.Stderr, fmt.Errorf("load config: %w", err))
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	result, err := bootstrap.RunSearchOutboxCleanup(ctx, bootstrap.SearchCleanupOptions{
		ReceiptPath: *receipt, ReconcileReportPath: *reconcileReport, TargetIndex: *targetIndex,
		BatchSize: *batchSize, Execute: *execute, MaintenanceConfirmed: *confirmed, Operator: *operator,
	})
	if result.JobName != "" {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if encodeErr := encoder.Encode(result); encodeErr != nil {
			fmt.Fprintln(os.Stderr, encodeErr)
			os.Exit(1)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
