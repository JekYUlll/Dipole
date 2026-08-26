package bootstrap

import (
	"context"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
)

func TestSyncTransportModesPassSharedApplicationContract(t *testing.T) {
	for _, mode := range []string{"local", "grpc"} {
		t.Run(mode, func(t *testing.T) {
			local := rpcSyncStub{}
			rpcCfg := config.InternalRPC{}
			if mode == "grpc" {
				rpcCfg = config.InternalRPC{Enabled: true, SharedSecret: "test-secret", SyncListenAddress: "127.0.0.1:0", DialTimeoutSeconds: 2}
				server, err := NewSyncRPCServer(rpcCfg, local)
				if err != nil {
					t.Fatalf("start Sync rpc: %v", err)
				}
				t.Cleanup(func() {
					ctx, cancel := context.WithTimeout(context.Background(), time.Second)
					defer cancel()
					server.Close(ctx)
				})
				rpcCfg.SyncTarget = server.Address()
			}
			transport, err := newSyncApplicationTransport(t.Context(), config.Sync{Transport: mode}, rpcCfg, local)
			if err != nil {
				t.Fatalf("new %s transport: %v", mode, err)
			}
			t.Cleanup(transport.Close)
			runSyncApplicationContract(t, transport.Application)
		})
	}
}

func TestSyncTransportRejectsUnknownMode(t *testing.T) {
	if _, err := newSyncApplicationTransport(t.Context(), config.Sync{Transport: "shadow"}, config.InternalRPC{}, rpcSyncStub{}); err == nil {
		t.Fatal("expected unknown Sync transport to fail")
	}
}

func TestSyncTransportRemoteFailureKeepsLocalRollbackAvailable(t *testing.T) {
	rpcCfg := config.InternalRPC{Enabled: true, SharedSecret: "test-secret", SyncTarget: "127.0.0.1:1", DialTimeoutSeconds: 1}
	local := rpcSyncStub{}
	if _, err := newSyncApplicationTransport(t.Context(), config.Sync{Transport: "grpc"}, rpcCfg, local); err == nil {
		t.Fatal("expected unavailable remote Sync transport to fail")
	}
	transport, err := newSyncApplicationTransport(t.Context(), config.Sync{Transport: "local"}, rpcCfg, local)
	if err != nil {
		t.Fatalf("local rollback transport: %v", err)
	}
	transport.Close()
}

func runSyncApplicationContract(t *testing.T, syncApplication application.SyncApplication) {
	t.Helper()
	page, err := syncApplication.List("U1", 7, 20)
	if err != nil || page == nil || page.NextSeq != 8 {
		t.Fatalf("list Sync page: page=%+v err=%v", page, err)
	}
	checkpoint, err := syncApplication.AdvanceCheckpoint("U1", "web-1", 11)
	if err != nil || checkpoint == nil || checkpoint.SyncSeq != 11 {
		t.Fatalf("advance checkpoint: checkpoint=%+v err=%v", checkpoint, err)
	}
	group, err := syncApplication.AdvanceGroupCheckpoint("U1", "web-1", "G1", 12)
	if err != nil || group == nil || group.PulledMessageSeq != 12 {
		t.Fatalf("advance group checkpoint: checkpoint=%+v err=%v", group, err)
	}
}
