package main

import (
	"os"

	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/logger"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
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

	if err := store.InitMySQL(); err != nil {
		logger.L().Fatal("mysql init failed", zap.Error(err))
	}

	if err := store.InitRedis(); err != nil {
		logger.L().Fatal("redis init failed", zap.Error(err))
	}

	defer func() {
		if err := platformKafka.Close(); err != nil {
			logger.Warn("kafka close failed", zap.Error(err))
		}
	}()

	if err := platformKafka.Init(); err != nil {
		logger.L().Fatal("kafka init failed", zap.Error(err))
	}

	if err := store.AutoMigrate(); err != nil {
		logger.L().Fatal("auto migrate failed", zap.Error(err))
	}

	srv := server.New()

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
