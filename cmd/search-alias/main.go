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
	"time"

	"github.com/JekYUlll/Dipole/internal/bootstrap"
	"github.com/JekYUlll/Dipole/internal/config"
	searchcutover "github.com/JekYUlll/Dipole/internal/cutover/search"
)

func main() {
	action := flag.String("action", "switch", "alias operation: switch or rollback")
	jobName := flag.String("job", "", "completed Search backfill job bound to the target index")
	fromIndex := flag.String("from-index", "", "physical index currently owning the production aliases")
	toIndex := flag.String("to-index", "", "verified physical index that will receive the aliases")
	batchSize := flag.Int("batch-size", 500, "final mutation states read per reconciliation page")
	maxExamples := flag.Int("max-examples", 100, "maximum reconciliation mismatch examples")
	rollbackHours := flag.Int("rollback-window-hours", 24, "old-index retention window recorded in the receipt")
	confirmed := flag.Bool("confirm-maintenance-window", false, "confirm Message mutation producers are paused for the operation")
	source := flag.String("source", bootstrap.SearchSourceMySQL, "snapshot source: mysql or archive")
	archiveManifest := flag.String("archive-manifest", "", "verified archive manifest when source=archive")
	flag.Parse()

	if strings.TrimSpace(*jobName) == "" || strings.TrimSpace(*fromIndex) == "" || strings.TrimSpace(*toIndex) == "" {
		fmt.Fprintln(os.Stderr, "-job, -from-index, and -to-index are required")
		os.Exit(1)
	}
	if err := config.Load(); err != nil {
		fmt.Fprintln(os.Stderr, fmt.Errorf("load config: %w", err))
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	receipt, err := bootstrap.RunSearchAliasOperation(ctx, bootstrap.SearchAliasOptions{
		Action: searchcutover.Action(strings.TrimSpace(*action)), JobName: *jobName,
		FromIndex: *fromIndex, ToIndex: *toIndex, BatchSize: *batchSize, MaxExamples: *maxExamples,
		MaintenanceConfirmed: *confirmed, RollbackWindow: time.Duration(*rollbackHours) * time.Hour,
		Source: *source, ArchiveManifest: *archiveManifest,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(receipt); err != nil {
		fmt.Fprintln(os.Stderr, fmt.Errorf("encode Search alias receipt: %w", err))
		os.Exit(1)
	}
}
