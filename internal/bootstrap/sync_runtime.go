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
	platformkafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	platformobservability "github.com/JekYUlll/Dipole/internal/platform/observability"
	syncprojector "github.com/JekYUlll/Dipole/internal/projector/sync"
	_ "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type SyncRuntime struct {
	rpc         *InternalRPCServer
	coreConn    *grpc.ClientConn
	db          *sql.DB
	metrics     *platformobservability.MetricsServer
	projector   bool
	shutdownSec int
}

func InitializeSyncService(ctx context.Context) (*SyncRuntime, error) {
	return initializeSyncService(ctx, config.InternalRPCConfig(), config.MySQLConfig(), config.MetricsConfig(), config.SyncConfig(), config.KafkaConfig())
}

func initializeSyncService(ctx context.Context, rpcCfg config.InternalRPC, mysqlCfg config.MySQL, metricsCfg config.Metrics, syncCfg config.Sync, kafkaCfg config.Kafka) (*SyncRuntime, error) {
	if !rpcCfg.Enabled {
		return nil, fmt.Errorf("Sync Service requires internal_rpc.enabled")
	}
	if err := validateSyncProjectorConfig(syncCfg, kafkaCfg); err != nil {
		return nil, err
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
	var subscriber *platformkafka.Consumer
	if syncCfg.ProjectorEnabled {
		projector, projectorErr := syncprojector.New(repositories.Projection)
		if projectorErr != nil {
			runtime.Close()
			return nil, fmt.Errorf("compose Sync projector: %w", projectorErr)
		}
		if projectorErr = platformkafka.Init(); projectorErr != nil {
			runtime.Close()
			return nil, fmt.Errorf("initialize Sync projector Kafka publisher: %w", projectorErr)
		}
		runtime.projector = true
		if projectorErr = platformkafka.InitConsumerForService(syncServiceName); projectorErr != nil {
			runtime.Close()
			return nil, fmt.Errorf("initialize Sync projector Kafka consumer: %w", projectorErr)
		}
		subscriber = platformkafka.Subscriber
		topics := syncprojector.Topics()
		for _, topic := range topics {
			platformkafka.Subscriber.Register(topic, projector.Handler())
		}
		if projectorErr = platformkafka.Client.EnsureTopics(topics); projectorErr != nil {
			runtime.Close()
			return nil, fmt.Errorf("ensure Sync projector topics: %w", projectorErr)
		}
		if projectorErr = platformkafka.Subscriber.Start(ctx); projectorErr != nil {
			runtime.Close()
			return nil, fmt.Errorf("start Sync projector consumer: %w", projectorErr)
		}
	}
	runtime.metrics, err = startRuntimeMetrics(metricsCfg, subscriber)
	if err != nil {
		runtime.Close()
		return nil, fmt.Errorf("start Sync Service metrics: %w", err)
	}
	runtime.rpc, err = NewSyncRPCServer(rpcCfg, syncApplication)
	if err != nil {
		runtime.Close()
		return nil, fmt.Errorf("start Sync rpc server: %w", err)
	}
	logger.Info("Sync Service runtime initialized", zap.Bool("projector_enabled", syncCfg.ProjectorEnabled))
	return runtime, nil
}

func validateSyncProjectorConfig(syncCfg config.Sync, kafkaCfg config.Kafka) error {
	if syncCfg.ProjectorEnabled && !kafkaCfg.Enabled {
		return fmt.Errorf("Sync projector requires kafka.enabled")
	}
	return nil
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
	if r.projector {
		if err := platformkafka.CloseConsumer(); err != nil {
			logger.Warn("Sync projector Kafka consumer close failed", zap.Error(err))
		}
		if err := platformkafka.Close(); err != nil {
			logger.Warn("Sync projector Kafka publisher close failed", zap.Error(err))
		}
		r.projector = false
	}
	if r.db != nil {
		_ = r.db.Close()
		r.db = nil
	}
}
