// Package rpc owns the transport-level lifecycle shared by internal gRPC
// services. Domain adapters and method authorization remain at service edges.
package rpc

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

	"github.com/JekYUlll/Dipole/internal/config"
	grpcauth "github.com/JekYUlll/Dipole/internal/transport/grpc/auth"
	"google.golang.org/grpc"
	grpcCredentials "google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
)

// Server owns a single internal gRPC listener and its serving health state.
type Server struct {
	listener net.Listener
	server   *grpc.Server
	health   *health.Server
	done     chan struct{}
	stopOnce sync.Once
}

// NewServer creates and starts an authenticated internal gRPC server.
func NewServer(cfg config.InternalRPC, address string, allowedCallers []string, register func(*grpc.Server), additionalInterceptors ...grpc.UnaryServerInterceptor) (*Server, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return nil, errors.New("internal rpc listen address is required")
	}
	if register == nil {
		return nil, errors.New("internal rpc register callback is required")
	}
	interceptor, err := grpcauth.NewUnaryServerInterceptor(cfg.SharedSecret, allowedCallers...)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", address, err)
	}
	interceptors := append([]grpc.UnaryServerInterceptor{interceptor}, additionalInterceptors...)
	options := []grpc.ServerOption{grpc.ChainUnaryInterceptor(interceptors...)}
	transportCredentials, err := serverCredentials(cfg, address)
	if err != nil {
		_ = listener.Close()
		return nil, err
	}
	if transportCredentials != nil {
		options = append(options, grpc.Creds(transportCredentials))
	}
	server := grpc.NewServer(options...)
	register(server)
	healthServer := servingHealthServer()
	healthv1.RegisterHealthServer(server, healthServer)
	runtime := &Server{listener: listener, server: server, health: healthServer, done: make(chan struct{})}
	go func() {
		defer close(runtime.done)
		_ = server.Serve(listener)
	}()
	return runtime, nil
}

// Dial opens an authenticated internal gRPC connection and verifies serving
// health before returning it to the caller.
func Dial(ctx context.Context, cfg config.InternalRPC, target string, credentials grpcauth.Credentials) (*grpc.ClientConn, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, errors.New("internal rpc target is required")
	}
	interceptor, err := grpcauth.NewUnaryClientInterceptor(credentials)
	if err != nil {
		return nil, err
	}
	if cfg.DialTimeoutSeconds <= 0 {
		cfg.DialTimeoutSeconds = 5
	}
	transportCredentials, err := clientCredentials(cfg, target)
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

func serverCredentials(cfg config.InternalRPC, address string) (grpcCredentials.TransportCredentials, error) {
	if !cfg.TLSEnabled {
		if !loopbackAddress(address) {
			return nil, fmt.Errorf("plaintext internal rpc listener must use loopback address: %s", address)
		}
		return nil, nil
	}
	certificate, roots, err := mutualTLSIdentity(cfg)
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

func clientCredentials(cfg config.InternalRPC, target string) (grpcCredentials.TransportCredentials, error) {
	if !cfg.TLSEnabled {
		if !loopbackAddress(target) {
			return nil, fmt.Errorf("plaintext internal rpc target must use loopback address: %s", target)
		}
		return insecure.NewCredentials(), nil
	}
	certificate, roots, err := mutualTLSIdentity(cfg)
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

func mutualTLSIdentity(cfg config.InternalRPC) (tls.Certificate, *x509.CertPool, error) {
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

func loopbackAddress(address string) bool {
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

func servingHealthServer() *health.Server {
	server := health.NewServer()
	server.SetServingStatus("", healthv1.HealthCheckResponse_SERVING)
	return server
}

func (s *Server) Address() string {
	if s == nil || s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

func (s *Server) SetServing(serving bool) {
	if s == nil || s.health == nil {
		return
	}
	status := healthv1.HealthCheckResponse_NOT_SERVING
	if serving {
		status = healthv1.HealthCheckResponse_SERVING
	}
	s.health.SetServingStatus("", status)
}

func (s *Server) Close(ctx context.Context) {
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
