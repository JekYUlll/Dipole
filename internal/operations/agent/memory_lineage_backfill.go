package agentops

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/data/migration"
	mysqldata "github.com/JekYUlll/Dipole/internal/data/mysql"
	"github.com/JekYUlll/Dipole/internal/data/mysqlconfig"
	memorylineage "github.com/JekYUlll/Dipole/internal/operations/agent/memorylineage"
	_ "github.com/go-sql-driver/mysql"
)

type MemoryLineageBackfillOptions struct {
	JobName       string
	OwnerID       string
	BatchSize     int
	LeaseDuration time.Duration
}

func ReadMemoryLineageBackfillHighWatermark(ctx context.Context) (uint64, error) {
	db, err := openMemoryLineageMaintenanceMySQL(ctx)
	if err != nil {
		return 0, err
	}
	defer db.Close()
	store, err := mysqldata.NewStore(db)
	if err != nil {
		return 0, err
	}
	source, err := mysqldata.NewMemoryLineageBackfillSource(store)
	if err != nil {
		return 0, err
	}
	return source.HighWatermark(ctx)
}

func RunMemoryLineageBackfill(ctx context.Context, options MemoryLineageBackfillOptions) (memorylineage.Result, error) {
	db, err := openMemoryLineageMaintenanceMySQL(ctx)
	if err != nil {
		return memorylineage.Result{}, err
	}
	defer db.Close()
	store, err := mysqldata.NewStore(db)
	if err != nil {
		return memorylineage.Result{}, err
	}
	source, err := mysqldata.NewMemoryLineageBackfillSource(store)
	if err != nil {
		return memorylineage.Result{}, err
	}
	checkpoints, err := mysqldata.NewMemoryLineageBackfillCheckpointStore(store)
	if err != nil {
		return memorylineage.Result{}, err
	}
	target, err := mysqldata.NewMemoryLineageBackfillTarget(store)
	if err != nil {
		return memorylineage.Result{}, err
	}
	runner, err := memorylineage.NewRunner(source, checkpoints, target, memorylineage.Config{
		JobName: options.JobName, OwnerID: options.OwnerID, BatchSize: options.BatchSize, LeaseDuration: options.LeaseDuration,
	})
	if err != nil {
		return memorylineage.Result{}, err
	}
	return runner.Run(ctx)
}

func openMemoryLineageMaintenanceMySQL(ctx context.Context) (*sql.DB, error) {
	db, err := sql.Open("mysql", mysqlconfig.DSN(config.MySQLConfig(), false))
	if err != nil {
		return nil, fmt.Errorf("open Memory lineage backfill MySQL metadata: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping Memory lineage backfill MySQL metadata: %w", err)
	}
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		db.Close()
		return nil, err
	}
	if err := runner.ValidateCurrent(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("validate Memory lineage backfill MySQL schema: %w", err)
	}
	return db, nil
}
