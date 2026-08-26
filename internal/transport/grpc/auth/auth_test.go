package grpcauth

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestUnaryAuthentication(t *testing.T) {
	tests := []struct {
		name        string
		credentials Credentials
		wantCode    codes.Code
	}{
		{
			name:        "valid credentials",
			credentials: Credentials{Service: "dipole-message", Secret: "test-secret"},
			wantCode:    codes.OK,
		},
		{
			name:        "missing credentials",
			credentials: Credentials{},
			wantCode:    codes.Unauthenticated,
		},
		{
			name:        "wrong secret",
			credentials: Credentials{Service: "dipole-message", Secret: "wrong-secret"},
			wantCode:    codes.Unauthenticated,
		},
		{
			name:        "caller not allowed",
			credentials: Credentials{Service: "dipole-gateway", Secret: "test-secret"},
			wantCode:    codes.PermissionDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newHealthClient(t, tt.credentials)
			_, err := client.Check(context.Background(), &healthv1.HealthCheckRequest{})
			if code := status.Code(err); code != tt.wantCode {
				t.Fatalf("health check code = %s, want %s (err=%v)", code, tt.wantCode, err)
			}
		})
	}
}

func TestNewUnaryInterceptorsRejectInvalidConfiguration(t *testing.T) {
	if _, err := NewUnaryClientInterceptor(Credentials{}); err == nil {
		t.Fatal("expected empty client credentials to fail")
	}
	if _, err := NewUnaryServerInterceptor("", "dipole-message"); err == nil {
		t.Fatal("expected empty server secret to fail")
	}
	if _, err := NewUnaryServerInterceptor("test-secret"); err == nil {
		t.Fatal("expected empty caller allowlist to fail")
	}
}

func newHealthClient(t *testing.T, credentials Credentials) healthv1.HealthClient {
	t.Helper()

	serverInterceptor, err := NewUnaryServerInterceptor("test-secret", "dipole-message")
	if err != nil {
		t.Fatalf("create server interceptor: %v", err)
	}
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer(grpc.UnaryInterceptor(serverInterceptor))
	healthv1.RegisterHealthServer(server, health.NewServer())
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})

	dialOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
	}
	if credentials != (Credentials{}) {
		clientInterceptor, err := NewUnaryClientInterceptor(credentials)
		if err != nil {
			t.Fatalf("create client interceptor: %v", err)
		}
		dialOptions = append(dialOptions, grpc.WithUnaryInterceptor(clientInterceptor))
	}
	connection, err := grpc.NewClient("passthrough:///auth-test", dialOptions...)
	if err != nil {
		t.Fatalf("create grpc client: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	return healthv1.NewHealthClient(connection)
}
