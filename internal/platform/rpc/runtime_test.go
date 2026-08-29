package rpc

import (
	"context"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/config"
	grpcauth "github.com/JekYUlll/Dipole/internal/transport/grpc/auth"
	"google.golang.org/grpc"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
)

func TestServerAndDialOwnAuthenticatedHealthLifecycle(t *testing.T) {
	cfg := config.InternalRPC{Enabled: true, SharedSecret: "test-secret", DialTimeoutSeconds: 2}
	server, err := NewServer(cfg, "127.0.0.1:0", []string{"dipole-gateway"}, func(_ *grpc.Server) {})
	if err != nil {
		t.Fatalf("start internal rpc server: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		server.Close(ctx)
	})

	connection, err := Dial(context.Background(), cfg, server.Address(), grpcauth.Credentials{Service: "dipole-gateway", Secret: cfg.SharedSecret})
	if err != nil {
		t.Fatalf("dial internal rpc server: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	if _, err := healthv1.NewHealthClient(connection).Check(context.Background(), &healthv1.HealthCheckRequest{}); err != nil {
		t.Fatalf("check internal rpc health: %v", err)
	}
}

func TestNewServerRejectsNonLoopbackPlaintextListener(t *testing.T) {
	cfg := config.InternalRPC{Enabled: true, SharedSecret: "test-secret"}
	if _, err := NewServer(cfg, "192.0.2.1:0", []string{"dipole-gateway"}, func(_ *grpc.Server) {}); err == nil {
		t.Fatal("expected non-loopback plaintext listener rejection")
	}
}
