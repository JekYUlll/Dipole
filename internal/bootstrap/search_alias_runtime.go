package bootstrap

import (
	"context"
	"fmt"
	"time"

	searchbackfill "github.com/JekYUlll/Dipole/internal/backfill/search"
	"github.com/JekYUlll/Dipole/internal/config"
	searchcutover "github.com/JekYUlll/Dipole/internal/cutover/search"
	mysqldata "github.com/JekYUlll/Dipole/internal/data/mysql"
	"github.com/JekYUlll/Dipole/internal/platform/elasticsearch"
	searchreconcile "github.com/JekYUlll/Dipole/internal/reconcile/search"
)

type SearchAliasOptions struct {
	Action               searchcutover.Action
	JobName              string
	FromIndex            string
	ToIndex              string
	BatchSize            int
	MaxExamples          int
	MaintenanceConfirmed bool
	RollbackWindow       time.Duration
	Source               string
	ArchiveManifest      string
}

func RunSearchAliasOperation(ctx context.Context, options SearchAliasOptions) (searchcutover.Receipt, error) {
	elasticsearchCfg := config.ElasticsearchConfig()
	if !elasticsearchCfg.Enabled {
		return searchcutover.Receipt{}, fmt.Errorf("Search alias operation requires elasticsearch.enabled")
	}
	db, err := openSearchMaintenanceMySQL(ctx, "alias operation")
	if err != nil {
		return searchcutover.Receipt{}, err
	}
	defer db.Close()
	store, err := mysqldata.NewStore(db)
	if err != nil {
		return searchcutover.Receipt{}, err
	}
	source, err := openSearchSnapshotSource(options.Source, options.ArchiveManifest, store)
	if err != nil {
		return searchcutover.Receipt{}, err
	}
	checkpoints, err := mysqldata.NewSearchBackfillCheckpointStore(store, options.ToIndex)
	if err != nil {
		return searchcutover.Receipt{}, err
	}
	snapshot := &sourceBoundSearchSnapshot{checkpoints: checkpoints, source: source}
	index, client, err := openSearchMaintenanceIndex(elasticsearchCfg)
	if err != nil {
		return searchcutover.Receipt{}, err
	}
	defer client.CloseIdleConnections()
	target, err := index.PhysicalTarget(ctx, options.ToIndex)
	if err != nil {
		return searchcutover.Receipt{}, fmt.Errorf("open Search alias target: %w", err)
	}
	verifier := &searchCutoverVerifier{
		source: source, target: target, jobName: options.JobName,
		batchSize: options.BatchSize, maxExamples: options.MaxExamples,
	}
	switcher, err := searchcutover.New(source, snapshot, verifier, index, searchcutover.Config{
		Action: options.Action, JobName: options.JobName, FromIndex: options.FromIndex, ToIndex: options.ToIndex,
		MaintenanceConfirmed: options.MaintenanceConfirmed, RollbackWindow: options.RollbackWindow,
	})
	if err != nil {
		return searchcutover.Receipt{}, err
	}
	return switcher.Run(ctx)
}

type sourceBoundSearchSnapshot struct {
	checkpoints *mysqldata.SearchBackfillCheckpointStore
	source      searchbackfill.Source
}

func (s *sourceBoundSearchSnapshot) CompletedHighWatermark(ctx context.Context, jobName string) (uint64, error) {
	highWatermark, err := s.checkpoints.CompletedHighWatermark(ctx, jobName)
	if err != nil {
		return 0, err
	}
	descriptor, err := s.source.Descriptor(ctx, highWatermark)
	if err != nil {
		return 0, err
	}
	return s.checkpoints.CompletedHighWatermarkForSource(ctx, jobName, descriptor)
}

type searchCutoverVerifier struct {
	source      searchreconcile.Source
	target      *elasticsearch.PhysicalTarget
	jobName     string
	batchSize   int
	maxExamples int
}

func (v *searchCutoverVerifier) Verify(ctx context.Context, highWatermark uint64) (searchcutover.Verification, error) {
	if err := v.target.Refresh(ctx); err != nil {
		return searchcutover.Verification{}, err
	}
	reconciler, err := searchreconcile.New(v.source, v.target, searchreconcile.Config{
		JobName: v.jobName, HighWatermarkID: highWatermark, BatchSize: v.batchSize, MaxExamples: v.maxExamples,
	})
	if err != nil {
		return searchcutover.Verification{}, err
	}
	report, err := reconciler.Run(ctx)
	if err != nil {
		return searchcutover.Verification{}, err
	}
	return searchcutover.Verification{Consistent: report.Consistent, SourceCount: report.SourceCount, TargetCount: report.TargetCount}, nil
}
