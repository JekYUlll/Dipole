package bootstrap

import (
	"context"
	"time"

	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/gateway"
)

type Runtime = GatewayRuntime

// InitializeService keeps the Gateway entrypoint owned by the Gateway service
// while preserving realtime authority and rollback semantics.
func InitializeService(ctx context.Context) (*Runtime, error) {
	return Initialize(ctx)
}

func RunServer(server *gateway.Server, tlsCfg config.TLS) error {
	return RunGatewayServer(server, tlsCfg)
}

func ShutdownTimeout() time.Duration {
	return GatewayShutdownTimeout()
}
