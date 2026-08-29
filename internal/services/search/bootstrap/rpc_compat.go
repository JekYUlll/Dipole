package bootstrap

import (
	"context"

	"github.com/JekYUlll/Dipole/internal/application"
	legacybootstrap "github.com/JekYUlll/Dipole/internal/bootstrap"
	"github.com/JekYUlll/Dipole/internal/config"
	coregrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/core"
	searchgrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/search"
	"google.golang.org/grpc"
)

// RPC helpers remain a narrow compatibility seam until shared internal RPC
// transport is extracted into a platform package.
type InternalRPCServer = legacybootstrap.InternalRPCServer

func NewCoreRPCServer(cfg config.InternalRPC, capability application.CoreCapability) (*InternalRPCServer, error) {
	return legacybootstrap.NewCoreRPCServer(cfg, capability)
}

func DialSearchCoreCapability(ctx context.Context, cfg config.InternalRPC) (*coregrpc.Client, *grpc.ClientConn, error) {
	return legacybootstrap.DialSearchCoreCapability(ctx, cfg)
}

func NewSearchRPCServer(cfg config.InternalRPC, search application.SearchApplication) (*InternalRPCServer, error) {
	return legacybootstrap.NewSearchRPCServer(cfg, search)
}

func DialSearchApplication(ctx context.Context, cfg config.InternalRPC) (*searchgrpc.Client, *grpc.ClientConn, error) {
	return legacybootstrap.DialSearchApplication(ctx, cfg)
}
