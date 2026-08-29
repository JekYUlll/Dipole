package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	searchbackfill "github.com/JekYUlll/Dipole/internal/backfill/search"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/data/migration"
	mysqldata "github.com/JekYUlll/Dipole/internal/data/mysql"
	"github.com/JekYUlll/Dipole/internal/data/mysqlconfig"
	"github.com/JekYUlll/Dipole/internal/platform/elasticsearch"
	_ "github.com/go-sql-driver/mysql"
)

type SearchBackfillOptions struct {
	JobName         string
	OwnerID         string
	TargetIndex     string
	BatchSize       int
	LeaseDuration   time.Duration
	Source          string
	ArchiveManifest string
}

func RunSearchBackfill(ctx context.Context, options SearchBackfillOptions) (searchbackfill.Result, error) {
	elasticsearchCfg := config.ElasticsearchConfig()
	if !elasticsearchCfg.Enabled {
		return searchbackfill.Result{}, fmt.Errorf("Search backfill requires elasticsearch.enabled")
	}
	db, err := openSearchMaintenanceMySQL(ctx, "backfill")
	if err != nil {
		return searchbackfill.Result{}, err
	}
	defer db.Close()
	store, err := mysqldata.NewStore(db)
	if err != nil {
		return searchbackfill.Result{}, err
	}
	source, err := openSearchSnapshotSource(options.Source, options.ArchiveManifest, store)
	if err != nil {
		return searchbackfill.Result{}, err
	}
	checkpoints, err := mysqldata.NewSearchBackfillCheckpointStore(store, options.TargetIndex)
	if err != nil {
		return searchbackfill.Result{}, err
	}
	index, client, err := openSearchMaintenanceIndex(elasticsearchCfg)
	if err != nil {
		return searchbackfill.Result{}, err
	}
	defer client.CloseIdleConnections()
	target, err := index.CreatePhysicalTarget(ctx, options.TargetIndex)
	if err != nil {
		return searchbackfill.Result{}, fmt.Errorf("prepare Search backfill target: %w", err)
	}
	runner, err := searchbackfill.NewRunner(source, checkpoints, target, searchbackfill.Config{
		JobName: options.JobName, OwnerID: options.OwnerID, BatchSize: options.BatchSize, LeaseDuration: options.LeaseDuration,
	})
	if err != nil {
		return searchbackfill.Result{}, err
	}
	return runner.Run(ctx)
}

func openSearchMaintenanceMySQL(ctx context.Context, operation string) (*sql.DB, error) {
	db, err := sql.Open("mysql", mysqlconfig.DSN(config.SearchMySQLConfig(), false))
	if err != nil {
		return nil, fmt.Errorf("open Search %s MySQL source: %w", operation, err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping Search %s MySQL source: %w", operation, err)
	}
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		db.Close()
		return nil, err
	}
	if err := runner.ValidateCurrent(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("validate Search %s MySQL schema: %w", operation, err)
	}
	return db, nil
}

func openSearchMaintenanceIndex(cfg config.Elasticsearch) (*elasticsearch.Index, *http.Client, error) {
	if cfg.RequestTimeoutSeconds <= 0 {
		return nil, nil, fmt.Errorf("Elasticsearch request timeout must be positive")
	}
	client := &http.Client{Timeout: time.Duration(cfg.RequestTimeoutSeconds) * time.Second}
	index, err := elasticsearch.NewIndex(elasticsearch.Config{
		Address: cfg.Address, IndexPrefix: cfg.IndexPrefix, Shards: cfg.Shards, Replicas: cfg.Replicas,
		Username: cfg.Username, Password: cfg.Password, APIKey: cfg.APIKey, HTTPClient: client,
	})
	if err != nil {
		client.CloseIdleConnections()
		return nil, nil, err
	}
	return index, client, nil
}
