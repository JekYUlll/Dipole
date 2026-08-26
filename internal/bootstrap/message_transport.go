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
	shadow      *messageShadowApplication
}

func newMessageApplicationTransport(ctx context.Context, cfg config.Message, rpcCfg config.InternalRPC, local application.MessageApplication) (*messageApplicationTransport, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Transport)) {
	case "", "local":
		if local == nil {
			return nil, fmt.Errorf("local message application is required")
		}
		if !cfg.ShadowQueries {
			return &messageApplicationTransport{Application: local}, nil
		}
		if !rpcCfg.Enabled {
			return nil, fmt.Errorf("message shadow queries require internal_rpc.enabled")
		}
		remote, connection, err := DialMessageApplication(ctx, rpcCfg)
		if err != nil {
			return nil, err
		}
		shadow := newMessageShadowApplication(local, remote, nil)
		return &messageApplicationTransport{Application: shadow, connection: connection, shadow: shadow}, nil
	case "grpc":
		if !rpcCfg.Enabled {
			return nil, fmt.Errorf("message grpc transport requires internal_rpc.enabled")
		}
		client, connection, err := DialMessageApplication(ctx, rpcCfg)
		if err != nil {
			return nil, err
		}
		if !cfg.ShadowQueries {
			return &messageApplicationTransport{Application: client, connection: connection}, nil
		}
		if local == nil {
			_ = connection.Close()
			return nil, fmt.Errorf("message grpc shadow queries require local application")
		}
		shadow := newMessageShadowApplication(client, local, nil)
		return &messageApplicationTransport{Application: shadow, connection: connection, shadow: shadow}, nil
	default:
		return nil, fmt.Errorf("unsupported message.transport %q", cfg.Transport)
	}
}

func (t *messageApplicationTransport) Close() {
	if t == nil {
		return
	}
	if t.shadow != nil {
		t.shadow.Wait()
	}
	if t.connection != nil {
		_ = t.connection.Close()
	}
}
