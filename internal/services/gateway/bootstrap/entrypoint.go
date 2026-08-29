package bootstrap

import (
	"context"
	"time"

	legacybootstrap "github.com/JekYUlll/Dipole/internal/bootstrap"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/gateway"
)

// Runtime aliases the compatibility runtime while Gateway bootstrap is being
// moved behind its service boundary.
type Runtime = legacybootstrap.GatewayRuntime

// InitializeService keeps the Gateway entrypoint owned by the Gateway service
// while preserving realtime authority and rollback semantics.
func InitializeService(ctx context.Context) (*Runtime, error) {
	return legacybootstrap.InitializeGateway(ctx)
}

func RunServer(server *gateway.Server, tlsCfg config.TLS) error {
	return legacybootstrap.RunGatewayServer(server, tlsCfg)
}

func ShutdownTimeout() time.Duration {
	return legacybootstrap.GatewayShutdownTimeout()
}
