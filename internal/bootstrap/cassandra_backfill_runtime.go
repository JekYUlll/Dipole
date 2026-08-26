package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	cassandrabackfill "github.com/JekYUlll/Dipole/internal/backfill/cassandra"
	"github.com/JekYUlll/Dipole/internal/config"
	cassandradata "github.com/JekYUlll/Dipole/internal/data/cassandra"
	"github.com/JekYUlll/Dipole/internal/data/migration"
	mysqldata "github.com/JekYUlll/Dipole/internal/data/mysql"
	"github.com/JekYUlll/Dipole/internal/data/mysqlconfig"
	_ "github.com/go-sql-driver/mysql"
)

type CassandraBackfillOptions struct {
	JobName       string
	OwnerID       string
	BatchSize     int
	LeaseDuration time.Duration
}

func RunCassandraBackfill(ctx context.Context, options CassandraBackfillOptions) (cassandrabackfill.Result, error) {
	cassandraCfg := config.CassandraConfig()
	if !cassandraCfg.Enabled {
		return cassandrabackfill.Result{}, fmt.Errorf("Cassandra backfill requires cassandra.enabled")
	}

	db, err := sql.Open("mysql", mysqlconfig.DSN(config.MySQLConfig(), false))
	if err != nil {
		return cassandrabackfill.Result{}, fmt.Errorf("open Cassandra backfill MySQL source: %w", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return cassandrabackfill.Result{}, fmt.Errorf("ping Cassandra backfill MySQL source: %w", err)
	}
	migrationRunner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		return cassandrabackfill.Result{}, err
	}
	if err := migrationRunner.ValidateCurrent(ctx); err != nil {
		return cassandrabackfill.Result{}, fmt.Errorf("validate Cassandra backfill MySQL schema: %w", err)
	}
	mysqlStore, err := mysqldata.NewStore(db)
	if err != nil {
		return cassandrabackfill.Result{}, err
	}
	source, err := mysqldata.NewCassandraBackfillSource(mysqlStore)
	if err != nil {
		return cassandrabackfill.Result{}, err
	}
	checkpoints, err := mysqldata.NewCassandraBackfillCheckpointStore(mysqlStore)
	if err != nil {
		return cassandrabackfill.Result{}, err
	}

	session, err := cassandradata.OpenSession(cassandraCfg)
	if err != nil {
		return cassandrabackfill.Result{}, err
	}
	defer session.Close()
	if err := cassandradata.ValidateTimelineSchema(ctx, session, cassandraCfg.Keyspace); err != nil {
		return cassandrabackfill.Result{}, err
	}
	timeline, err := cassandradata.NewTimelineStore(session, cassandraCfg.TimelineBucketSize)
	if err != nil {
		return cassandrabackfill.Result{}, err
	}
	runner, err := cassandrabackfill.NewRunner(source, checkpoints, timeline, cassandrabackfill.Config{
		JobName: options.JobName, OwnerID: options.OwnerID,
		BatchSize: options.BatchSize, LeaseDuration: options.LeaseDuration,
	})
	if err != nil {
		return cassandrabackfill.Result{}, err
	}
	return runner.Run(ctx)
}
