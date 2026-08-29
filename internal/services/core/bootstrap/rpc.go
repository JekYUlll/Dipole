package bootstrap

import (
	"context"
	"fmt"

	corev1 "github.com/JekYUlll/Dipole/api/gen/go/core/v1"
	messagev1 "github.com/JekYUlll/Dipole/api/gen/go/message/v1"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	platformrpc "github.com/JekYUlll/Dipole/internal/platform/rpc"
	grpcauth "github.com/JekYUlll/Dipole/internal/transport/grpc/auth"
	coregrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/core"
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
func NewCoreRPCServer(cfg config.InternalRPC, capability application.CoreCapability) (*platformrpc.Server, error) {
	adapter, err := coregrpc.NewServer(capability)
	if err != nil {
		return nil, fmt.Errorf("create core rpc adapter: %w", err)
	}
	return platformrpc.NewServer(
		cfg,
		cfg.CoreListenAddress,
		[]string{"dipole-message", "dipole-gateway", "dipole-search", "dipole-sync"},
		func(server *grpc.Server) {
			corev1.RegisterCoreCapabilityServiceServer(server, adapter)
		},
		coregrpc.RestrictServiceMethods,
	)
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
