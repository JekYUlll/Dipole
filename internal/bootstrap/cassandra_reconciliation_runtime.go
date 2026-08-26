package bootstrap

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/config"
	cassandradata "github.com/JekYUlll/Dipole/internal/data/cassandra"
	"github.com/JekYUlll/Dipole/internal/data/migration"
	mysqldata "github.com/JekYUlll/Dipole/internal/data/mysql"
	"github.com/JekYUlll/Dipole/internal/data/mysqlconfig"
	cassandrareconcile "github.com/JekYUlll/Dipole/internal/reconcile/cassandra"
	_ "github.com/go-sql-driver/mysql"
)

type CassandraReconciliationOptions struct {
	JobName       string
	BatchSize     int
	SampleModulus uint64
	MaxExamples   int
}

func RunCassandraReconciliation(ctx context.Context, options CassandraReconciliationOptions) (cassandrareconcile.Report, error) {
	cassandraCfg := config.CassandraConfig()
	if !cassandraCfg.Enabled {
		return cassandrareconcile.Report{}, fmt.Errorf("Cassandra reconciliation requires cassandra.enabled")
	}
	db, err := sql.Open("mysql", mysqlconfig.DSN(config.MySQLConfig(), false))
	if err != nil {
		return cassandrareconcile.Report{}, fmt.Errorf("open Cassandra reconciliation MySQL source: %w", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return cassandrareconcile.Report{}, fmt.Errorf("ping Cassandra reconciliation MySQL source: %w", err)
	}
	migrationRunner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		return cassandrareconcile.Report{}, err
	}
	if err := migrationRunner.ValidateCurrent(ctx); err != nil {
		return cassandrareconcile.Report{}, fmt.Errorf("validate Cassandra reconciliation MySQL schema: %w", err)
	}
	mysqlStore, err := mysqldata.NewStore(db)
	if err != nil {
		return cassandrareconcile.Report{}, err
	}
	source, err := mysqldata.NewCassandraBackfillSource(mysqlStore)
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
