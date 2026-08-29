package bootstrap

import (
	"context"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/config"
	mysqldata "github.com/JekYUlll/Dipole/internal/data/mysql"
	cassandradata "github.com/JekYUlll/Dipole/internal/platform/cassandra"
	cassandrareconcile "github.com/JekYUlll/Dipole/internal/reconcile/cassandra"
)

type CassandraReconciliationOptions struct {
	JobName         string
	BatchSize       int
	SampleModulus   uint64
	MaxExamples     int
	Source          string
	ArchiveManifest string
}

func RunCassandraReconciliation(ctx context.Context, options CassandraReconciliationOptions) (cassandrareconcile.Report, error) {
	cassandraCfg := config.CassandraConfig()
	if !cassandraCfg.Enabled {
		return cassandrareconcile.Report{}, fmt.Errorf("Cassandra reconciliation requires cassandra.enabled")
	}
	db, err := openCassandraMaintenanceMySQL(ctx, "reconciliation")
	if err != nil {
		return cassandrareconcile.Report{}, err
	}
	defer db.Close()
	mysqlStore, err := mysqldata.NewStore(db)
	if err != nil {
		return cassandrareconcile.Report{}, err
	}
	source, err := openCassandraSnapshotSource(options.Source, options.ArchiveManifest, mysqlStore)
	if err != nil {
		return cassandrareconcile.Report{}, err
	}
	checkpoints, err := mysqldata.NewCassandraBackfillCheckpointStore(mysqlStore)
	if err != nil {
		return cassandrareconcile.Report{}, err
	}
	highWatermark, err := checkpoints.CompletedHighWatermark(ctx, options.JobName)
	if err != nil {
		return cassandrareconcile.Report{}, err
	}
	descriptor, err := source.Descriptor(ctx, highWatermark)
	if err != nil {
		return cassandrareconcile.Report{}, fmt.Errorf("describe Cassandra reconciliation source: %w", err)
	}
	if _, err := checkpoints.CompletedHighWatermarkForSource(ctx, options.JobName, descriptor); err != nil {
		return cassandrareconcile.Report{}, err
	}

	session, err := cassandradata.OpenSession(cassandraCfg)
	if err != nil {
		return cassandrareconcile.Report{}, err
	}
	defer session.Close()
	if err := cassandradata.ValidateTimelineSchema(ctx, session, cassandraCfg.Keyspace); err != nil {
		return cassandrareconcile.Report{}, err
	}
	timeline, err := cassandradata.NewTimelineStore(session, cassandraCfg.TimelineBucketSize)
	if err != nil {
		return cassandrareconcile.Report{}, err
	}
	reconciler, err := cassandrareconcile.New(source, timeline, cassandrareconcile.Config{
		JobName: options.JobName, HighWatermarkID: highWatermark, BatchSize: options.BatchSize,
		SampleModulus: options.SampleModulus, MaxExamples: options.MaxExamples,
	})
	if err != nil {
		return cassandrareconcile.Report{}, err
	}
	return reconciler.Run(ctx)
}
