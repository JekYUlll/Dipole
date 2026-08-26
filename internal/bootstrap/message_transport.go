package bootstrap

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	messagev1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/message/v1"
	messagegrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/message"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type messageApplicationTransport struct {
	Application application.MessageApplication
	server      *grpc.Server
	connection  *grpc.ClientConn
	listener    *bufconn.Listener
}

func newMessageApplicationTransport(cfg config.Message, local application.MessageApplication) (*messageApplicationTransport, error) {
	if local == nil {
		return nil, fmt.Errorf("local message application is required")
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Transport)) {
	case "", "local":
		return &messageApplicationTransport{Application: local}, nil
	case "grpc":
		return newInProcessMessageTransport(local)
	default:
		return nil, fmt.Errorf("unsupported message.transport %q", cfg.Transport)
	}
}

func newInProcessMessageTransport(local application.MessageApplication) (*messageApplicationTransport, error) {
	adapter, err := messagegrpc.NewServer(local)
	if err != nil {
		return nil, fmt.Errorf("create in-process message grpc server: %w", err)
	}
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	messagev1.RegisterMessageServiceServer(server, adapter)
	go func() { _ = server.Serve(listener) }()

	connection, err := grpc.NewClient(
		"passthrough:///dipole-message-in-process",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
	)
	if err != nil {
		server.Stop()
		_ = listener.Close()
		return nil, fmt.Errorf("create in-process message grpc client: %w", err)
	}
	client, err := messagegrpc.NewClient(messagev1.NewMessageServiceClient(connection))
	if err != nil {
		_ = connection.Close()
		server.Stop()
		_ = listener.Close()
		return nil, fmt.Errorf("create remote message application: %w", err)
	}
	return &messageApplicationTransport{Application: client, server: server, connection: connection, listener: listener}, nil
}

func (t *messageApplicationTransport) Close() {
	if t == nil {
		return
	}
	if t.connection != nil {
		_ = t.connection.Close()
	}
	if t.server != nil {
		t.server.Stop()
	}
	if t.listener != nil {
		_ = t.listener.Close()
	}
}
