package bootstrap

import (
	"context"
	"fmt"

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
func NewCoreRPCServer(cfg config.InternalRPC, capability application.CoreCapability) (*platformrpc.Server, error) {
	return corerpc.NewServer(cfg, capability, nil)
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
