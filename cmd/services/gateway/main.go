package main

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/JekYUlll/Dipole/internal/bootstrap"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/logger"
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
	runtime, err := bootstrap.InitializeGateway(ctx)
	if err != nil {
		logger.L().Fatal("gateway initialize failed", zap.Error(err))
	}
	defer runtime.Close()

	srv := runtime.Server()
	serverErr := make(chan error, 1)
	go func() { serverErr <- bootstrap.RunGatewayServer(srv, config.TLSConfig()) }()
	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.L().Fatal("gateway run failed", zap.Error(err))
		}
		return
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), bootstrap.GatewayShutdownTimeout())
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.L().Fatal("gateway graceful shutdown failed", zap.Error(err))
	}
	if err := <-serverErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.L().Fatal("gateway stopped with unexpected error", zap.Error(err))
	}
}
