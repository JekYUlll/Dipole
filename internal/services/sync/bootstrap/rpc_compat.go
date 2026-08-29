package bootstrap

import (
	"context"

	"github.com/JekYUlll/Dipole/internal/application"
	legacybootstrap "github.com/JekYUlll/Dipole/internal/bootstrap"
	"github.com/JekYUlll/Dipole/internal/config"
	coregrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/core"
	"google.golang.org/grpc"
)

// RPC helpers remain a narrow compatibility seam until shared internal RPC
// transport is extracted into a platform package.
type InternalRPCServer = legacybootstrap.InternalRPCServer

func DialSyncCoreCapability(ctx context.Context, cfg config.InternalRPC) (*coregrpc.Client, *grpc.ClientConn, error) {
	return legacybootstrap.DialSyncCoreCapability(ctx, cfg)
}

func NewSyncRPCServer(cfg config.InternalRPC, syncApp application.SyncApplication) (*InternalRPCServer, error) {
	return legacybootstrap.NewSyncRPCServer(cfg, syncApp)
}
