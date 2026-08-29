package bootstrap

import (
	"context"
	"fmt"

	corev1 "github.com/JekYUlll/Dipole/api/gen/go/core/v1"
	deliveryv1 "github.com/JekYUlll/Dipole/api/gen/go/delivery/v1"
	messagev1 "github.com/JekYUlll/Dipole/api/gen/go/message/v1"
	searchv1 "github.com/JekYUlll/Dipole/api/gen/go/search/v1"
	syncv1 "github.com/JekYUlll/Dipole/api/gen/go/sync/v1"
	"github.com/JekYUlll/Dipole/internal/config"
	platformrpc "github.com/JekYUlll/Dipole/internal/platform/rpc"
	grpcauth "github.com/JekYUlll/Dipole/internal/transport/grpc/auth"
	coregrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/core"
	deliverygrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/delivery"
	messagegrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/message"
	searchgrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/search"
	syncgrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/sync"
	"google.golang.org/grpc"
)

type InternalRPCServer = platformrpc.Server

func NewDeliveryObservationRPCServer(cfg config.InternalRPC, adapter *deliverygrpc.ShadowServer) (*InternalRPCServer, error) {
	if adapter == nil {
		return nil, fmt.Errorf("delivery observation rpc adapter is required")
	}
	return platformrpc.NewServer(cfg, cfg.DeliveryObservationListenAddress, []string{"dipole-realtime"}, func(server *grpc.Server) {
		deliveryv1.RegisterNodeDeliveryServiceServer(server, adapter)
	})
}

func DialMessageApplication(ctx context.Context, cfg config.InternalRPC) (*messagegrpc.Client, *grpc.ClientConn, error) {
	connection, err := platformrpc.Dial(ctx, cfg, cfg.MessageTarget, grpcauth.Credentials{Service: gatewayServiceName, Secret: cfg.SharedSecret})
	if err != nil {
		return nil, nil, fmt.Errorf("dial message rpc: %w", err)
	}
	client, err := messagegrpc.NewClientForService(messagev1.NewMessageServiceClient(connection), gatewayServiceName)
	if err != nil {
		_ = connection.Close()
		return nil, nil, fmt.Errorf("create message application client: %w", err)
	}
	return client, connection, nil
}

func DialSyncApplication(ctx context.Context, cfg config.InternalRPC) (*syncgrpc.Client, *grpc.ClientConn, error) {
	connection, err := platformrpc.Dial(ctx, cfg, cfg.SyncTarget, grpcauth.Credentials{Service: gatewayServiceName, Secret: cfg.SharedSecret})
	if err != nil {
		return nil, nil, fmt.Errorf("dial sync rpc: %w", err)
	}
	client, err := syncgrpc.NewClientForService(syncv1.NewSyncQueryServiceClient(connection), gatewayServiceName)
	if err != nil {
		_ = connection.Close()
		return nil, nil, fmt.Errorf("create sync application client: %w", err)
	}
	return client, connection, nil
}

func DialGatewayCoreCapability(ctx context.Context, cfg config.InternalRPC) (*coregrpc.Client, *grpc.ClientConn, error) {
	connection, err := platformrpc.Dial(ctx, cfg, cfg.CoreTarget, grpcauth.Credentials{Service: gatewayServiceName, Secret: cfg.SharedSecret})
	if err != nil {
		return nil, nil, fmt.Errorf("dial core rpc: %w", err)
	}
	client, err := coregrpc.NewClientForService(corev1.NewCoreCapabilityServiceClient(connection), gatewayServiceName)
	if err != nil {
		_ = connection.Close()
		return nil, nil, fmt.Errorf("create core capability client: %w", err)
	}
	return client, connection, nil
}

func DialSearchApplication(ctx context.Context, cfg config.InternalRPC) (*searchgrpc.Client, *grpc.ClientConn, error) {
	connection, err := platformrpc.Dial(ctx, cfg, cfg.SearchTarget, grpcauth.Credentials{Service: gatewayServiceName, Secret: cfg.SharedSecret})
	if err != nil {
		return nil, nil, fmt.Errorf("dial Search rpc: %w", err)
	}
	client, err := searchgrpc.NewClientForService(searchv1.NewSearchServiceClient(connection), gatewayServiceName)
	if err != nil {
		_ = connection.Close()
		return nil, nil, fmt.Errorf("create Search application client: %w", err)
	}
	return client, connection, nil
}
