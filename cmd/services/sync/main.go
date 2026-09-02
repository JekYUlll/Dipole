package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/logger"
	syncbootstrap "github.com/JekYUlll/Dipole/internal/services/sync/bootstrap"
	"go.uber.org/zap"
)

func main() {
	config.MustLoad()
	if err := logger.Init(); err != nil {
		panic(err)
	}
	defer func() { _ = logger.Sync() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	runtime, err := syncbootstrap.InitializeService(ctx)
	if err != nil {
		logger.L().Fatal("Sync Service initialize failed", zap.Error(err))
	}
	logger.L().Info("Sync Service started", zap.String("addr", runtime.Address()))
	<-ctx.Done()
	logger.L().Info("Sync Service shutdown requested")
	runtime.Close()
	logger.L().Info("Sync Service exited gracefully")
}
