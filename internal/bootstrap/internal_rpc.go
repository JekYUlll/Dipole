package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	grpcauth "github.com/JekYUlll/Dipole/internal/transport/grpc/auth"
	coregrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/core"
	corev1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/core/v1"
	messagev1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/message/v1"
	messagegrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/message"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
)

const (
	gatewayServiceName = "dipole-gateway"
	messageServiceName = "dipole-message"
)

type InternalRPCServer struct {
	listener net.Listener
	server   *grpc.Server
	done     chan struct{}
	stopOnce sync.Once
}

func NewCoreRPCServer(cfg config.InternalRPC, capability application.CoreCapability) (*InternalRPCServer, error) {
	adapter, err := coregrpc.NewServer(capability)
	if err != nil {
		return nil, fmt.Errorf("create core rpc adapter: %w", err)
	}
	return newInternalRPCServer(cfg.CoreListenAddress, cfg.SharedSecret, []string{messageServiceName}, func(server *grpc.Server) {
		corev1.RegisterCoreCapabilityServiceServer(server, adapter)
	})
}

func DialCoreCapability(ctx context.Context, cfg config.InternalRPC) (*coregrpc.Client, *grpc.ClientConn, error) {
	connection, err := dialInternalRPC(ctx, cfg.CoreTarget, cfg.DialTimeoutSeconds, grpcauth.Credentials{
		Service: messageServiceName,
		Secret:  cfg.SharedSecret,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("dial core rpc: %w", err)
	}
	client, err := coregrpc.NewClient(corev1.NewCoreCapabilityServiceClient(connection))
	if err != nil {
		_ = connection.Close()
		return nil, nil, fmt.Errorf("create core capability client: %w", err)
	}
	return client, connection, nil
}

func NewMessageRPCServer(cfg config.InternalRPC, messages application.MessageApplication) (*InternalRPCServer, error) {
	adapter, err := messagegrpc.NewServer(messages)
	if err != nil {
		return nil, fmt.Errorf("create message rpc adapter: %w", err)
	}
	return newInternalRPCServer(cfg.MessageListenAddress, cfg.SharedSecret, []string{gatewayServiceName}, func(server *grpc.Server) {
		messagev1.RegisterMessageServiceServer(server, adapter)
	})
}

func DialMessageApplication(ctx context.Context, cfg config.InternalRPC) (*messagegrpc.Client, *grpc.ClientConn, error) {
	connection, err := dialInternalRPC(ctx, cfg.MessageTarget, cfg.DialTimeoutSeconds, grpcauth.Credentials{
		Service: gatewayServiceName,
		Secret:  cfg.SharedSecret,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("dial message rpc: %w", err)
	}
	client, err := messagegrpc.NewClient(messagev1.NewMessageServiceClient(connection))
	if err != nil {
		_ = connection.Close()
		return nil, nil, fmt.Errorf("create message application client: %w", err)
	}
	return client, connection, nil
}

func newInternalRPCServer(address, secret string, allowedCallers []string, register func(*grpc.Server)) (*InternalRPCServer, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return nil, errors.New("internal rpc listen address is required")
	}
	interceptor, err := grpcauth.NewUnaryServerInterceptor(secret, allowedCallers...)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", address, err)
	}
	server := grpc.NewServer(grpc.UnaryInterceptor(interceptor))
	register(server)
	healthv1.RegisterHealthServer(server, newServingHealthServer())
	runtime := &InternalRPCServer{listener: listener, server: server, done: make(chan struct{})}
	go func() {
		defer close(runtime.done)
		_ = server.Serve(listener)
	}()
	return runtime, nil
}

func dialInternalRPC(ctx context.Context, target string, timeoutSeconds int, credentials grpcauth.Credentials) (*grpc.ClientConn, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, errors.New("internal rpc target is required")
	}
	interceptor, err := grpcauth.NewUnaryClientInterceptor(credentials)
	if err != nil {
		return nil, err
	}
	if timeoutSeconds <= 0 {
		timeoutSeconds = 5
	}
	dialCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()
	connection, err := grpc.DialContext(
		dialCtx,
		target,
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(interceptor),
	)
	if err != nil {
		return nil, err
	}
	if _, err := healthv1.NewHealthClient(connection).Check(dialCtx, &healthv1.HealthCheckRequest{}); err != nil {
		_ = connection.Close()
		return nil, fmt.Errorf("internal rpc health check failed: %w", err)
	}
	return connection, nil
}

func newServingHealthServer() *health.Server {
	server := health.NewServer()
	server.SetServingStatus("", healthv1.HealthCheckResponse_SERVING)
	return server
}

func (s *InternalRPCServer) Address() string {
	if s == nil || s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

func (s *InternalRPCServer) Close(ctx context.Context) {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		stopped := make(chan struct{})
		go func() {
			s.server.GracefulStop()
			close(stopped)
		}()
		select {
		case <-stopped:
		case <-ctx.Done():
			s.server.Stop()
			<-stopped
		}
		_ = s.listener.Close()
		<-s.done
	})
}
