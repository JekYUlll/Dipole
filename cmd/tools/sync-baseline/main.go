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

	"github.com/JekYUlll/Dipole/internal/config"
	syncops "github.com/JekYUlll/Dipole/internal/operations/sync"
)

func main() {
	action := flag.String("action", "reconcile", "baseline action: capture, reconcile, or restore")
	jobName := flag.String("job", "sync-legacy-v1", "immutable Sync historical baseline job name")
	maxExamples := flag.Int("max-examples", 100, "maximum mismatch examples included in a report")
	flag.Parse()
	if err := config.Load(); err != nil {
		fail(fmt.Errorf("load config: %w", err))
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	switch strings.ToLower(strings.TrimSpace(*action)) {
	case "capture":
		manifest, err := syncops.RunSyncBaselineCapture(ctx, *jobName)
		if err != nil {
			fail(err)
		}
		if err := encoder.Encode(manifest); err != nil {
			fail(fmt.Errorf("encode Sync baseline manifest: %w", err))
		}
	case "reconcile":
		report, err := syncops.RunSyncBaselineReconciliation(ctx, *jobName, *maxExamples)
		if err != nil {
			fail(err)
		}
		if err := encoder.Encode(report); err != nil {
			fail(fmt.Errorf("encode Sync baseline report: %w", err))
		}
		if !report.Consistent {
			os.Exit(2)
		}
	case "restore":
		report, err := syncops.RunSyncBaselineRestore(ctx, *jobName, *maxExamples)
		if encodeErr := encoder.Encode(report); encodeErr != nil {
			fail(fmt.Errorf("encode Sync baseline restore report: %w", encodeErr))
		}
		if err != nil {
			fail(err)
		}
		if !report.Consistent {
			os.Exit(2)
		}
	default:
		fail(fmt.Errorf("unsupported Sync baseline action %q", *action))
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
