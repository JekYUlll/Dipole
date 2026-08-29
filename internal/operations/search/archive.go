package searchops

import (
	"context"
	"fmt"
	"strings"
	"time"

	searchbackfill "github.com/JekYUlll/Dipole/internal/backfill/search"
	"github.com/JekYUlll/Dipole/internal/config"
	mysqldata "github.com/JekYUlll/Dipole/internal/data/mysql"
	platformstorage "github.com/JekYUlll/Dipole/internal/platform/storage"
)

type SearchArchiveOptions struct {
	ManifestPath string
	SnapshotID   string
	BatchSize    int
}

func PublishSearchArchive(ctx context.Context, manifestPath, receiptPath, prefix string, retentionDays int) (searchbackfill.ArchiveReceipt, error) {
	store, configuredDays, err := openSearchArchiveObjectStore()
	if err != nil {
		return searchbackfill.ArchiveReceipt{}, err
	}
	if retentionDays == 0 {
		retentionDays = configuredDays
	}
	receipt, err := searchbackfill.PublishArchive(ctx, store, manifestPath, prefix, time.Now().UTC(), time.Duration(retentionDays)*24*time.Hour)
	if err != nil {
		return searchbackfill.ArchiveReceipt{}, err
	}
	if err := searchbackfill.WriteArchiveReceipt(receiptPath, receipt); err != nil {
		return searchbackfill.ArchiveReceipt{}, fmt.Errorf("write Search archive receipt: %w", err)
	}
	return receipt, nil
}

func RestoreSearchArchive(ctx context.Context, receiptPath, destination string) (string, error) {
	store, _, err := openSearchArchiveObjectStore()
	if err != nil {
		return "", err
	}
	receipt, err := searchbackfill.ReadArchiveReceipt(receiptPath)
	if err != nil {
		return "", fmt.Errorf("read Search archive receipt: %w", err)
	}
	return searchbackfill.RestoreArchive(ctx, store, receipt, destination)
}

func openSearchArchiveObjectStore() (*platformstorage.SearchArchiveStore, int, error) {
	cfg := config.StorageConfig()
	if !cfg.Enabled || strings.ToLower(strings.TrimSpace(cfg.Provider)) != "minio" {
		return nil, 0, fmt.Errorf("Search archive object storage requires enabled MinIO storage")
	}
	if cfg.SearchArchiveRetentionDays <= 0 {
		return nil, 0, fmt.Errorf("Search archive retention days must be positive")
	}
	store, err := platformstorage.NewSearchArchiveStore(platformstorage.SearchArchiveConfig{
		Endpoint: cfg.Endpoint, AccessKey: cfg.AccessKey, SecretKey: cfg.SecretKey, UseSSL: cfg.UseSSL,
		Bucket: cfg.SearchArchiveBucket, MinimumRetention: time.Duration(cfg.SearchArchiveRetentionDays) * 24 * time.Hour,
	})
	return store, cfg.SearchArchiveRetentionDays, err
}

func RunSearchArchive(ctx context.Context, options SearchArchiveOptions) (searchbackfill.ArchiveManifest, error) {
	db, err := openSearchMaintenanceMySQL(ctx, "archive")
	if err != nil {
		return searchbackfill.ArchiveManifest{}, err
	}
	defer db.Close()
	store, err := mysqldata.NewStore(db)
	if err != nil {
		return searchbackfill.ArchiveManifest{}, err
	}
	source, err := mysqldata.NewSearchBackfillSource(store)
	if err != nil {
		return searchbackfill.ArchiveManifest{}, err
	}
	return searchbackfill.CreateArchive(ctx, source, options.ManifestPath, options.SnapshotID, options.BatchSize)
}
