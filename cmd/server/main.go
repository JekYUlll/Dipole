package main

import (
	"context"

	"github.com/JekYUlll/Dipole/internal/bootstrap"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/logger"
	"go.uber.org/zap"
)

// @title Dipole API
// @version 1.0
// @description Dipole 的 HTTP API 文档，覆盖认证、用户、联系人、会话、消息、群组、文件、设备会话和后台总览等核心接口。
// @BasePath /api/v1
// @schemes http https
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description 输入格式为 `Bearer <token>`。
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
	logCfg := config.LogConfig()
	runtime, err := bootstrap.Initialize(context.Background())
	if err != nil {
		logger.L().Fatal("bootstrap initialize failed", zap.Error(err))
	}
	defer runtime.Close()

	srv := runtime.Server()

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

	runErr := bootstrap.RunServer(srv, tlsCfg)
	if runErr != nil {
		logger.L().Fatal("server run failed", zap.Error(runErr))
	}
}
