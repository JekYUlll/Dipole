package searchops

import (
	"context"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/config"
	mysqldata "github.com/JekYUlll/Dipole/internal/data/mysql"
	searchreconcile "github.com/JekYUlll/Dipole/internal/reconcile/search"
)

type SearchReconciliationOptions struct {
	JobName         string
	TargetIndex     string
	BatchSize       int
	MaxExamples     int
	Source          string
	ArchiveManifest string
}

func RunSearchReconciliation(ctx context.Context, options SearchReconciliationOptions) (searchreconcile.Report, error) {
	elasticsearchCfg := config.ElasticsearchConfig()
	if !elasticsearchCfg.Enabled {
		return searchreconcile.Report{}, fmt.Errorf("Search reconciliation requires elasticsearch.enabled")
	}
	db, err := openSearchMaintenanceMySQL(ctx, "reconciliation")
	if err != nil {
		return searchreconcile.Report{}, err
	}
	defer db.Close()
	store, err := mysqldata.NewStore(db)
	if err != nil {
		return searchreconcile.Report{}, err
	}
	source, err := openSearchSnapshotSource(options.Source, options.ArchiveManifest, store)
	if err != nil {
		return searchreconcile.Report{}, err
	}
	checkpoints, err := mysqldata.NewSearchBackfillCheckpointStore(store, options.TargetIndex)
	if err != nil {
		return searchreconcile.Report{}, err
	}
	highWatermark, err := checkpoints.CompletedHighWatermark(ctx, options.JobName)
	if err != nil {
		return searchreconcile.Report{}, err
	}
	descriptor, descriptorErr := source.Descriptor(ctx, highWatermark)
	if descriptorErr != nil {
		return searchreconcile.Report{}, descriptorErr
	}
	highWatermark, err = checkpoints.CompletedHighWatermarkForSource(ctx, options.JobName, descriptor)
	if err != nil {
		return searchreconcile.Report{}, err
	}
	index, client, err := openSearchMaintenanceIndex(elasticsearchCfg)
	if err != nil {
		return searchreconcile.Report{}, err
	}
	defer client.CloseIdleConnections()
	target, err := index.PhysicalTarget(ctx, options.TargetIndex)
	if err != nil {
		return searchreconcile.Report{}, fmt.Errorf("open Search reconciliation target: %w", err)
	}
	if err := target.Refresh(ctx); err != nil {
		return searchreconcile.Report{}, err
	}
	reconciler, err := searchreconcile.New(source, target, searchreconcile.Config{
		JobName: options.JobName, HighWatermarkID: highWatermark, BatchSize: options.BatchSize, MaxExamples: options.MaxExamples,
	})
	if err != nil {
		return searchreconcile.Report{}, err
	}
	return reconciler.Run(ctx)
}
