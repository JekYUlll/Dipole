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
	platformStorage "github.com/JekYUlll/Dipole/internal/platform/storage"
	"github.com/JekYUlll/Dipole/internal/repository"
	"github.com/JekYUlll/Dipole/internal/server"
	"github.com/JekYUlll/Dipole/internal/store"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type Runtime struct {
	server *server.Server
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

	srv := server.New()
	if err := RegisterKafkaHandlers(srv.WSHub()); err != nil {
		return nil, fmt.Errorf("register kafka handlers failed: %w", err)
	}
	if platformKafka.Subscriber != nil {
		if err := platformKafka.Subscriber.Start(ctx); err != nil {
			return nil, fmt.Errorf("kafka consumer start failed: %w", err)
		}
		logger.Info("kafka consumer started")
	}

	return &Runtime{server: srv}, nil
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
	if err := platformKafka.Close(); err != nil {
		logger.Warn("kafka close failed", zap.Error(err))
	}
	if err := platformKafka.CloseConsumer(); err != nil {
		logger.Warn("kafka consumer close failed", zap.Error(err))
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
