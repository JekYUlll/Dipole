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
	manifestPath := flag.String("manifest", "", "immutable Search archive manifest output path")
	snapshotID := flag.String("snapshot-id", "", "operator-defined immutable snapshot identity")
	batchSize := flag.Int("batch-size", 500, "final mutation states streamed per source page")
	flag.Parse()
	if strings.TrimSpace(*manifestPath) == "" || strings.TrimSpace(*snapshotID) == "" {
		fmt.Fprintln(os.Stderr, "-manifest and -snapshot-id are required")
		os.Exit(1)
	}
	if err := config.Load(); err != nil {
		fmt.Fprintln(os.Stderr, fmt.Errorf("load config: %w", err))
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	manifest, err := bootstrap.RunSearchArchive(ctx, bootstrap.SearchArchiveOptions{
		ManifestPath: *manifestPath, SnapshotID: *snapshotID, BatchSize: *batchSize,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(manifest); err != nil {
		fmt.Fprintln(os.Stderr, fmt.Errorf("encode Search archive manifest: %w", err))
		os.Exit(1)
	}
}
