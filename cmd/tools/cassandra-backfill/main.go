package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JekYUlll/Dipole/internal/bootstrap"
	"github.com/JekYUlll/Dipole/internal/config"
)

func main() {
	jobName := flag.String("job", "message-timeline-v1", "durable backfill job name")
	ownerID := flag.String("owner", defaultOwnerID(), "lease owner identity")
	batchSize := flag.Int("batch-size", 500, "messages copied before checkpoint advance")
	leaseSeconds := flag.Int("lease-seconds", 60, "owner lease duration renewed after each batch")
	source := flag.String("source", "mysql", "snapshot source: mysql or archive")
	archiveManifest := flag.String("archive-manifest", "", "immutable message archive manifest path")
	flag.Parse()

	if err := config.Load(); err != nil {
		fmt.Fprintln(os.Stderr, fmt.Errorf("load config: %w", err))
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	result, err := bootstrap.RunCassandraBackfill(ctx, bootstrap.CassandraBackfillOptions{
		JobName: *jobName, OwnerID: *ownerID, BatchSize: *batchSize,
		LeaseDuration: time.Duration(*leaseSeconds) * time.Second,
		Source:        *source, ArchiveManifest: *archiveManifest,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Cassandra backfill complete: high_watermark=%d last_processed=%d processed=%d inserted=%d duplicates=%d\n",
		result.HighWatermarkID, result.LastProcessedID, result.Processed, result.Inserted, result.Duplicates)
}

func defaultOwnerID() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "unknown-host"
	}
	return fmt.Sprintf("%s:%d", hostname, os.Getpid())
}
