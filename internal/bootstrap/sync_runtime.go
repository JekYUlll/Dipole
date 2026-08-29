package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/logger"
	cassandradata "github.com/JekYUlll/Dipole/internal/platform/cassandra"
	platformkafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/config"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/migration"
	platformobservability "github.com/JekYUlll/Dipole/internal/platform/observability"
	shadowdata "github.com/JekYUlll/Dipole/internal/platform/storage/shadow"
	syncapplication "github.com/JekYUlll/Dipole/internal/services/sync/application"
	syncprojector "github.com/JekYUlll/Dipole/internal/services/sync/infrastructure/kafka"
	syncmysql "github.com/JekYUlll/Dipole/internal/services/sync/infrastructure/mysql"
	"github.com/apache/cassandra-gocql-driver/v2"
	_ "github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type SyncRuntime struct {
	rpc              *InternalRPCServer
	coreConn         *grpc.ClientConn
	db               *sql.DB
	metrics          *platformobservability.MetricsServer
	projector        bool
	shadowHydrator   *shadowdata.SyncMessageHydrator
	hydrationMetrics *shadowdata.SyncHydrationMetrics
	cassandra        *gocql.Session
	shutdownSec      int
}

func InitializeSyncService(ctx context.Context) (*SyncRuntime, error) {
	return initializeSyncService(ctx, config.InternalRPCConfig(), config.SyncMySQLConfig(), config.MetricsConfig(), config.SyncConfig(), config.KafkaConfig(), config.CassandraConfig())
}

func initializeSyncService(ctx context.Context, rpcCfg config.InternalRPC, mysqlCfg config.MySQL, metricsCfg config.Metrics, syncCfg config.Sync, kafkaCfg config.Kafka, cassandraCfg config.Cassandra) (*SyncRuntime, error) {
	if !rpcCfg.Enabled {
		return nil, fmt.Errorf("Sync Service requires internal_rpc.enabled")
	}
	if err := validateSyncProjectorConfig(syncCfg, kafkaCfg); err != nil {
		return nil, err
	}
	if err := validateSyncHydrationConfig(syncCfg, cassandraCfg); err != nil {
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
	if syncCfg.EnforceDBPermissions {
		if err := verifySyncDatabaseBoundary(ctx, db); err != nil {
			runtime.Close()
			return nil, fmt.Errorf("verify Sync Service database permissions: %w", err)
		}
	}
	primaryHydrator, err := syncmysql.NewMySQLSyncMessageHydrator(generated.New(db))
	if err != nil {
		runtime.Close()
		return nil, fmt.Errorf("compose MySQL Sync message hydrator: %w", err)
	}
	var hydrator application.SyncMessageHydrator = primaryHydrator
	if syncCfg.CassandraShadowHydration || syncCfg.CassandraPrimaryHydration {
		runtime.cassandra, err = cassandradata.OpenSession(cassandraCfg)
		if err != nil {
			runtime.Close()
			return nil, fmt.Errorf("open Cassandra Sync shadow session: %w", err)
		}
		if err = cassandradata.ValidateTimelineSchema(ctx, runtime.cassandra, cassandraCfg.Keyspace); err != nil {
			runtime.Close()
			return nil, err
		}
		timeline, timelineErr := cassandradata.NewTimelineStore(runtime.cassandra, cassandraCfg.TimelineBucketSize)
		if timelineErr != nil {
			runtime.Close()
			return nil, fmt.Errorf("create Cassandra Sync timeline reader: %w", timelineErr)
		}
		cassandraHydrator, hydrationErr := cassandradata.NewSyncMessageHydrator(timeline)
		if hydrationErr != nil {
			runtime.Close()
			return nil, hydrationErr
		}
		if syncCfg.CassandraShadowHydration {
			runtime.shadowHydrator = shadowdata.NewSyncMessageHydrator(primaryHydrator, cassandraHydrator, logSyncHydrationComparison)
			hydrator = runtime.shadowHydrator
		} else {
			runtime.hydrationMetrics = shadowdata.NewSyncHydrationMetrics()
			fallbackHydrator, fallbackErr := shadowdata.NewFallbackSyncMessageHydratorWithMetrics(cassandraHydrator, primaryHydrator, logSyncHydrationRoute, runtime.hydrationMetrics)
			if fallbackErr != nil {
				runtime.Close()
				return nil, fallbackErr
			}
			hydrator = fallbackHydrator
		}
	}
	repositories, err := syncmysql.NewProcessRepositories(db, hydrator)
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
	syncApplication := syncapplication.New(repositories.Sync, core)
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
		if projectorErr = platformkafka.InitReplayableConsumerForService(syncServiceName); projectorErr != nil {
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
	var hydrationCollectors []prometheus.Collector
	if runtime.shadowHydrator != nil {
		hydrationCollectors = append(hydrationCollectors, runtime.shadowHydrator)
	}
	if runtime.hydrationMetrics != nil {
		hydrationCollectors = append(hydrationCollectors, runtime.hydrationMetrics)
	}
	runtime.metrics, err = startRuntimeMetrics(metricsCfg, syncServiceName, subscriber, hydrationCollectors...)
	if err != nil {
		runtime.Close()
		return nil, fmt.Errorf("start Sync Service metrics: %w", err)
	}
	if err := configureRuntimeDependencyReadiness(runtime.metrics, metricsCfg,
		mysqlReadinessProbe("mysql", runtime.db), grpcReadinessProbe("core-rpc", runtime.coreConn),
	); err != nil {
		runtime.Close()
		return nil, fmt.Errorf("configure Sync dependency readiness: %w", err)
	}
	runtime.rpc, err = NewSyncRPCServer(rpcCfg, syncApplication)
	if err != nil {
		runtime.Close()
		return nil, fmt.Errorf("start Sync rpc server: %w", err)
	}
	if runtime.metrics != nil {
		bindRPCReadiness(runtime.metrics, runtime.rpc)
		markRuntimeReady(runtime.metrics)
	}
	logger.Info("Sync Service runtime initialized", zap.Bool("projector_enabled", syncCfg.ProjectorEnabled), zap.Bool("cassandra_shadow_hydration", syncCfg.CassandraShadowHydration), zap.Bool("cassandra_primary_hydration", syncCfg.CassandraPrimaryHydration))
	return runtime, nil
}

func validateSyncHydrationConfig(syncCfg config.Sync, cassandraCfg config.Cassandra) error {
	if syncCfg.CassandraShadowHydration && syncCfg.CassandraPrimaryHydration {
		return fmt.Errorf("Sync Cassandra shadow and primary hydration are mutually exclusive")
	}
	if (syncCfg.CassandraShadowHydration || syncCfg.CassandraPrimaryHydration) && !cassandraCfg.Enabled {
		return fmt.Errorf("Sync Cassandra hydration requires cassandra.enabled")
	}
	return nil
}

func logSyncHydrationRoute(outcome string) {
	logger.Debug("Sync Cassandra hydration route selected", zap.String("outcome", outcome))
}

func logSyncHydrationComparison(comparison shadowdata.SyncHydrationComparison) {
	fields := []zap.Field{zap.Bool("match", comparison.Match), zap.Bool("skipped", comparison.Skipped), zap.String("skip_reason", comparison.SkipReason), zap.Int("primary_count", comparison.PrimaryCount), zap.Int("shadow_count", comparison.ShadowCount), zap.String("shadow_error", comparison.ShadowError)}
	if comparison.Match || comparison.Skipped {
		logger.Debug("Sync Cassandra hydration shadow compared", fields...)
		return
	}
	logger.Warn("Sync Cassandra hydration shadow mismatch", fields...)
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
	if err := closeRuntimeMetrics(r.metrics); err != nil {
		logger.Warn("Sync Service metrics close failed", zap.Error(err))
	}
	r.metrics = nil
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
	if r.shadowHydrator != nil {
		r.shadowHydrator.Wait()
		r.shadowHydrator = nil
	}
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
	if r.cassandra != nil {
		r.cassandra.Close()
		r.cassandra = nil
	}
}
