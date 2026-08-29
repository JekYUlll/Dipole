package bootstrap

import (
	"context"
	"fmt"

	corev1 "github.com/JekYUlll/Dipole/api/gen/go/core/v1"
	syncv1 "github.com/JekYUlll/Dipole/api/gen/go/sync/v1"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	platformrpc "github.com/JekYUlll/Dipole/internal/platform/rpc"
	grpcauth "github.com/JekYUlll/Dipole/internal/transport/grpc/auth"
	coregrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/core"
	syncgrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/sync"
	"google.golang.org/grpc"
)

type InternalRPCServer = platformrpc.Server

func DialSyncCoreCapability(ctx context.Context, cfg config.InternalRPC) (*coregrpc.Client, *grpc.ClientConn, error) {
	connection, err := platformrpc.Dial(ctx, cfg, cfg.CoreTarget, grpcauth.Credentials{Service: syncServiceName, Secret: cfg.SharedSecret})
	if err != nil {
		return nil, nil, fmt.Errorf("dial core rpc: %w", err)
	}
	client, err := coregrpc.NewClientForService(corev1.NewCoreCapabilityServiceClient(connection), syncServiceName)
	if err != nil {
		_ = connection.Close()
		return nil, nil, fmt.Errorf("create core capability client: %w", err)
	}
	return client, connection, nil
}

func NewSyncRPCServer(cfg config.InternalRPC, syncApp application.SyncApplication) (*InternalRPCServer, error) {
	adapter, err := syncgrpc.NewServer(syncApp)
	if err != nil {
		return nil, fmt.Errorf("create Sync rpc adapter: %w", err)
	}
	return platformrpc.NewServer(cfg, cfg.SyncListenAddress, []string{"dipole-gateway", "dipole-core"}, func(server *grpc.Server) {
		syncv1.RegisterSyncQueryServiceServer(server, adapter)
	})
}
