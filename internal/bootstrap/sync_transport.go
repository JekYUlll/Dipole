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
}

func newSyncApplicationTransport(ctx context.Context, cfg config.Sync, rpcCfg config.InternalRPC, local application.SyncApplication) (*syncApplicationTransport, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Transport)) {
	case "", "local":
		if local == nil {
			return nil, fmt.Errorf("local Sync application is required")
		}
		return &syncApplicationTransport{Application: local}, nil
	case "grpc":
		if !rpcCfg.Enabled {
			return nil, fmt.Errorf("Sync grpc transport requires internal_rpc.enabled")
		}
		client, connection, err := DialCoreSyncApplication(ctx, rpcCfg)
		if err != nil {
			return nil, err
		}
		return &syncApplicationTransport{Application: client, connection: connection}, nil
	default:
		return nil, fmt.Errorf("unsupported sync.transport %q", cfg.Transport)
	}
}

func (t *syncApplicationTransport) Close() {
	if t != nil && t.connection != nil {
		_ = t.connection.Close()
		t.connection = nil
	}
}
