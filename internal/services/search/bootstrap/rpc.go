package bootstrap

import (
	"context"
	"fmt"

	corev1 "github.com/JekYUlll/Dipole/api/gen/go/core/v1"
	searchv1 "github.com/JekYUlll/Dipole/api/gen/go/search/v1"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	platformrpc "github.com/JekYUlll/Dipole/internal/platform/rpc"
	grpcauth "github.com/JekYUlll/Dipole/internal/transport/grpc/auth"
	coregrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/core"
	searchgrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/search"
	"google.golang.org/grpc"
)

type InternalRPCServer = platformrpc.Server

func DialSearchCoreCapability(ctx context.Context, cfg config.InternalRPC) (*coregrpc.Client, *grpc.ClientConn, error) {
	connection, err := platformrpc.Dial(ctx, cfg, cfg.CoreTarget, grpcauth.Credentials{Service: searchServiceName, Secret: cfg.SharedSecret})
	if err != nil {
		return nil, nil, fmt.Errorf("dial core rpc: %w", err)
	}
	client, err := coregrpc.NewClientForService(corev1.NewCoreCapabilityServiceClient(connection), searchServiceName)
	if err != nil {
		_ = connection.Close()
		return nil, nil, fmt.Errorf("create core capability client: %w", err)
	}
	return client, connection, nil
}

func NewSearchRPCServer(cfg config.InternalRPC, search application.SearchApplication) (*InternalRPCServer, error) {
	adapter, err := searchgrpc.NewServer(search)
	if err != nil {
		return nil, fmt.Errorf("create Search rpc adapter: %w", err)
	}
	return platformrpc.NewServer(cfg, cfg.SearchListenAddress, []string{"dipole-gateway", "dipole-core"}, func(server *grpc.Server) {
		searchv1.RegisterSearchServiceServer(server, adapter)
	})
}

func DialSearchApplication(ctx context.Context, cfg config.InternalRPC) (*searchgrpc.Client, *grpc.ClientConn, error) {
	connection, err := platformrpc.Dial(ctx, cfg, cfg.SearchTarget, grpcauth.Credentials{Service: "dipole-gateway", Secret: cfg.SharedSecret})
	if err != nil {
		return nil, nil, fmt.Errorf("dial Search rpc: %w", err)
	}
	client, err := searchgrpc.NewClientForService(searchv1.NewSearchServiceClient(connection), "dipole-gateway")
	if err != nil {
		_ = connection.Close()
		return nil, nil, fmt.Errorf("create Search application client: %w", err)
	}
	return client, connection, nil
}
