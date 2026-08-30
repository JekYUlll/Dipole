package bootstrap

import (
	"context"
	"errors"
	"fmt"

	agentv1 "github.com/JekYUlll/Dipole/api/gen/go/agent/v1"
	messagev1 "github.com/JekYUlll/Dipole/api/gen/go/message/v1"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	platformrpc "github.com/JekYUlll/Dipole/internal/platform/rpc"
	corerpc "github.com/JekYUlll/Dipole/internal/services/core/rpc"
	grpcauth "github.com/JekYUlll/Dipole/internal/transport/grpc/auth"
	messagegrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/message"
	"google.golang.org/grpc"
)

const (
	coreServiceName = "dipole-core"
)

type InternalRPCServer = platformrpc.Server

// NewCoreRPCServer owns the standalone Core RPC adapter and its platform
// transport. The legacy bootstrap keeps a compatibility constructor for the
// embedded runtime and existing migration tests.
func NewCoreRPCServer(cfg config.InternalRPC, capability application.CoreCapability, adapters ...agentv1.AgentCapabilityServiceServer) (*platformrpc.Server, error) {
	if len(adapters) > 1 {
		return nil, errors.New("at most one Agent Capability RPC adapter may be configured")
	}
	var adapter agentv1.AgentCapabilityServiceServer
	if len(adapters) == 1 {
		adapter = adapters[0]
	}
	return corerpc.NewServer(cfg, capability, adapter)
}

func dialCoreMessageApplication(ctx context.Context, cfg config.InternalRPC) (*messagegrpc.Client, *grpc.ClientConn, error) {
	connection, err := platformrpc.Dial(ctx, cfg, cfg.MessageTarget, grpcauth.Credentials{
		Service: coreServiceName,
		Secret:  cfg.SharedSecret,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("dial message rpc: %w", err)
	}
	client, err := messagegrpc.NewClientForService(messagev1.NewMessageServiceClient(connection), coreServiceName)
	if err != nil {
		_ = connection.Close()
		return nil, nil, err
	}
	return client, connection, nil
}
