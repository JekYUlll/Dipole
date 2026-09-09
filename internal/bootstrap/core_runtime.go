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
	agentapplication "github.com/JekYUlll/Dipole/internal/services/agent/application"
	agentmysql "github.com/JekYUlll/Dipole/internal/services/agent/infrastructure/mysql"
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
	search        *lazyCoreSearchApplication
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
	if err := ensureAIAssistantUser(coreRepos.Users); err != nil {
		return nil, fmt.Errorf("ensure AI assistant user: %w", err)
	}
	agentRepos, err := agentmysql.NewProcessRepositories(platformmysql.SQLDB)
	if err != nil {
		return nil, fmt.Errorf("compose Agent repositories: %w", err)
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
	runtime.search = newLazyCoreSearchApplication(config.InternalRPCConfig())
	cleanup := func() { runtime.Close() }
	runtime.server = server.NewWithDependencies(processRepos, server.Dependencies{Messaging: messaging, SystemMessages: systemMessages})
	if err := RegisterCoreProjectionKafkaHandlers(messaging); err != nil {
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
		if runtime.messageSender == nil {
			cleanup()
			return nil, fmt.Errorf("Agent RPC requires core.message.transport=grpc")
		}
		permissions, scopes := applicationPort.EmbeddedAgentPolicyGrantV1()
		if err := agentapplication.EnsureEmbeddedAgentDefinitionV1(ctx, agentRepos.Policy, "dipole", config.AIConfig().AssistantUUID, permissions, scopes); err != nil {
			cleanup()
			return nil, fmt.Errorf("ensure embedded Agent Definition: %w", err)
		}
		commands, composeErr := agentapplication.NewLocalAgentCommandV1(runtime.messageSender)
		if composeErr != nil {
			cleanup()
			return nil, fmt.Errorf("compose Agent Command: %w", composeErr)
		}
		agentCapability, composeErr := agentapplication.NewLocalAgentCapabilityV1(messaging.Core, runtime.messageSender, messaging.Conversations, commands, runtime.search)
		if composeErr != nil {
			cleanup()
			return nil, fmt.Errorf("compose Agent Capability: %w", composeErr)
		}
		resolver, composeErr := agentapplication.NewPersistentAgentInvocationResolverV1(agentRepos.Policy)
		if composeErr != nil {
			cleanup()
			return nil, fmt.Errorf("compose Agent resolver: %w", composeErr)
		}
		admission, composeErr := agentapplication.NewPersistentAgentRunAdmissionV1(agentRepos.Policy)
		if composeErr != nil {
			cleanup()
			return nil, fmt.Errorf("compose Agent admission: %w", composeErr)
		}
		approvals, composeErr := agentapplication.NewPersistentAgentApprovalServiceV1(agentRepos.Policy)
		if composeErr != nil {
			cleanup()
			return nil, fmt.Errorf("compose Agent approvals: %w", composeErr)
		}
		controls, composeErr := agentapplication.NewPersistentAgentTaskControlAuthorizerV1(agentRepos.Policy)
		if composeErr != nil {
			cleanup()
			return nil, fmt.Errorf("compose Agent controls: %w", composeErr)
		}
		projection, composeErr := agentapplication.NewPersistentAgentTaskWorkflowProjectionServiceV1(agentRepos.Policy)
		if composeErr != nil {
			cleanup()
			return nil, fmt.Errorf("compose Agent projection: %w", composeErr)
		}
		repairs, composeErr := agentapplication.NewPersistentAgentWorkflowRepairAuditServiceV1(agentRepos.Policy, agentRepos.Repairs)
		if composeErr != nil {
			cleanup()
			return nil, fmt.Errorf("compose Agent repair audit: %w", composeErr)
		}
		toolAudits, composeErr := agentapplication.NewPersistentAgentToolInvocationAuditServiceV1(agentRepos.ToolAudits, resolver, agentRepos.Policy, runtime.messageSender)
		if composeErr != nil {
			cleanup()
			return nil, fmt.Errorf("compose Agent Tool audit: %w", composeErr)
		}
		messageCommands, composeErr := agentapplication.NewAgentMessageCommandExecutionV1(agentRepos.ToolAudits, resolver, commands)
		if composeErr != nil {
			cleanup()
			return nil, fmt.Errorf("compose Agent Message Command execution: %w", composeErr)
		}
		approvalGrants, composeErr := agentapplication.NewPersistentAgentApprovalGrantResolverV1(agentRepos.ApprovalGrants)
		if composeErr != nil {
			cleanup()
			return nil, fmt.Errorf("compose Agent approval grants: %w", composeErr)
		}
		runtime.coreRPC, err = NewCoreRPCServerWithAgentArtifacts(
			rpcCfg, messaging.Core, agentCapability, resolver, admission, approvals, controls, projection, repairs,
			nil, nil, nil, nil, toolAudits, nil, nil, messageCommands, approvalGrants,
			nil, nil, nil, nil, nil, nil, agentRepos.TaskTimeline,
		)
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
	if r.search != nil {
		if err := r.search.Close(); err != nil {
			logger.Warn("Core Search client close failed", zap.Error(err))
		}
		r.search = nil
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
