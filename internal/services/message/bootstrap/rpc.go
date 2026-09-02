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

type InternalRPCServer = platformrpc.Server

// DialCoreCapability opens the Core capability channel with the Message
// service identity.
func DialCoreCapability(ctx context.Context, cfg config.InternalRPC) (*coregrpc.Client, *grpc.ClientConn, error) {
	connection, err := platformrpc.Dial(ctx, cfg, cfg.CoreTarget, grpcauth.Credentials{Service: "dipole-message", Secret: cfg.SharedSecret})
	if err != nil {
		return nil, nil, fmt.Errorf("dial core rpc: %w", err)
	}
	client, err := coregrpc.NewClientForService(corev1.NewCoreCapabilityServiceClient(connection), "dipole-message")
	if err != nil {
		_ = connection.Close()
		return nil, nil, fmt.Errorf("create core capability client: %w", err)
	}
	return client, connection, nil
}

func NewMessageRPCServer(cfg config.InternalRPC, messages application.MessageApplication) (*InternalRPCServer, error) {
	adapter, err := messagegrpc.NewServer(messages)
	if err != nil {
		return nil, err
	}
	return platformrpc.NewServer(cfg, cfg.MessageListenAddress, []string{"dipole-gateway", "dipole-core"}, func(server *grpc.Server) {
		messagev1.RegisterMessageServiceServer(server, adapter)
	})
}
