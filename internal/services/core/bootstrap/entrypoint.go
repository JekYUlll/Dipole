package bootstrap

import (
	"context"

	legacybootstrap "github.com/JekYUlll/Dipole/internal/bootstrap"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/server"
)

// EmbeddedRuntime aliases the compatibility aggregate runtime.
type EmbeddedRuntime = legacybootstrap.Runtime

// Runtime is the standalone Core service runtime.
type Runtime = CoreRuntime

func InitializeEmbedded(ctx context.Context) (*EmbeddedRuntime, error) {
	return legacybootstrap.Initialize(ctx)
}

func InitializeService(ctx context.Context) (*Runtime, error) {
	return InitializeCoreService(ctx)
}

func RunServer(srv *server.Server, tlsCfg config.TLS) error {
	return legacybootstrap.RunServer(srv, tlsCfg)
}
