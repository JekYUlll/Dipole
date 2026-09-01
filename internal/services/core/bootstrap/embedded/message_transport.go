package embedded

import (
	"context"
	"fmt"
	"strings"

	messagev1 "github.com/JekYUlll/Dipole/api/gen/go/message/v1"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	platformrpc "github.com/JekYUlll/Dipole/internal/platform/rpc"
	grpcauth "github.com/JekYUlll/Dipole/internal/transport/grpc/auth"
	messagegrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/message"
	"google.golang.org/grpc"
)

type MessageApplicationTransport struct {
	Application application.MessageApplication
	connection  *grpc.ClientConn
	shadow      *messageShadowApplication
}

func NewMessageApplicationTransport(ctx context.Context, cfg config.Message, rpcCfg config.InternalRPC, local application.MessageApplication) (*MessageApplicationTransport, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Transport)) {
	case "", "local":
		if local == nil {
			return nil, fmt.Errorf("local message application is required")
		}
		if !cfg.ShadowQueries {
			return &MessageApplicationTransport{Application: local}, nil
		}
		if !rpcCfg.Enabled {
			return nil, fmt.Errorf("message shadow queries require internal_rpc.enabled")
		}
		remote, connection, err := DialCoreMessageApplication(ctx, rpcCfg)
		if err != nil {
			return nil, err
		}
		shadow := newMessageShadowApplication(local, remote, nil)
		return &MessageApplicationTransport{Application: shadow, connection: connection, shadow: shadow}, nil
	case "grpc":
		if !rpcCfg.Enabled {
			return nil, fmt.Errorf("message grpc transport requires internal_rpc.enabled")
		}
		client, connection, err := DialCoreMessageApplication(ctx, rpcCfg)
		if err != nil {
			return nil, err
		}
		if !cfg.ShadowQueries {
			return &MessageApplicationTransport{Application: client, connection: connection}, nil
		}
		if local == nil {
			_ = connection.Close()
			return nil, fmt.Errorf("message grpc shadow queries require local application")
		}
		shadow := newMessageShadowApplication(client, local, nil)
		return &MessageApplicationTransport{Application: shadow, connection: connection, shadow: shadow}, nil
	default:
		return nil, fmt.Errorf("unsupported message.transport %q", cfg.Transport)
	}
}

// DialMessageApplication opens the embedded Gateway-compatible Message RPC.
func DialMessageApplication(ctx context.Context, cfg config.InternalRPC) (*messagegrpc.Client, *grpc.ClientConn, error) {
	return dialMessageApplicationAs(ctx, cfg, "dipole-gateway")
}

// DialCoreMessageApplication opens the embedded Core-compatible Message RPC.
func DialCoreMessageApplication(ctx context.Context, cfg config.InternalRPC) (*messagegrpc.Client, *grpc.ClientConn, error) {
	return dialMessageApplicationAs(ctx, cfg, "dipole-core")
}

func dialMessageApplicationAs(ctx context.Context, cfg config.InternalRPC, callerService string) (*messagegrpc.Client, *grpc.ClientConn, error) {
	connection, err := platformrpc.Dial(ctx, cfg, cfg.MessageTarget, grpcauth.Credentials{Service: callerService, Secret: cfg.SharedSecret})
	if err != nil {
		return nil, nil, fmt.Errorf("dial message rpc: %w", err)
	}
	client, err := messagegrpc.NewClientForService(messagev1.NewMessageServiceClient(connection), callerService)
	if err != nil {
		_ = connection.Close()
		return nil, nil, fmt.Errorf("create message application client: %w", err)
	}
	return client, connection, nil
}

func (t *MessageApplicationTransport) Close() {
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
