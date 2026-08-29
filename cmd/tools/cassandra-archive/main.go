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
	cassandraops "github.com/JekYUlll/Dipole/internal/operations/cassandra"
)

func main() {
	action := flag.String("action", "create", "archive operation: create, publish, or restore")
	manifestPath := flag.String("manifest", "", "immutable message archive manifest path")
	snapshotID := flag.String("snapshot-id", "", "operator-defined immutable snapshot identity")
	batchSize := flag.Int("batch-size", 500, "complete messages streamed per source page")
	receiptPath := flag.String("receipt", "", "object archive version receipt path")
	destination := flag.String("destination", "", "restore destination directory")
	objectPrefix := flag.String("object-prefix", "cassandra", "object key prefix for published archives")
	retentionDays := flag.Int("retention-days", 0, "object retention days; zero uses configured minimum")
	flag.Parse()
	if err := config.Load(); err != nil {
		fmt.Fprintln(os.Stderr, fmt.Errorf("load config: %w", err))
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	var result any
	var err error
	switch strings.ToLower(strings.TrimSpace(*action)) {
	case "create":
		if strings.TrimSpace(*manifestPath) == "" || strings.TrimSpace(*snapshotID) == "" {
			err = fmt.Errorf("create requires -manifest and -snapshot-id")
			break
		}
		result, err = cassandraops.RunCassandraArchive(ctx, cassandraops.CassandraArchiveOptions{ManifestPath: *manifestPath, SnapshotID: *snapshotID, BatchSize: *batchSize})
	case "publish":
		if strings.TrimSpace(*manifestPath) == "" || strings.TrimSpace(*receiptPath) == "" {
			err = fmt.Errorf("publish requires -manifest and -receipt")
			break
		}
		result, err = cassandraops.PublishCassandraArchive(ctx, *manifestPath, *receiptPath, *objectPrefix, *retentionDays)
	case "restore":
		if strings.TrimSpace(*receiptPath) == "" || strings.TrimSpace(*destination) == "" {
			err = fmt.Errorf("restore requires -receipt and -destination")
			break
		}
		var restored string
		restored, err = cassandraops.RestoreCassandraArchive(ctx, *receiptPath, *destination)
		result = map[string]string{"manifest": restored}
	default:
		err = fmt.Errorf("unsupported archive action: %s", *action)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, fmt.Errorf("encode Cassandra message archive result: %w", err))
		os.Exit(1)
	}
}
