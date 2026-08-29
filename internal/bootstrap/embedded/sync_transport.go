package embedded

import (
	"context"
	"fmt"
	"strings"

	syncv1 "github.com/JekYUlll/Dipole/api/gen/go/sync/v1"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	platformrpc "github.com/JekYUlll/Dipole/internal/platform/rpc"
	grpcauth "github.com/JekYUlll/Dipole/internal/transport/grpc/auth"
	syncgrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/sync"
	"google.golang.org/grpc"
)

type SyncApplicationTransport struct {
	Application application.SyncApplication
	connection  *grpc.ClientConn
	shadow      *syncShadowApplication
}

func NewSyncApplicationTransport(ctx context.Context, cfg config.Sync, rpcCfg config.InternalRPC, local application.SyncApplication) (*SyncApplicationTransport, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Transport)) {
	case "", "local":
		if local == nil {
			return nil, fmt.Errorf("local Sync application is required")
		}
		if !cfg.ShadowQueries {
			return &SyncApplicationTransport{Application: local}, nil
		}
		if !rpcCfg.Enabled {
			return nil, fmt.Errorf("Sync shadow queries require internal_rpc.enabled")
		}
		remote, connection, err := DialCoreSyncApplication(ctx, rpcCfg)
		if err != nil {
			return nil, err
		}
		shadow := newSyncShadowApplication(local, remote, nil)
		return &SyncApplicationTransport{Application: shadow, connection: connection, shadow: shadow}, nil
	case "grpc":
		if !rpcCfg.Enabled {
			return nil, fmt.Errorf("Sync grpc transport requires internal_rpc.enabled")
		}
		client, connection, err := DialCoreSyncApplication(ctx, rpcCfg)
		if err != nil {
			return nil, err
		}
		if !cfg.ShadowQueries {
			return &SyncApplicationTransport{Application: client, connection: connection}, nil
		}
		if local == nil {
			_ = connection.Close()
			return nil, fmt.Errorf("Sync grpc shadow queries require local application")
		}
		shadow := newSyncShadowApplication(client, local, nil)
		return &SyncApplicationTransport{Application: shadow, connection: connection, shadow: shadow}, nil
	default:
		return nil, fmt.Errorf("unsupported sync.transport %q", cfg.Transport)
	}
}

func (t *SyncApplicationTransport) Close() {
	if t == nil {
		return
	}
	if t.shadow != nil {
		t.shadow.Wait()
	}
	if t.connection != nil {
		_ = t.connection.Close()
		t.connection = nil
	}
}

// DialSyncApplication opens the embedded Gateway-compatible Sync RPC.
func DialSyncApplication(ctx context.Context, cfg config.InternalRPC) (*syncgrpc.Client, *grpc.ClientConn, error) {
	return dialSyncApplicationAs(ctx, cfg, "dipole-gateway")
}

// DialCoreSyncApplication opens the embedded Core-compatible Sync RPC.
func DialCoreSyncApplication(ctx context.Context, cfg config.InternalRPC) (*syncgrpc.Client, *grpc.ClientConn, error) {
	return dialSyncApplicationAs(ctx, cfg, "dipole-core")
}

func dialSyncApplicationAs(ctx context.Context, cfg config.InternalRPC, callerService string) (*syncgrpc.Client, *grpc.ClientConn, error) {
	connection, err := platformrpc.Dial(ctx, cfg, cfg.SyncTarget, grpcauth.Credentials{Service: callerService, Secret: cfg.SharedSecret})
	if err != nil {
		return nil, nil, fmt.Errorf("dial Sync rpc: %w", err)
	}
	client, err := syncgrpc.NewClientForService(syncv1.NewSyncQueryServiceClient(connection), callerService)
	if err != nil {
		_ = connection.Close()
		return nil, nil, fmt.Errorf("create Sync application client: %w", err)
	}
	return client, connection, nil
}
