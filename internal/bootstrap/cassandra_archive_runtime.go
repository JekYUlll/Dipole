package bootstrap

import (
	"context"
	"fmt"
	"strings"
	"time"

	cassandrabackfill "github.com/JekYUlll/Dipole/internal/backfill/cassandra"
	"github.com/JekYUlll/Dipole/internal/config"
	mysqldata "github.com/JekYUlll/Dipole/internal/data/mysql"
	platformstorage "github.com/JekYUlll/Dipole/internal/platform/storage"
)

type CassandraArchiveOptions struct {
	ManifestPath string
	SnapshotID   string
	BatchSize    int
}

func RunCassandraArchive(ctx context.Context, options CassandraArchiveOptions) (cassandrabackfill.ArchiveManifest, error) {
	db, err := openCassandraMaintenanceMySQL(ctx, "archive")
	if err != nil {
		return cassandrabackfill.ArchiveManifest{}, err
	}
	defer db.Close()
	store, err := mysqldata.NewStore(db)
	if err != nil {
		return cassandrabackfill.ArchiveManifest{}, err
	}
	source, err := mysqldata.NewCassandraBackfillSource(store)
	if err != nil {
		return cassandrabackfill.ArchiveManifest{}, err
	}
	return cassandrabackfill.CreateArchive(ctx, source, options.ManifestPath, options.SnapshotID, options.BatchSize)
}

func PublishCassandraArchive(ctx context.Context, manifestPath, receiptPath, prefix string, retentionDays int) (cassandrabackfill.ArchiveReceipt, error) {
	store, configuredDays, err := openCassandraArchiveObjectStore()
	if err != nil {
		return cassandrabackfill.ArchiveReceipt{}, err
	}
	if retentionDays == 0 {
		retentionDays = configuredDays
	}
	receipt, err := cassandrabackfill.PublishArchive(ctx, store, manifestPath, prefix, time.Now().UTC(), time.Duration(retentionDays)*24*time.Hour)
	if err != nil {
		return cassandrabackfill.ArchiveReceipt{}, err
	}
	if err := cassandrabackfill.WriteArchiveReceipt(receiptPath, receipt); err != nil {
		return cassandrabackfill.ArchiveReceipt{}, fmt.Errorf("write Cassandra message archive receipt: %w", err)
	}
	return receipt, nil
}

func RestoreCassandraArchive(ctx context.Context, receiptPath, destination string) (string, error) {
	store, _, err := openCassandraArchiveObjectStore()
	if err != nil {
		return "", err
	}
	receipt, err := cassandrabackfill.ReadArchiveReceipt(receiptPath)
	if err != nil {
		return "", fmt.Errorf("read Cassandra message archive receipt: %w", err)
	}
	return cassandrabackfill.RestoreArchive(ctx, store, receipt, destination)
}

func openCassandraArchiveObjectStore() (*platformstorage.VersionedArchiveStore, int, error) {
	cfg := config.StorageConfig()
	if !cfg.Enabled || strings.ToLower(strings.TrimSpace(cfg.Provider)) != "minio" {
		return nil, 0, fmt.Errorf("Cassandra message archive object storage requires enabled MinIO storage")
	}
	if cfg.MessageArchiveRetentionDays <= 0 {
		return nil, 0, fmt.Errorf("Cassandra message archive retention days must be positive")
	}
	store, err := platformstorage.NewVersionedArchiveStore(platformstorage.VersionedArchiveConfig{
		Endpoint: cfg.Endpoint, AccessKey: cfg.AccessKey, SecretKey: cfg.SecretKey, UseSSL: cfg.UseSSL,
		Bucket: cfg.MessageArchiveBucket, MinimumRetention: time.Duration(cfg.MessageArchiveRetentionDays) * 24 * time.Hour,
	})
	return store, cfg.MessageArchiveRetentionDays, err
}
