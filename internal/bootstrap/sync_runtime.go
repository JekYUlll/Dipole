package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	appcomposition "github.com/JekYUlll/Dipole/internal/app"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/data/migration"
	"github.com/JekYUlll/Dipole/internal/data/mysqlconfig"
	"github.com/JekYUlll/Dipole/internal/logger"
	platformobservability "github.com/JekYUlll/Dipole/internal/platform/observability"
	_ "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type SyncRuntime struct {
	rpc         *InternalRPCServer
	coreConn    *grpc.ClientConn
	db          *sql.DB
	metrics     *platformobservability.MetricsServer
	shutdownSec int
}

func InitializeSyncService(ctx context.Context) (*SyncRuntime, error) {
	return initializeSyncService(ctx, config.InternalRPCConfig(), config.MySQLConfig(), config.MetricsConfig())
}

func initializeSyncService(ctx context.Context, rpcCfg config.InternalRPC, mysqlCfg config.MySQL, metricsCfg config.Metrics) (*SyncRuntime, error) {
	if !rpcCfg.Enabled {
		return nil, fmt.Errorf("Sync Service requires internal_rpc.enabled")
	}
	runtime := &SyncRuntime{shutdownSec: rpcCfg.ShutdownTimeoutSeconds}
	db, err := sql.Open("mysql", mysqlconfig.DSN(mysqlCfg, false))
	if err != nil {
		return nil, fmt.Errorf("open Sync Service MySQL: %w", err)
	}
	runtime.db = db
	if err := db.PingContext(ctx); err != nil {
		runtime.Close()
		return nil, fmt.Errorf("ping Sync Service MySQL: %w", err)
	}
	migrationRunner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		runtime.Close()
		return nil, fmt.Errorf("initialize Sync Service migration validation: %w", err)
	}
	if err := migrationRunner.ValidateCurrent(ctx); err != nil {
		runtime.Close()
		return nil, fmt.Errorf("Sync Service database schema is not ready: %w", err)
	}
	repositories, err := appcomposition.NewSyncProcessRepositories(db)
	if err != nil {
		runtime.Close()
		return nil, fmt.Errorf("compose Sync Service repositories: %w", err)
	}
	core, coreConnection, err := DialSyncCoreCapability(ctx, rpcCfg)
	if err != nil {
		runtime.Close()
		return nil, err
	}
	runtime.coreConn = coreConnection
	syncApplication := appcomposition.NewSyncApplication(repositories.Sync, core)
	runtime.metrics, err = startRuntimeMetrics(metricsCfg, nil)
	if err != nil {
		runtime.Close()
		return nil, fmt.Errorf("start Sync Service metrics: %w", err)
	}
	runtime.rpc, err = NewSyncRPCServer(rpcCfg, syncApplication)
	if err != nil {
		runtime.Close()
		return nil, fmt.Errorf("start Sync rpc server: %w", err)
	}
	logger.Info("Sync Service runtime initialized")
	return runtime, nil
}

func (r *SyncRuntime) Address() string {
	if r == nil || r.rpc == nil {
		return ""
	}
	return r.rpc.Address()
}

func (r *SyncRuntime) Close() {
	if r == nil {
		return
	}
	shutdownSec := r.shutdownSec
	if shutdownSec <= 0 {
		shutdownSec = 15
	}
	if r.rpc != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(shutdownSec)*time.Second)
		r.rpc.Close(ctx)
		cancel()
		r.rpc = nil
	}
	if r.coreConn != nil {
		_ = r.coreConn.Close()
		r.coreConn = nil
	}
	if err := closeRuntimeMetrics(r.metrics); err != nil {
		logger.Warn("Sync Service metrics close failed", zap.Error(err))
	}
	r.metrics = nil
	if r.db != nil {
		_ = r.db.Close()
		r.db = nil
	}
}
