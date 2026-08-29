package bootstrap

import (
	"context"
	"os"

	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/logger"
	"github.com/JekYUlll/Dipole/internal/server"
	"go.uber.org/zap"
)

// Runtime is the standalone Core service runtime.
type Runtime = CoreRuntime

func InitializeService(ctx context.Context) (*Runtime, error) {
	return InitializeCoreService(ctx)
}

func RunServer(srv *server.Server, tlsCfg config.TLS) error {
	if !tlsCfg.Enabled {
		return srv.Run(config.Addr())
	}
	if err := ensureTLSFiles(tlsCfg); err != nil {
		return err
	}
	logger.Info("Core TLS enabled",
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
