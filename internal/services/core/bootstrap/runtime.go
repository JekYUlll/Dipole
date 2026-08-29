package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	appComposition "github.com/JekYUlll/Dipole/internal/bootstrap/embedded"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/logger"
	platformBloom "github.com/JekYUlll/Dipole/internal/platform/bloom"
	"github.com/JekYUlll/Dipole/internal/platform/cache"
	platformHotGroup "github.com/JekYUlll/Dipole/internal/platform/hotgroup"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	platformmysql "github.com/JekYUlll/Dipole/internal/platform/mysql"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/migration"
	platformObservability "github.com/JekYUlll/Dipole/internal/platform/observability"
	platformRuntime "github.com/JekYUlll/Dipole/internal/platform/runtime"
	platformStorage "github.com/JekYUlll/Dipole/internal/platform/storage"
	"github.com/JekYUlll/Dipole/internal/server"
	coreapplication "github.com/JekYUlll/Dipole/internal/services/core/application"
	corekafka "github.com/JekYUlll/Dipole/internal/services/core/infrastructure/kafka"
	coremysql "github.com/JekYUlll/Dipole/internal/services/core/infrastructure/mysql"
	"go.uber.org/zap"
)

// CoreRuntime owns only Core repositories, Core projections and the Core
// capability RPC. The embedded aggregate runtime remains available for local
// compatibility mode and rollback.
type CoreRuntime struct {
	server        *server.Server
	coreRPC       *InternalRPCServer
	metrics       *platformObservability.MetricsServer
	messageSender *lazyCoreMessageSender
}

func InitializeCoreService(ctx context.Context) (*CoreRuntime, error) {
	gatewayMode := config.GatewayConfig().Mode
	if err := validateStandaloneCoreMode(gatewayMode); err != nil {
		return nil, err
	}

	if err := platformmysql.InitMySQL(); err != nil {
		return nil, fmt.Errorf("Core MySQL init failed: %w", err)
	}
	if err := cache.InitRedis(); err != nil {
		return nil, fmt.Errorf("Core Redis init failed: %w", err)
	}
	if err := platformStorage.Init(); err != nil {
		return nil, fmt.Errorf("Core storage init failed: %w", err)
	}
	runner, err := migration.NewRunner(platformmysql.SQLDB, migrations.Files)
	if err != nil {
		return nil, fmt.Errorf("initialize Core migration validation: %w", err)
	}
	if err := runner.ValidateCurrent(ctx); err != nil {
		return nil, fmt.Errorf("Core database schema is not ready: %w", err)
	}
	coreRepos, err := coremysql.NewProcessRepositories(platformmysql.SQLDB)
	if err != nil {
		return nil, fmt.Errorf("compose Core repositories: %w", err)
	}
	if err := coreapplication.EnsureAIAssistantUser(coreRepos.Users); err != nil {
		return nil, fmt.Errorf("ensure AI assistant user: %w", err)
	}
	if err := platformBloom.Init(); err != nil {
		return nil, fmt.Errorf("Core bloom filter init failed: %w", err)
	}

	if err := platformKafka.Init(); err != nil {
		return nil, fmt.Errorf("Core Kafka publisher init failed: %w", err)
	}
	if err := platformKafka.InitConsumerForService(coreServiceName); err != nil {
		return nil, fmt.Errorf("Core Kafka consumer init failed: %w", err)
	}

	var events applicationPort.EventPublisher
	if config.KafkaConfig().Enabled {
		events = platformKafka.Client
	}
	processRepos := &appComposition.Repositories{
		CoreProcess:   coreRepos,
		Users:         coreRepos.Users,
		Files:         coreRepos.Files,
		Conversations: coreRepos.Conversations,
		Contacts:      coreRepos.Contacts,
		Groups:        coreRepos.Groups,
		Admin:         coreRepos.Admin,
	}
	messaging := appComposition.NewMessagingServicesFromProcesses(
		coreRepos,
		&appComposition.MessageProcessRepositories{},
		&appComposition.SyncProcessRepositories{},
		appComposition.MessagingDependencies{
			Events:    events,
			HotGroups: platformHotGroup.NewDetectorWithClient(config.HotGroupConfig(), cache.RDB),
			Storage:   platformStorage.Client,
		},
	)

	runtime := &CoreRuntime{}
	var systemMessages applicationPort.SystemMessageSender
	if config.CoreMessageConfig().Transport == "grpc" {
		runtime.messageSender = newLazyCoreMessageSender(config.InternalRPCConfig())
		systemMessages = runtime.messageSender
	}
	cleanup := func() { runtime.Close() }
	runtime.server = server.NewWithDependencies(processRepos, server.Dependencies{Messaging: messaging, SystemMessages: systemMessages})
	if err := corekafka.RegisterConversationProjections(messaging.Conversations); err != nil {
		cleanup()
		return nil, fmt.Errorf("register Core Kafka projections: %w", err)
	}
	if platformKafka.Subscriber != nil {
		if err := platformKafka.Subscriber.Start(ctx); err != nil {
			cleanup()
			return nil, fmt.Errorf("start Core Kafka consumer: %w", err)
		}
	}

	rpcCfg := config.InternalRPCConfig()
	if rpcCfg.Enabled {
		runtime.coreRPC, err = NewCoreRPCServer(rpcCfg, messaging.Core)
		if err != nil {
			cleanup()
			return nil, fmt.Errorf("initialize Core capability RPC: %w", err)
		}
	}
	runtime.metrics, err = platformRuntime.StartMetrics(config.MetricsConfig(), coreServiceName, platformKafka.Subscriber)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("start Core metrics: %w", err)
	}
	if runtime.metrics != nil {
		probes := []platformObservability.DependencyProbe{
			platformRuntime.MySQLReadinessProbe("mysql", platformmysql.SQLDB),
			platformRuntime.RedisReadinessProbe("redis", cache.RDB),
		}
		if platformKafka.Client != nil {
			probes = append(probes, platformRuntime.KafkaReadinessProbe("kafka", platformKafka.Client))
		}
		if err := platformRuntime.ConfigureDependencyReadiness(runtime.metrics, config.MetricsConfig(), probes...); err != nil {
			cleanup()
			return nil, fmt.Errorf("configure Core readiness: %w", err)
		}
		platformRuntime.BindRPCReadiness(runtime.metrics, runtime.coreRPC)
		platformRuntime.MarkReady(runtime.metrics)
	}
	logger.Info("standalone Core runtime initialized", zap.String("mode", gatewayMode), zap.String("rpc_addr", rpcAddress(runtime.coreRPC)))
	return runtime, nil
}

func validateStandaloneCoreMode(mode string) error {
	if mode != "remote" {
		return fmt.Errorf("standalone Core service requires gateway.mode=remote; use embedded runtime for local compatibility")
	}
	return nil
}

func (r *CoreRuntime) Server() *server.Server {
	if r == nil {
		return nil
	}
	return r.server
}

func (r *CoreRuntime) Close() {
	if r == nil {
		return
	}
	if r.messageSender != nil {
		if err := r.messageSender.Close(); err != nil {
			logger.Warn("Core Message sender close failed", zap.Error(err))
		}
		r.messageSender = nil
	}
	if err := platformRuntime.CloseMetrics(r.metrics); err != nil {
		logger.Warn("Core metrics close failed", zap.Error(err))
	}
	if r.coreRPC != nil {
		shutdownSeconds := config.InternalRPCConfig().ShutdownTimeoutSeconds
		if shutdownSeconds <= 0 {
			shutdownSeconds = 15
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(shutdownSeconds)*time.Second)
		r.coreRPC.Close(shutdownCtx)
		cancel()
		r.coreRPC = nil
	}
	if err := platformKafka.CloseConsumer(); err != nil {
		logger.Warn("Core Kafka consumer close failed", zap.Error(err))
	}
	if err := platformKafka.Close(); err != nil {
		logger.Warn("Core Kafka publisher close failed", zap.Error(err))
	}
}

func rpcAddress(rpc *InternalRPCServer) string {
	if rpc == nil {
		return "disabled"
	}
	return rpc.Address()
}
