package bootstrap

import (
	"context"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/logger"
	platformRuntime "github.com/JekYUlll/Dipole/internal/platform/runtime"
	"github.com/JekYUlll/Dipole/internal/services/core/server"
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
	if err := platformRuntime.ValidateTLSFiles(tlsCfg); err != nil {
		return err
	}
	logger.Info("Core TLS enabled",
		zap.String("cert_file", tlsCfg.CertFile),
		zap.String("key_file", tlsCfg.KeyFile),
	)
	return srv.RunTLS(config.Addr(), tlsCfg.CertFile, tlsCfg.KeyFile)
}
