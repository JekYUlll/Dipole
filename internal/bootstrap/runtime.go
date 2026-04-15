package bootstrap

import (
	"context"
	"fmt"
	"os"

	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/logger"
	"github.com/JekYUlll/Dipole/internal/model"
	platformBloom "github.com/JekYUlll/Dipole/internal/platform/bloom"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	platformPresence "github.com/JekYUlll/Dipole/internal/platform/presence"
	platformStorage "github.com/JekYUlll/Dipole/internal/platform/storage"
	"github.com/JekYUlll/Dipole/internal/repository"
	"github.com/JekYUlll/Dipole/internal/server"
	"github.com/JekYUlll/Dipole/internal/store"
	wsTransport "github.com/JekYUlll/Dipole/internal/transport/ws"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type Runtime struct {
	server *server.Server
	router *wsTransport.PubSubRouter // nil 表示单节点模式（Kafka 或 Presence 未启用）
}

func Initialize(ctx context.Context) (*Runtime, error) {
	mysqlCfg := config.MySQLConfig()
	redisCfg := config.RedisConfig()
	kafkaCfg := config.KafkaConfig()
	storageCfg := config.StorageConfig()

	if err := store.InitMySQL(); err != nil {
		return nil, fmt.Errorf("mysql init failed: %w", err)
	}
	logger.Info("mysql init succeeded",
		zap.String("host", mysqlCfg.Host),
		zap.Int("port", mysqlCfg.Port),
		zap.String("dbname", mysqlCfg.DBName),
		zap.String("user", mysqlCfg.User),
	)

	if err := store.InitRedis(); err != nil {
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

	if err := store.AutoMigrate(); err != nil {
		return nil, fmt.Errorf("auto migrate failed: %w", err)
	}
	if err := ensureAIAssistantUser(); err != nil {
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

	srv := server.New()

	// 跨节点 WS 路由：仅在 Kafka + Presence 同时启用时激活。
	// 单节点部署时 router 为 nil，直接使用 hub 本地投递。
	rt := &Runtime{server: srv}
	var wsEventSender kafkaWSEventSender = srv.WSHub()
	if kafkaCfg.Enabled && config.PresenceConfig().Enabled && store.RDB != nil {
		// NewRedisPresence() 是无状态的，与 server.New() 内部实例共享同一 Redis 连接，无冲突。
		redisPresence := platformPresence.NewRedisPresence()
		router := wsTransport.NewPubSubRouter(srv.WSHub(), redisPresence, store.RDB)
		if router != nil {
			router.Start()
			rt.router = router
			wsEventSender = router
			logger.Info("ws pubsub router started",
				zap.String("node_id", redisPresence.NodeID()),
			)
		}
	}
	if err := RegisterKafkaHandlers(wsEventSender); err != nil {
		return nil, fmt.Errorf("register kafka handlers failed: %w", err)
	}
	if platformKafka.Subscriber != nil {
		if err := platformKafka.Subscriber.Start(ctx); err != nil {
			return nil, fmt.Errorf("kafka consumer start failed: %w", err)
		}
		logger.Info("kafka consumer started")
	}

	return rt, nil
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

func ensureAIAssistantUser() error {
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

	if err := repository.NewUserRepository().UpsertAssistant(assistant); err != nil {
		return err
	}

	logger.Info("ai assistant user ensured",
		zap.String("assistant_uuid", assistant.UUID),
		zap.String("provider", cfg.Provider),
		zap.String("model", cfg.Model),
	)
	return nil
}
