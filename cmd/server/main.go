package main

import (
	"context"
	"os"

	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/logger"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	platformStorage "github.com/JekYUlll/Dipole/internal/platform/storage"
	"github.com/JekYUlll/Dipole/internal/server"
	"github.com/JekYUlll/Dipole/internal/store"
	"go.uber.org/zap"
)

func main() {
	config.MustLoad()
	if err := logger.Init(); err != nil {
		panic(err)
	}
	defer func() {
		_ = logger.Sync()
	}()

	appCfg := config.AppConfig()
	tlsCfg := config.TLSConfig()
	mysqlCfg := config.MySQLConfig()
	redisCfg := config.RedisConfig()
	kafkaCfg := config.KafkaConfig()
	logCfg := config.LogConfig()
	storageCfg := config.StorageConfig()

	if err := store.InitMySQL(); err != nil {
		logger.L().Fatal("mysql init failed", zap.Error(err))
	}
	logger.Info("mysql init succeeded",
		zap.String("host", mysqlCfg.Host),
		zap.Int("port", mysqlCfg.Port),
		zap.String("dbname", mysqlCfg.DBName),
		zap.String("user", mysqlCfg.User),
	)

	if err := store.InitRedis(); err != nil {
		logger.L().Fatal("redis init failed", zap.Error(err))
	}
	logger.Info("redis init succeeded",
		zap.String("host", redisCfg.Host),
		zap.Int("port", redisCfg.Port),
		zap.Int("db", redisCfg.DB),
	)

	defer func() {
		if err := platformKafka.Close(); err != nil {
			logger.Warn("kafka close failed", zap.Error(err))
		}
		if err := platformKafka.CloseConsumer(); err != nil {
			logger.Warn("kafka consumer close failed", zap.Error(err))
		}
	}()

	if err := platformKafka.Init(); err != nil {
		logger.L().Fatal("kafka init failed", zap.Error(err))
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
		logger.L().Fatal("kafka consumer init failed", zap.Error(err))
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
		logger.L().Fatal("storage init failed", zap.Error(err))
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
		logger.L().Fatal("auto migrate failed", zap.Error(err))
	}

	srv := server.New()
	srv.RegisterKafkaHandlers()
	if platformKafka.Subscriber != nil {
		if err := platformKafka.Subscriber.Start(context.Background()); err != nil {
			logger.L().Fatal("kafka consumer start failed", zap.Error(err))
		}
		logger.Info("kafka consumer started")
	}

	logger.Info("logger destination",
		zap.Bool("file_enabled", logCfg.FileEnabled),
		zap.String("file_path", logCfg.FilePath),
		zap.Bool("file_rotate_daily", logCfg.FileRotateDaily),
	)

	logger.L().Info("server starting",
		zap.String("app", appCfg.Name),
		zap.String("env", appCfg.Env),
		zap.String("addr", config.Addr()),
		zap.Bool("tls_enabled", tlsCfg.Enabled),
	)

	runErr := runServer(srv, tlsCfg)
	if runErr != nil {
		logger.L().Fatal("server run failed", zap.Error(runErr))
	}
}

func runServer(srv *server.Server, tlsCfg config.TLS) error {
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

func ensureTLSFiles(tlsCfg config.TLS) error {
	if _, err := os.Stat(tlsCfg.CertFile); err != nil {
		return err
	}
	if _, err := os.Stat(tlsCfg.KeyFile); err != nil {
		return err
	}

	return nil
}
