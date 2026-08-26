package bootstrap

import (
	"context"
	"fmt"
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	"google.golang.org/grpc"
)

type syncApplicationTransport struct {
	Application application.SyncApplication
	connection  *grpc.ClientConn
	shadow      *syncShadowApplication
}

func newSyncApplicationTransport(ctx context.Context, cfg config.Sync, rpcCfg config.InternalRPC, local application.SyncApplication) (*syncApplicationTransport, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Transport)) {
	case "", "local":
		if local == nil {
			return nil, fmt.Errorf("local Sync application is required")
		}
		if !cfg.ShadowQueries {
			return &syncApplicationTransport{Application: local}, nil
		}
		if !rpcCfg.Enabled {
			return nil, fmt.Errorf("Sync shadow queries require internal_rpc.enabled")
		}
		remote, connection, err := DialCoreSyncApplication(ctx, rpcCfg)
		if err != nil {
			return nil, err
		}
		shadow := newSyncShadowApplication(local, remote, nil)
		return &syncApplicationTransport{Application: shadow, connection: connection, shadow: shadow}, nil
	case "grpc":
		if !rpcCfg.Enabled {
			return nil, fmt.Errorf("Sync grpc transport requires internal_rpc.enabled")
		}
		client, connection, err := DialCoreSyncApplication(ctx, rpcCfg)
		if err != nil {
			return nil, err
		}
		if !cfg.ShadowQueries {
			return &syncApplicationTransport{Application: client, connection: connection}, nil
		}
		if local == nil {
			_ = connection.Close()
			return nil, fmt.Errorf("Sync grpc shadow queries require local application")
		}
		shadow := newSyncShadowApplication(client, local, nil)
		return &syncApplicationTransport{Application: shadow, connection: connection, shadow: shadow}, nil
	default:
		return nil, fmt.Errorf("unsupported sync.transport %q", cfg.Transport)
	}
}

func (t *syncApplicationTransport) Close() {
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
