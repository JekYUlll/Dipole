package main

import (
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/logger"
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

	if err := store.InitMySQL(); err != nil {
		logger.L().Fatal("mysql init failed", zap.Error(err))
	}

	if err := store.InitRedis(); err != nil {
		logger.L().Fatal("redis init failed", zap.Error(err))
	}

	if err := store.AutoMigrate(); err != nil {
		logger.L().Fatal("auto migrate failed", zap.Error(err))
	}

	srv := server.New()

	logger.L().Info("server starting",
		zap.String("app", appCfg.Name),
		zap.String("env", appCfg.Env),
		zap.String("addr", config.Addr()),
	)

	if err := srv.Run(config.Addr()); err != nil {
		logger.L().Fatal("server run failed", zap.Error(err))
	}
}
