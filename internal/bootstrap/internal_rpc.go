package bootstrap

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"os"
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
	grpcCredentials "google.golang.org/grpc/credentials"
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
	return newInternalRPCServer(cfg, cfg.CoreListenAddress, []string{messageServiceName}, func(server *grpc.Server) {
		corev1.RegisterCoreCapabilityServiceServer(server, adapter)
	})
}

func DialCoreCapability(ctx context.Context, cfg config.InternalRPC) (*coregrpc.Client, *grpc.ClientConn, error) {
	connection, err := dialInternalRPC(ctx, cfg, cfg.CoreTarget, grpcauth.Credentials{
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
	return newInternalRPCServer(cfg, cfg.MessageListenAddress, []string{gatewayServiceName}, func(server *grpc.Server) {
		messagev1.RegisterMessageServiceServer(server, adapter)
	})
}

func DialMessageApplication(ctx context.Context, cfg config.InternalRPC) (*messagegrpc.Client, *grpc.ClientConn, error) {
	connection, err := dialInternalRPC(ctx, cfg, cfg.MessageTarget, grpcauth.Credentials{
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

func newInternalRPCServer(cfg config.InternalRPC, address string, allowedCallers []string, register func(*grpc.Server)) (*InternalRPCServer, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return nil, errors.New("internal rpc listen address is required")
	}
	interceptor, err := grpcauth.NewUnaryServerInterceptor(cfg.SharedSecret, allowedCallers...)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", address, err)
	}
	options := []grpc.ServerOption{grpc.UnaryInterceptor(interceptor)}
	transportCredentials, err := internalRPCServerCredentials(cfg, address)
	if err != nil {
		_ = listener.Close()
		return nil, err
	}
	if transportCredentials != nil {
		options = append(options, grpc.Creds(transportCredentials))
	}
	server := grpc.NewServer(options...)
	register(server)
	healthv1.RegisterHealthServer(server, newServingHealthServer())
	runtime := &InternalRPCServer{listener: listener, server: server, done: make(chan struct{})}
	go func() {
		defer close(runtime.done)
		_ = server.Serve(listener)
	}()
	return runtime, nil
}

func dialInternalRPC(ctx context.Context, cfg config.InternalRPC, target string, serviceCredentials grpcauth.Credentials) (*grpc.ClientConn, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, errors.New("internal rpc target is required")
	}
	interceptor, err := grpcauth.NewUnaryClientInterceptor(serviceCredentials)
	if err != nil {
		return nil, err
	}
	if cfg.DialTimeoutSeconds <= 0 {
		cfg.DialTimeoutSeconds = 5
	}
	transportCredentials, err := internalRPCClientCredentials(cfg, target)
	if err != nil {
		return nil, err
	}
	dialCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.DialTimeoutSeconds)*time.Second)
	defer cancel()
	connection, err := grpc.DialContext(
		dialCtx,
		target,
		grpc.WithBlock(),
		grpc.WithTransportCredentials(transportCredentials),
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

func internalRPCServerCredentials(cfg config.InternalRPC, address string) (grpcCredentials.TransportCredentials, error) {
	if !cfg.TLSEnabled {
		if !isLoopbackAddress(address) {
			return nil, fmt.Errorf("plaintext internal rpc listener must use loopback address: %s", address)
		}
		return nil, nil
	}
	certificate, roots, err := loadMutualTLSIdentity(cfg)
	if err != nil {
		return nil, err
	}
	return grpcCredentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{certificate},
		ClientCAs:    roots,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS13,
	}), nil
}

func internalRPCClientCredentials(cfg config.InternalRPC, target string) (grpcCredentials.TransportCredentials, error) {
	if !cfg.TLSEnabled {
		if !isLoopbackAddress(target) {
			return nil, fmt.Errorf("plaintext internal rpc target must use loopback address: %s", target)
		}
		return insecure.NewCredentials(), nil
	}
	certificate, roots, err := loadMutualTLSIdentity(cfg)
	if err != nil {
		return nil, err
	}
	return grpcCredentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{certificate},
		RootCAs:      roots,
		ServerName:   cfg.TLSServerName,
		MinVersion:   tls.VersionTLS13,
	}), nil
}

func loadMutualTLSIdentity(cfg config.InternalRPC) (tls.Certificate, *x509.CertPool, error) {
	certificate, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("load internal rpc certificate: %w", err)
	}
	caPEM, err := os.ReadFile(cfg.TLSCAFile)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("read internal rpc ca: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return tls.Certificate{}, nil, errors.New("internal rpc ca contains no certificates")
	}
	return certificate, roots, nil
}

func isLoopbackAddress(address string) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(address))
	if err != nil {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
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
