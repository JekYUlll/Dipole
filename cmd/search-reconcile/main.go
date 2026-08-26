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
	jobName := flag.String("job", "message-search-v1", "completed Search backfill job defining the source snapshot")
	targetIndex := flag.String("target-index", "", "explicit Elasticsearch physical build index")
	batchSize := flag.Int("batch-size", 500, "final mutation states read per page")
	maxExamples := flag.Int("max-examples", 100, "maximum mismatch examples included in the report")
	flag.Parse()
	if strings.TrimSpace(*targetIndex) == "" {
		fmt.Fprintln(os.Stderr, "-target-index is required")
		os.Exit(1)
	}
	if err := config.Load(); err != nil {
		fmt.Fprintln(os.Stderr, fmt.Errorf("load config: %w", err))
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	report, err := bootstrap.RunSearchReconciliation(ctx, bootstrap.SearchReconciliationOptions{
		JobName: *jobName, TargetIndex: *targetIndex, BatchSize: *batchSize, MaxExamples: *maxExamples,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fmt.Fprintln(os.Stderr, fmt.Errorf("encode Search reconciliation report: %w", err))
		os.Exit(1)
	}
	if !report.Consistent {
		os.Exit(2)
	}
}
