package bootstrap

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	appComposition "github.com/JekYUlll/Dipole/internal/app"
	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/data/migration"
	"github.com/JekYUlll/Dipole/internal/logger"
	"github.com/JekYUlll/Dipole/internal/model"
	platformBloom "github.com/JekYUlll/Dipole/internal/platform/bloom"
	"github.com/JekYUlll/Dipole/internal/platform/cache"
	platformHotGroup "github.com/JekYUlll/Dipole/internal/platform/hotgroup"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	platformmysql "github.com/JekYUlll/Dipole/internal/platform/mysql"
	platformObservability "github.com/JekYUlll/Dipole/internal/platform/observability"
	platformPresence "github.com/JekYUlll/Dipole/internal/platform/presence"
	platformStorage "github.com/JekYUlll/Dipole/internal/platform/storage"
	"github.com/JekYUlll/Dipole/internal/server"
	agentapplication "github.com/JekYUlll/Dipole/internal/services/agent/application"
	wsTransport "github.com/JekYUlll/Dipole/internal/transport/ws"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type Runtime struct {
	server      *server.Server
	router      *wsTransport.PubSubRouter // nil 表示单节点模式（Kafka 或 Presence 未启用）
	outboxFlow  *outboxRelay
	messageFlow *messageApplicationTransport
	syncFlow    *syncApplicationTransport
	coreRPC     *InternalRPCServer
	metrics     *platformObservability.MetricsServer
}

func Initialize(ctx context.Context) (*Runtime, error) {
	mysqlCfg := config.MySQLConfig()
	redisCfg := config.RedisConfig()
	kafkaCfg := config.KafkaConfig()
	gatewayCfg := config.GatewayConfig()
	storageCfg := config.StorageConfig()
	messageCfg := config.CoreMessageConfig()
	if err := validateTimelineNotifyMode(messageCfg); err != nil {
		return nil, err
	}
	if gatewayCfg.Mode != "embedded" && gatewayCfg.Mode != "remote" {
		return nil, fmt.Errorf("unsupported gateway.mode %q", gatewayCfg.Mode)
	}
	if err := platformmysql.InitMySQL(); err != nil {
		return nil, fmt.Errorf("mysql init failed: %w", err)
	}
	logger.Info("mysql init succeeded",
		zap.String("host", mysqlCfg.Host),
		zap.Int("port", mysqlCfg.Port),
		zap.String("dbname", mysqlCfg.DBName),
		zap.String("user", mysqlCfg.User),
	)

	if err := cache.InitRedis(); err != nil {
		return nil, fmt.Errorf("redis init failed: %w", err)
	}
	logger.Info("redis init succeeded",
		zap.String("host", redisCfg.Host),
		zap.Int("port", redisCfg.Port),
		zap.Int("db", redisCfg.DB),
	)

	if err := platformKafka.Init(); err != nil {
		return nil, fmt.Errorf("kafka init failed: %w", err)
	}
	if kafkaCfg.Enabled {
		logger.Info("kafka publisher init succeeded",
			zap.Strings("brokers", kafkaCfg.Brokers),
			zap.String("client_id", kafkaCfg.ClientID),
			zap.String("topic_prefix", kafkaCfg.TopicPrefix),
		)
	} else {
		logger.Info("kafka publisher is disabled")
	}

	if err := platformKafka.InitConsumer(); err != nil {
		return nil, fmt.Errorf("kafka consumer init failed: %w", err)
	}
	if kafkaCfg.Enabled {
		logger.Info("kafka consumer init succeeded",
			zap.Int("retry_max_attempts", kafkaCfg.ConsumeRetryMaxAttempts),
			zap.Int("retry_backoff_ms", kafkaCfg.ConsumeRetryBackoffMS),
		)
	} else {
		logger.Info("kafka consumer is disabled")
	}

	if err := platformStorage.Init(); err != nil {
		return nil, fmt.Errorf("storage init failed: %w", err)
	}
	if storageCfg.Enabled {
		logger.Info("storage init succeeded",
			zap.String("provider", storageCfg.Provider),
			zap.String("endpoint", storageCfg.Endpoint),
			zap.String("bucket", storageCfg.Bucket),
			zap.Int64("file_max_size_mb", storageCfg.FileMaxSizeMB),
		)
	} else {
		logger.Info("storage is disabled")
	}

	runner, err := migration.NewRunner(platformmysql.SQLDB, migrations.Files)
	if err != nil {
		return nil, fmt.Errorf("initialize migration validation: %w", err)
	}
	if err := runner.ValidateCurrent(ctx); err != nil {
		return nil, fmt.Errorf("database schema is not ready: %w", err)
	}
	repos, err := appComposition.NewRepositories(platformmysql.SQLDB)
	if err != nil {
		return nil, fmt.Errorf("compose repositories: %w", err)
	}
	if err := ensureAIAssistantUser(repos.Users); err != nil {
		return nil, fmt.Errorf("ensure ai assistant user failed: %w", err)
	}
	if err := platformBloom.Init(); err != nil {
		return nil, fmt.Errorf("bloom filter init failed: %w", err)
	}
	userCount, groupCount := platformBloom.Stats()
	logger.Info("bloom filter init succeeded",
		zap.Int("user_count", userCount),
		zap.Int("group_count", groupCount),
	)
	// 多节点部署时 bloom filter 是进程内状态，各节点独立维护，新注册用户只更新本节点内存。
	// Kafka 启用即视为分布式模式，禁用 bloom filter 拦截，直接走 DB 保证正确性。
	if kafkaCfg.Enabled {
		platformBloom.SetDistributed(true)
		logger.Info("bloom filter distributed mode enabled, local filter bypassed")
	}

	var messageEvents applicationPort.EventPublisher
	if kafkaCfg.Enabled {
		messageEvents = platformKafka.Client
	}
	localMessaging := appComposition.NewMessagingServices(repos, appComposition.MessagingDependencies{
		Events:    messageEvents,
		HotGroups: platformHotGroup.NewDetectorWithClient(config.HotGroupConfig(), cache.RDB),
		Storage:   platformStorage.Client,
	})
	conversationProjectionMetrics := platformObservability.NewConversationProjectionCollector()
	localMessaging.Conversations.SetProjectionWriteObserver(conversationProjectionMetrics.Observe)
	rpcCfg := config.InternalRPCConfig()
	var coreRPC *InternalRPCServer
	if rpcCfg.Enabled {
		permissions, scopes := applicationPort.EmbeddedAgentPolicyGrantV1()
		if err := appComposition.EnsureEmbeddedAgentDefinitionV1(ctx, repos.AgentPolicy, "dipole", config.AIConfig().AssistantUUID, permissions, scopes); err != nil {
			return nil, fmt.Errorf("ensure remote Agent Definition: %w", err)
		}
		agentCommands, composeErr := agentapplication.NewLocalAgentCommandV1(localMessaging.Messages)
		if composeErr != nil {
			return nil, fmt.Errorf("compose remote Agent Command: %w", composeErr)
		}
		agentCapability, composeErr := agentapplication.NewLocalAgentCapabilityV1(localMessaging.Core, localMessaging.Messages, localMessaging.Conversations, agentCommands)
		if composeErr != nil {
			return nil, fmt.Errorf("compose remote Agent Capability: %w", composeErr)
		}
		resolver, composeErr := appComposition.NewPersistentAgentInvocationResolverV1(repos.AgentPolicy)
		if composeErr != nil {
			return nil, fmt.Errorf("compose Agent Invocation resolver: %w", composeErr)
		}
		admission, composeErr := appComposition.NewPersistentAgentRunAdmissionV1(repos.AgentPolicy)
		if composeErr != nil {
			return nil, fmt.Errorf("compose Agent Run admission: %w", composeErr)
		}
		approvalService, composeErr := agentapplication.NewPersistentAgentApprovalServiceV1(repos.AgentPolicy)
		if composeErr != nil {
			return nil, fmt.Errorf("compose Agent Approval service: %w", composeErr)
		}
		approvalGrants, composeErr := agentapplication.NewPersistentAgentApprovalGrantResolverV1(repos.AgentApprovalGrants)
		if composeErr != nil {
			return nil, fmt.Errorf("compose Agent Approval grant resolver: %w", composeErr)
		}
		controlAuthorizer, composeErr := agentapplication.NewPersistentAgentTaskControlAuthorizerV1(repos.AgentPolicy)
		if composeErr != nil {
			return nil, fmt.Errorf("compose Agent Task control authorizer: %w", composeErr)
		}
		workflowProjection, composeErr := agentapplication.NewPersistentAgentTaskWorkflowProjectionServiceV1(repos.AgentPolicy)
		if composeErr != nil {
			return nil, fmt.Errorf("compose Agent Task Workflow projection: %w", composeErr)
		}
		workflowRepairAudit, composeErr := agentapplication.NewPersistentAgentWorkflowRepairAuditServiceV1(repos.AgentPolicy, repos.AgentRepairs)
		if composeErr != nil {
			return nil, fmt.Errorf("compose Agent Task Workflow repair audit: %w", composeErr)
		}
		promotionControls, composeErr := agentapplication.NewPersistentAgentRuntimePromotionControlServiceV1(repos.AgentPolicy, repos.AgentArtifacts, repos.AgentPromotionControls)
		if composeErr != nil {
			return nil, fmt.Errorf("compose Agent Runtime promotion control: %w", composeErr)
		}
		readinessEvidence, composeErr := agentapplication.NewPersistentAgentMCPReadinessEvidencePublisherV1(repos.AgentReadinessEvidence)
		if composeErr != nil {
			return nil, fmt.Errorf("compose Agent MCP readiness evidence Publisher: %w", composeErr)
		}
		readinessResolver, composeErr := agentapplication.NewPersistentAgentMCPReadinessEvidenceResolverV1(repos.AgentReadinessEvidence, time.Now)
		if composeErr != nil {
			return nil, fmt.Errorf("compose Agent MCP readiness evidence Resolver: %w", composeErr)
		}
		subscriptionResolver, composeErr := agentapplication.NewPersistentAgentEventSubscriptionResolverV1(repos.AgentSubscriptions, repos.AgentPolicy, time.Now)
		if composeErr != nil {
			return nil, fmt.Errorf("compose Agent Event Subscription resolver: %w", composeErr)
		}
		subscriptionControls, composeErr := agentapplication.NewPersistentAgentEventSubscriptionControlV1(repos.AgentSubscriptions, repos.AgentPolicy, localMessaging.Core, time.Now)
		if composeErr != nil {
			return nil, fmt.Errorf("compose Agent Event Subscription control: %w", composeErr)
		}
		definitionCatalog, composeErr := agentapplication.NewPersistentAgentDefinitionCatalogV1(repos.AgentDefinitionCatalog, time.Now)
		if composeErr != nil {
			return nil, fmt.Errorf("compose Agent Definition catalog: %w", composeErr)
		}
		memoryResolver, composeErr := agentapplication.NewPersistentAgentMemoryResolverV1(repos.AgentMemories, resolver, repos.AgentPolicy, time.Now)
		if composeErr != nil {
			return nil, fmt.Errorf("compose Agent Memory resolver: %w", composeErr)
		}
		memoryControls, composeErr := agentapplication.NewPersistentAgentMemoryOwnerControlV1(repos.AgentMemoryOwners, time.Now)
		if composeErr != nil {
			return nil, fmt.Errorf("compose Agent Memory owner control: %w", composeErr)
		}
		memoryPromotions, composeErr := agentapplication.NewPersistentAgentMemoryCandidatePromotionServiceV1(repos.AgentMemoryPromotions, time.Now)
		if composeErr != nil {
			return nil, fmt.Errorf("compose Agent Memory candidate promotion service: %w", composeErr)
		}
		toolAudits, composeErr := agentapplication.NewPersistentAgentToolInvocationAuditServiceV1(repos.AgentToolAudits, resolver, repos.AgentPolicy, localMessaging.Messages)
		if composeErr != nil {
			return nil, fmt.Errorf("compose Agent Tool invocation audit: %w", composeErr)
		}
		toolRounds, composeErr := agentapplication.NewPersistentAgentMCPToolRoundServiceV1(repos.AgentToolRounds, repos.AgentToolAudits)
		if composeErr != nil {
			return nil, fmt.Errorf("compose Agent MCP Tool round receipts: %w", composeErr)
		}
		toolTerminals, composeErr := agentapplication.NewPersistentAgentMCPToolInvocationTerminalServiceV1(repos.AgentToolRounds, repos.AgentToolAudits, toolAudits)
		if composeErr != nil {
			return nil, fmt.Errorf("compose Agent MCP Tool invocation terminal: %w", composeErr)
		}
		messageCommands, composeErr := agentapplication.NewAgentMessageCommandExecutionV1(repos.AgentToolAudits, resolver, agentCommands)
		if composeErr != nil {
			return nil, fmt.Errorf("compose Agent Message Command execution: %w", composeErr)
		}
		var artifactService applicationPort.AgentArtifactServiceV1
		var promotionEvidence applicationPort.AgentRuntimePromotionEvidenceReviewServiceV1
		if storageCfg.ArtifactEnabled {
			artifactBlobs, artifactErr := platformStorage.NewAgentArtifactBlobStoreFromConfig(ctx, platformStorage.AgentArtifactStorageConfigV1{
				Enabled: storageCfg.ArtifactEnabled, Endpoint: storageCfg.ArtifactEndpoint,
				AccessKey: storageCfg.ArtifactAccessKey, SecretKey: storageCfg.ArtifactSecretKey,
				UseSSL: storageCfg.ArtifactUseSSL, Bucket: storageCfg.ArtifactBucket,
				GeneralAccessKey: storageCfg.AccessKey, GeneralBucket: storageCfg.Bucket,
			})
			if artifactErr != nil {
				return nil, fmt.Errorf("compose Agent Artifact blob storage: %w", artifactErr)
			}
			persistentArtifacts, serviceErr := agentapplication.NewPersistentAgentArtifactServiceV1(repos.AgentPolicy, repos.AgentArtifacts, artifactBlobs)
			artifactErr = serviceErr
			if artifactErr != nil {
				return nil, fmt.Errorf("compose Agent Artifact service: %w", artifactErr)
			}
			artifactService = persistentArtifacts
			promotionEvidence, artifactErr = agentapplication.NewAgentRuntimePromotionEvidenceReviewServiceV1(promotionControls, persistentArtifacts)
			if artifactErr != nil {
				return nil, fmt.Errorf("compose Agent Runtime promotion evidence review: %w", artifactErr)
			}
		}
		coreRPC, err = NewCoreRPCServerWithAgentArtifacts(
			rpcCfg, localMessaging.Core, agentCapability, resolver, admission, approvalService, controlAuthorizer, workflowProjection, workflowRepairAudit, subscriptionResolver, subscriptionControls, definitionCatalog, artifactService, toolAudits, toolRounds, toolTerminals, messageCommands, approvalGrants, promotionControls, promotionEvidence, readinessEvidence, readinessResolver, memoryControls, memoryPromotions, repos.AgentTaskTimeline, memoryResolver,
		)
		if err != nil {
			return nil, fmt.Errorf("initialize core rpc server: %w", err)
		}
		logger.Info("core rpc server started", zap.String("addr", coreRPC.Address()))
	}
	messageFlow, err := newMessageApplicationTransport(ctx, messageCfg, rpcCfg, localMessaging.Messages)
	if err != nil {
		if coreRPC != nil {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			coreRPC.Close(shutdownCtx)
			cancel()
		}
		return nil, fmt.Errorf("initialize message transport: %w", err)
	}
	syncFlow, err := newSyncApplicationTransport(ctx, config.SyncConfig(), rpcCfg, localMessaging.Sync)
	if err != nil {
		messageFlow.Close()
		if coreRPC != nil {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			coreRPC.Close(shutdownCtx)
			cancel()
		}
		return nil, fmt.Errorf("initialize Sync transport: %w", err)
	}
	syncComparisonMetrics := platformObservability.NewClientSyncComparisonCollector()
	srv := server.NewWithDependencies(repos, server.Dependencies{
		Messages:       messageFlow.Application,
		Sync:           syncFlow.Application,
		SyncComparison: syncComparisonMetrics,
		Messaging:      localMessaging,
	})

	// 跨节点 WS 路由：仅在 Kafka + Presence 同时启用时激活。
	// 单节点部署时 router 为 nil，直接使用 hub 本地投递。
	rt := &Runtime{server: srv, messageFlow: messageFlow, syncFlow: syncFlow, coreRPC: coreRPC}
	var wsEventSender kafkaWSEventSender
	if gatewayCfg.Mode == "embedded" {
		wsEventSender = srv.WSHub()
	}
	if gatewayCfg.Mode == "embedded" && kafkaCfg.Enabled && config.PresenceConfig().Enabled && cache.RDB != nil {
		// NewRedisPresence() 是无状态的，与 server.New() 内部实例共享同一 Redis 连接，无冲突。
		redisPresence := platformPresence.NewRedisPresence()
		router := wsTransport.NewPubSubRouter(srv.WSHub(), redisPresence, cache.RDB)
		if router != nil {
			router.Start()
			rt.router = router
			wsEventSender = router
			logger.Info("ws pubsub router started",
				zap.String("node_id", redisPresence.NodeID()),
			)
		}
	}
	registerErr := registerCoreKafkaHandlers(
		wsEventSender,
		repos,
		localMessaging,
		coreOwnsMessagePersistence(gatewayCfg.Mode, messageCfg.Transport),
	)
	if registerErr != nil {
		return nil, fmt.Errorf("register kafka handlers failed: %w", registerErr)
	}
	if kafkaCfg.Enabled && platformKafka.Client != nil && coreOwnsMessagePersistence(gatewayCfg.Mode, messageCfg.Transport) {
		if err := platformKafka.Client.EnsureTopics(kafkaManagedTopics()); err != nil {
			return nil, fmt.Errorf("ensure kafka topics failed: %w", err)
		}
		logger.Info("kafka topics ensured",
			zap.Int("partitions", kafkaCfg.TopicPartitions),
			zap.Int("replication_factor", kafkaCfg.TopicReplicationFactor),
		)
	}
	if platformKafka.Subscriber != nil {
		if err := platformKafka.Subscriber.Start(ctx); err != nil {
			return nil, fmt.Errorf("kafka consumer start failed: %w", err)
		}
		logger.Info("kafka consumer started")
	}
	if kafkaCfg.Enabled && platformKafka.Client != nil {
		rt.outboxFlow = newOutboxRelay(repos.Outbox)
		if rt.outboxFlow != nil {
			rt.outboxFlow.Start()
			logger.Info("outbox relay started")
		}
	}
	rt.metrics, err = startRuntimeMetrics(
		config.MetricsConfig(),
		coreServiceName,
		platformKafka.Subscriber,
		syncComparisonMetrics,
		conversationProjectionMetrics,
	)
	if err != nil {
		rt.Close()
		return nil, fmt.Errorf("start runtime metrics: %w", err)
	}
	if rt.metrics != nil {
		if err := configureRuntimeDependencyReadiness(rt.metrics, config.MetricsConfig(), mysqlReadinessProbe("mysql", platformmysql.SQLDB)); err != nil {
			rt.Close()
			return nil, fmt.Errorf("configure Core dependency readiness: %w", err)
		}
		bindRPCReadiness(rt.metrics, rt.coreRPC)
		markRuntimeReady(rt.metrics)
	}

	return rt, nil
}

func coreOwnsMessagePersistence(gatewayMode, messageTransport string) bool {
	return gatewayMode == "embedded" && messageTransport != "grpc"
}

func validateTimelineNotifyMode(messageCfg config.Message) error {
	if messageCfg.TimelineNotifyMode != wsTransport.TimelineNotifyOff && messageCfg.TimelineNotifyMode != wsTransport.TimelineNotifyShadow && messageCfg.TimelineNotifyMode != wsTransport.TimelineNotifyPrimary {
		return fmt.Errorf("unsupported message.timeline_notify_mode %q", messageCfg.TimelineNotifyMode)
	}
	return nil
}

func (r *Runtime) Server() *server.Server {
	if r == nil {
		return nil
	}

	return r.server
}

func RunServer(srv *server.Server, tlsCfg config.TLS) error {
	if !tlsCfg.Enabled {
		return srv.Run(config.Addr())
	}

	if err := ensureTLSFiles(tlsCfg); err != nil {
		return err
	}

	logger.Info("tls enabled",
		zap.String("cert_file", tlsCfg.CertFile),
		zap.String("key_file", tlsCfg.KeyFile),
	)

	return srv.RunTLS(config.Addr(), tlsCfg.CertFile, tlsCfg.KeyFile)
}

func (r *Runtime) Close() {
	if err := closeRuntimeMetrics(r.metrics); err != nil {
		logger.Warn("metrics server close failed", zap.Error(err))
	}
	if r.coreRPC != nil {
		shutdownSeconds := config.InternalRPCConfig().ShutdownTimeoutSeconds
		if shutdownSeconds <= 0 {
			shutdownSeconds = 15
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(shutdownSeconds)*time.Second)
		r.coreRPC.Close(ctx)
		cancel()
	}
	if r.messageFlow != nil {
		r.messageFlow.Close()
	}
	if r.syncFlow != nil {
		r.syncFlow.Close()
	}
	if r.outboxFlow != nil {
		r.outboxFlow.Stop()
	}
	// Stop consumer first so in-flight retry/dead-letter publishes complete
	// before the publisher is torn down.
	if err := platformKafka.CloseConsumer(); err != nil {
		logger.Warn("kafka consumer close failed", zap.Error(err))
	}
	if err := platformKafka.Close(); err != nil {
		logger.Warn("kafka close failed", zap.Error(err))
	}
	// Stop the cross-node pubsub router after Kafka is shut down.
	if r.router != nil {
		r.router.Stop()
	}
}

func ensureTLSFiles(tlsCfg config.TLS) error {
	if _, err := os.Stat(tlsCfg.CertFile); err != nil {
		return err
	}
	if _, err := os.Stat(tlsCfg.KeyFile); err != nil {
		return err
	}

	return nil
}

type aiAssistantUserRepository interface {
	UpsertAssistant(user *model.User) error
}

func ensureAIAssistantUser(users aiAssistantUserRepository) error {
	cfg := config.AIConfig()
	if !cfg.Enabled {
		return nil
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte("dipole-ai-assistant"), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("generate ai assistant password hash: %w", err)
	}

	assistant := &model.User{
		UUID:         cfg.AssistantUUID,
		Nickname:     cfg.AssistantNickname,
		Telephone:    cfg.AssistantTelephone,
		Email:        cfg.AssistantEmail,
		Avatar:       cfg.AssistantAvatar,
		PasswordHash: string(passwordHash),
		IsAdmin:      false,
		UserType:     model.UserTypeAssistant,
		Status:       model.UserStatusNormal,
	}
	if assistant.Avatar == "" {
		assistant.Avatar = model.DefaultAvatarURL
	}

	if err := users.UpsertAssistant(assistant); err != nil {
		return err
	}

	logger.Info("ai assistant user ensured",
		zap.String("assistant_uuid", assistant.UUID),
		zap.String("provider", cfg.Provider),
		zap.String("model", cfg.Model),
	)
	return nil
}
