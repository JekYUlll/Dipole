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
	syncops "github.com/JekYUlll/Dipole/internal/operations/sync"
)

func main() {
	jobName := flag.String("job", "sync-inbox-v1", "durable Sync replay job name")
	ownerID := flag.String("owner", defaultOwnerID(), "lease owner identity")
	batchSize := flag.Int("batch-size", 500, "created events replayed before checkpoint advance")
	leaseSeconds := flag.Int("lease-seconds", 60, "owner lease duration renewed after each batch")
	flag.Parse()
	if err := config.Load(); err != nil {
		fmt.Fprintln(os.Stderr, fmt.Errorf("load config: %w", err))
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	result, err := syncops.RunSyncReplay(ctx, syncops.SyncReplayOptions{
		JobName: *jobName, OwnerID: *ownerID, BatchSize: *batchSize,
		LeaseDuration: time.Duration(*leaseSeconds) * time.Second,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Sync replay complete: high_watermark=%d last_processed=%d processed=%d projected=%d skipped=%d\n",
		result.HighWatermarkID, result.LastProcessedID, result.Processed, result.Projected, result.Skipped)
}

func defaultOwnerID() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "unknown-host"
	}
	return fmt.Sprintf("%s:%d", hostname, os.Getpid())
}
