package cassandraops

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	cassandrabackfill "github.com/JekYUlll/Dipole/internal/backfill/cassandra"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/data/migration"
	mysqldata "github.com/JekYUlll/Dipole/internal/data/mysql"
	"github.com/JekYUlll/Dipole/internal/data/mysqlconfig"
	cassandradata "github.com/JekYUlll/Dipole/internal/platform/cassandra"
	_ "github.com/go-sql-driver/mysql"
)

type CassandraBackfillOptions struct {
	JobName         string
	OwnerID         string
	BatchSize       int
	LeaseDuration   time.Duration
	Source          string
	ArchiveManifest string
}

func RunCassandraBackfill(ctx context.Context, options CassandraBackfillOptions) (cassandrabackfill.Result, error) {
	cassandraCfg := config.CassandraConfig()
	if !cassandraCfg.Enabled {
		return cassandrabackfill.Result{}, fmt.Errorf("Cassandra backfill requires cassandra.enabled")
	}

	db, err := openCassandraMaintenanceMySQL(ctx, "backfill")
	if err != nil {
		return cassandrabackfill.Result{}, err
	}
	defer db.Close()
	mysqlStore, err := mysqldata.NewStore(db)
	if err != nil {
		return cassandrabackfill.Result{}, err
	}
	source, err := openCassandraSnapshotSource(options.Source, options.ArchiveManifest, mysqlStore)
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

func openCassandraSnapshotSource(kind, manifestPath string, store *mysqldata.Store) (cassandrabackfill.Source, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "", "mysql":
		return mysqldata.NewCassandraBackfillSource(store)
	case "archive":
		if strings.TrimSpace(manifestPath) == "" {
			return nil, fmt.Errorf("Cassandra archive source requires an archive manifest")
		}
		return cassandrabackfill.OpenArchive(manifestPath)
	default:
		return nil, fmt.Errorf("unsupported Cassandra snapshot source: %s", kind)
	}
}

func openCassandraMaintenanceMySQL(ctx context.Context, operation string) (*sql.DB, error) {
	db, err := sql.Open("mysql", mysqlconfig.DSN(config.MySQLConfig(), false))
	if err != nil {
		return nil, fmt.Errorf("open Cassandra %s MySQL metadata: %w", operation, err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping Cassandra %s MySQL metadata: %w", operation, err)
	}
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		db.Close()
		return nil, err
	}
	if err := runner.ValidateCurrent(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("validate Cassandra %s MySQL schema: %w", operation, err)
	}
	return db, nil
}
