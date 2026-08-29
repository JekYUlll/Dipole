package syncops

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/config"
	sqlcrepository "github.com/JekYUlll/Dipole/internal/data/mysql/repository"
	syncbackfill "github.com/JekYUlll/Dipole/internal/operations/sync/backfill"
	syncreconcile "github.com/JekYUlll/Dipole/internal/operations/sync/reconcile"
	syncreplaymysql "github.com/JekYUlll/Dipole/internal/operations/sync/replay/mysql"
	platformmysql "github.com/JekYUlll/Dipole/internal/platform/mysql"
	mysqlconfig "github.com/JekYUlll/Dipole/internal/platform/mysql/config"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/migration"
	_ "github.com/go-sql-driver/mysql"
)

type SyncReplayOptions struct {
	JobName       string
	OwnerID       string
	BatchSize     int
	LeaseDuration time.Duration
}

type SyncReconciliationOptions struct {
	JobName     string
	BatchSize   int
	MaxExamples int
}

func RunSyncReplay(ctx context.Context, options SyncReplayOptions) (syncbackfill.Result, error) {
	db, store, err := openSyncRecoveryStore(ctx, "replay")
	if err != nil {
		return syncbackfill.Result{}, err
	}
	defer db.Close()
	source, err := syncreplaymysql.NewSyncReplaySource(store)
	if err != nil {
		return syncbackfill.Result{}, err
	}
	checkpoints, err := syncreplaymysql.NewSyncReplayCheckpointStore(store)
	if err != nil {
		return syncbackfill.Result{}, err
	}
	target, err := sqlcrepository.NewSyncProjectionRepository(store)
	if err != nil {
		return syncbackfill.Result{}, err
	}
	runner, err := syncbackfill.NewRunner(source, checkpoints, target, syncbackfill.Config{
		JobName: options.JobName, OwnerID: options.OwnerID,
		BatchSize: options.BatchSize, LeaseDuration: options.LeaseDuration,
	})
	if err != nil {
		return syncbackfill.Result{}, err
	}
	return runner.Run(ctx)
}

func RunSyncReconciliation(ctx context.Context, options SyncReconciliationOptions) (syncreconcile.Report, error) {
	db, store, err := openSyncRecoveryStore(ctx, "reconciliation")
	if err != nil {
		return syncreconcile.Report{}, err
	}
	defer db.Close()
	source, err := syncreplaymysql.NewSyncReplaySource(store)
	if err != nil {
		return syncreconcile.Report{}, err
	}
	checkpoints, err := syncreplaymysql.NewSyncReplayCheckpointStore(store)
	if err != nil {
		return syncreconcile.Report{}, err
	}
	target, err := syncreplaymysql.NewSyncInboxReconcileTarget(store)
	if err != nil {
		return syncreconcile.Report{}, err
	}
	reconciler, err := syncreconcile.NewReconciler(source, checkpoints, target, syncreconcile.Config{
		JobName: options.JobName, BatchSize: options.BatchSize, MaxExamples: options.MaxExamples,
	})
	if err != nil {
		return syncreconcile.Report{}, err
	}
	return reconciler.Run(ctx)
}

func openSyncRecoveryStore(ctx context.Context, operation string) (*sql.DB, *platformmysql.Store, error) {
	db, err := sql.Open("mysql", mysqlconfig.DSN(config.SyncMySQLConfig(), false))
	if err != nil {
		return nil, nil, fmt.Errorf("open Sync %s MySQL: %w", operation, err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("ping Sync %s MySQL: %w", operation, err)
	}
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		db.Close()
		return nil, nil, err
	}
	if err := runner.ValidateCurrent(ctx); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("validate Sync %s MySQL schema: %w", operation, err)
	}
	store, err := platformmysql.NewStore(db)
	if err != nil {
		db.Close()
		return nil, nil, err
	}
	return db, store, nil
}
