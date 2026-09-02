package main

import (
	"context"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/logger"
	indexerbootstrap "github.com/JekYUlll/Dipole/internal/services/search-indexer/bootstrap"
)

func main() {
	config.MustLoad()
	if err := logger.Init(); err != nil {
		panic(err)
	}
	defer func() { _ = logger.Sync() }()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	runtime, err := indexerbootstrap.InitializeService(ctx)
	if err != nil {
		logger.L().Fatal("Search Indexer initialize failed", zap.Error(err))
	}
	logger.L().Info("Search Indexer started")
	<-ctx.Done()
	logger.L().Info("Search Indexer shutdown requested")
	runtime.Close()
	logger.L().Info("Search Indexer exited gracefully")
}
