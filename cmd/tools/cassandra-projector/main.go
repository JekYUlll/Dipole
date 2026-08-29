package main

import (
	"context"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/JekYUlll/Dipole/internal/bootstrap"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/logger"
)

func main() {
	config.MustLoad()
	if err := logger.Init(); err != nil {
		panic(err)
	}
	defer func() { _ = logger.Sync() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	runtime, err := bootstrap.InitializeCassandraProjector(ctx)
	if err != nil {
		logger.L().Fatal("Cassandra projector initialize failed", zap.Error(err))
	}
	logger.L().Info("Cassandra projector started")
	<-ctx.Done()
	logger.L().Info("Cassandra projector shutdown requested")
	runtime.Close()
	logger.L().Info("Cassandra projector exited gracefully")
}
