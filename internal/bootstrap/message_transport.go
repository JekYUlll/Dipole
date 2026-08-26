package bootstrap

import (
	"context"
	"fmt"
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	"google.golang.org/grpc"
)

type messageApplicationTransport struct {
	Application application.MessageApplication
	connection  *grpc.ClientConn
}

func newMessageApplicationTransport(ctx context.Context, cfg config.Message, rpcCfg config.InternalRPC, local application.MessageApplication) (*messageApplicationTransport, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Transport)) {
	case "", "local":
		if local == nil {
			return nil, fmt.Errorf("local message application is required")
		}
		return &messageApplicationTransport{Application: local}, nil
	case "grpc":
		if !rpcCfg.Enabled {
			return nil, fmt.Errorf("message grpc transport requires internal_rpc.enabled")
		}
		client, connection, err := DialMessageApplication(ctx, rpcCfg)
		if err != nil {
			return nil, err
		}
		return &messageApplicationTransport{Application: client, connection: connection}, nil
	default:
		return nil, fmt.Errorf("unsupported message.transport %q", cfg.Transport)
	}
}

func (t *messageApplicationTransport) Close() {
	if t == nil {
		return
	}
	if t.connection != nil {
		_ = t.connection.Close()
	}
}
