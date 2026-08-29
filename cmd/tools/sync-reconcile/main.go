package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/JekYUlll/Dipole/internal/bootstrap"
	"github.com/JekYUlll/Dipole/internal/config"
)

func main() {
	jobName := flag.String("job", "sync-inbox-v1", "completed Sync replay job defining the source snapshot")
	batchSize := flag.Int("batch-size", 500, "created events compared per page")
	maxExamples := flag.Int("max-examples", 100, "maximum mismatch examples included in the report")
	flag.Parse()
	if err := config.Load(); err != nil {
		fmt.Fprintln(os.Stderr, fmt.Errorf("load config: %w", err))
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	report, err := bootstrap.RunSyncReconciliation(ctx, bootstrap.SyncReconciliationOptions{
		JobName: *jobName, BatchSize: *batchSize, MaxExamples: *maxExamples,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fmt.Fprintln(os.Stderr, fmt.Errorf("encode Sync reconciliation report: %w", err))
		os.Exit(1)
	}
	if !report.Consistent {
		os.Exit(2)
	}
}
