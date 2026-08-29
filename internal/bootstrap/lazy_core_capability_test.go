package bootstrap

import (
	"context"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/config"
)

func TestLazyCoreCapabilityDoesNotBlockMessageStartupAndRetries(t *testing.T) {
	cfg := config.InternalRPC{
		Enabled:            true,
		SharedSecret:       "test-secret",
		CoreTarget:         "127.0.0.1:1",
		DialTimeoutSeconds: 1,
	}
	lazy := newLazyCoreCapability(cfg)
	if lazy.client != nil || lazy.conn != nil {
		t.Fatal("lazy capability must not dial during construction")
	}

	startedAt := time.Now()
	if _, err := lazy.GetUserByUUID("U-before-core"); err == nil {
		t.Fatal("expected unavailable Core to fail the request")
	}
	if elapsed := time.Since(startedAt); elapsed > 2*time.Second {
		t.Fatalf("unavailable Core exceeded bounded retry: %v", elapsed)
	}
	if lazy.client != nil || lazy.conn != nil {
		t.Fatal("failed lazy dial must not be cached")
	}

	server, err := NewCoreRPCServer(config.InternalRPC{
		Enabled:            true,
		SharedSecret:       "test-secret",
		CoreListenAddress:  "127.0.0.1:0",
		DialTimeoutSeconds: 1,
	}, rpcCoreStub{})
	if err != nil {
		t.Fatalf("start Core RPC server: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		server.Close(ctx)
	})
	lazy.cfg.CoreTarget = server.Address()

	user, err := lazy.GetUserByUUID("U-after-core")
	if err != nil {
		t.Fatalf("retry Core capability: %v", err)
	}
	if user == nil || user.UUID != "U-after-core" {
		t.Fatalf("unexpected user after retry: %#v", user)
	}
	if lazy.client == nil || lazy.conn == nil {
		t.Fatal("successful retry must cache the healthy connection")
	}
	if err := lazy.Close(); err != nil {
		t.Fatalf("close lazy capability: %v", err)
	}
	if _, err := lazy.GetUserByUUID("U-closed"); err == nil {
		t.Fatal("closed lazy capability must reject new calls")
	}
}
