package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/JekYUlll/Dipole/internal/bootstrap"
	"github.com/JekYUlll/Dipole/internal/config"
)

func main() {
	jobName := flag.String("job", "message-search-v1", "durable Search backfill job name")
	targetIndex := flag.String("target-index", "", "explicit Elasticsearch physical build index")
	ownerID := flag.String("owner", defaultOwnerID(), "lease owner identity")
	batchSize := flag.Int("batch-size", 500, "final message states applied before checkpoint advance")
	leaseSeconds := flag.Int("lease-seconds", 60, "owner lease duration renewed after each batch")
	source := flag.String("source", bootstrap.SearchSourceMySQL, "snapshot source: mysql or archive")
	archiveManifest := flag.String("archive-manifest", "", "verified archive manifest when source=archive")
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
	result, err := bootstrap.RunSearchBackfill(ctx, bootstrap.SearchBackfillOptions{
		JobName: *jobName, OwnerID: *ownerID, TargetIndex: *targetIndex,
		BatchSize: *batchSize, LeaseDuration: time.Duration(*leaseSeconds) * time.Second,
		Source: *source, ArchiveManifest: *archiveManifest,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Search backfill complete: target=%s high_watermark=%d last_processed=%d processed=%d\n",
		*targetIndex, result.HighWatermarkID, result.LastProcessedID, result.Processed)
}

func defaultOwnerID() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "unknown-host"
	}
	return fmt.Sprintf("%s:%d", hostname, os.Getpid())
}
