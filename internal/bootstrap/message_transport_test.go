package bootstrap

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/model"
)

type stubMessageApplication struct{}

func (stubMessageApplication) SendDirectMessage(senderUUID, targetUUID, content, clientMessageID string) (*model.Message, error) {
	if senderUUID == "error" {
		return nil, errors.New("failed")
	}
	return &model.Message{UUID: "M1", SenderUUID: senderUUID, TargetUUID: targetUUID, Content: content, ClientMessageID: clientMessageID}, nil
}

func (stubMessageApplication) SendGroupMessage(senderUUID, groupUUID, content, clientMessageID string) (*model.Message, []string, error) {
	return &model.Message{UUID: "MG1", SenderUUID: senderUUID, TargetUUID: groupUUID, Content: content, ClientMessageID: clientMessageID}, []string{"U1", "U2"}, nil
}

func (stubMessageApplication) SendDirectFileMessage(senderUUID, targetUUID, fileUUID, clientMessageID string) (*model.Message, error) {
	return &model.Message{UUID: "MF1", SenderUUID: senderUUID, TargetUUID: targetUUID, FileID: fileUUID, ClientMessageID: clientMessageID}, nil
}

func (stubMessageApplication) SendGroupFileMessage(senderUUID, groupUUID, fileUUID, clientMessageID string) (*model.Message, []string, error) {
	return &model.Message{UUID: "MGF1", SenderUUID: senderUUID, TargetUUID: groupUUID, FileID: fileUUID, ClientMessageID: clientMessageID}, []string{"U1", "U2"}, nil
}

func (stubMessageApplication) ListDirectMessages(userUUID, targetUUID string, beforeID uint, limit int) ([]*model.Message, error) {
	return []*model.Message{{ID: beforeID, SenderUUID: userUUID, TargetUUID: targetUUID, Content: "direct"}}, nil
}

func (stubMessageApplication) ListGroupMessages(userUUID, groupUUID string, beforeID uint, limit int) ([]*model.Message, error) {
	return []*model.Message{{ID: beforeID, SenderUUID: userUUID, TargetUUID: groupUUID, Content: "group-before"}}, nil
}

func (stubMessageApplication) ListGroupMessagesAfter(userUUID, groupUUID string, afterID uint, limit int) ([]*model.Message, error) {
	return []*model.Message{{ID: afterID + 1, SenderUUID: userUUID, TargetUUID: groupUUID, Content: "group-after"}}, nil
}

func (stubMessageApplication) ListOfflineMessages(userUUID string, afterID uint, limit int) ([]*model.Message, error) {
	return []*model.Message{{ID: afterID + 1, TargetUUID: userUUID, Content: "offline"}}, nil
}

func TestMessageTransportDefaultsToLocal(t *testing.T) {
	local := stubMessageApplication{}
	transport, err := newMessageApplicationTransport(context.Background(), config.Message{}, config.InternalRPC{}, local)
	if err != nil {
		t.Fatalf("new local transport: %v", err)
	}
	defer transport.Close()
	if _, ok := transport.Application.(stubMessageApplication); !ok {
		t.Fatalf("expected local application, got %T", transport.Application)
	}
}

func TestMessageTransportModesPassSharedApplicationContract(t *testing.T) {
	for _, mode := range []string{"local", "grpc"} {
		t.Run(mode, func(t *testing.T) {
			rpcCfg := config.InternalRPC{}
			if mode == "grpc" {
				rpcCfg = config.InternalRPC{Enabled: true, SharedSecret: "test-secret", MessageListenAddress: "127.0.0.1:0", DialTimeoutSeconds: 2}
				server, err := NewMessageRPCServer(rpcCfg, stubMessageApplication{})
				if err != nil {
					t.Fatalf("start message rpc server: %v", err)
				}
				t.Cleanup(func() {
					ctx, cancel := context.WithTimeout(context.Background(), time.Second)
					defer cancel()
					server.Close(ctx)
				})
				rpcCfg.MessageTarget = server.Address()
			}
			transport, err := newMessageApplicationTransport(context.Background(), config.Message{Transport: mode}, rpcCfg, stubMessageApplication{})
			if err != nil {
				t.Fatalf("new %s transport: %v", mode, err)
			}
			defer transport.Close()
			runMessageApplicationContract(t, transport.Application)
		})
	}
}

func TestMessageTransportRejectsUnknownMode(t *testing.T) {
	_, err := newMessageApplicationTransport(context.Background(), config.Message{Transport: "shadow"}, config.InternalRPC{}, stubMessageApplication{})
	if err == nil {
		t.Fatal("expected unknown message transport to fail")
	}
}

func TestMessageTransportRejectsShadowWithoutRPC(t *testing.T) {
	_, err := newMessageApplicationTransport(
		context.Background(),
		config.Message{Transport: "local", ShadowQueries: true},
		config.InternalRPC{},
		stubMessageApplication{},
	)
	if err == nil {
		t.Fatal("expected shadow queries without internal rpc to fail")
	}
}

func TestMessageTransportRemoteFailureKeepsLocalRollbackAvailable(t *testing.T) {
	rpcCfg := config.InternalRPC{
		Enabled:            true,
		SharedSecret:       "test-secret",
		MessageTarget:      "127.0.0.1:1",
		DialTimeoutSeconds: 1,
	}
	startedAt := time.Now()
	if _, err := newMessageApplicationTransport(context.Background(), config.Message{Transport: "grpc"}, rpcCfg, stubMessageApplication{}); err == nil {
		t.Fatal("expected unavailable remote transport to fail")
	}
	if elapsed := time.Since(startedAt); elapsed > 2*time.Second {
		t.Fatalf("remote transport exceeded bounded startup failure: %v", elapsed)
	}
	local, err := newMessageApplicationTransport(context.Background(), config.Message{Transport: "local"}, rpcCfg, stubMessageApplication{})
	if err != nil {
		t.Fatalf("local rollback transport failed: %v", err)
	}
	defer local.Close()
	if message, err := local.Application.SendDirectMessage("U1", "U2", "rollback", "C-rollback"); err != nil || message == nil {
		t.Fatalf("local rollback command failed: message=%+v err=%v", message, err)
	}
}

func runMessageApplicationContract(t *testing.T, messages application.MessageApplication) {
	t.Helper()
	direct, err := messages.SendDirectMessage("U1", "U2", "hello", "C1")
	if err != nil || direct == nil || direct.UUID != "M1" || direct.ClientMessageID != "C1" {
		t.Fatalf("direct command mismatch: message=%+v err=%v", direct, err)
	}
	group, recipients, err := messages.SendGroupMessage("U1", "G1", "group", "C2")
	if err != nil || group == nil || group.UUID != "MG1" || len(recipients) != 2 {
		t.Fatalf("group command mismatch: message=%+v recipients=%+v err=%v", group, recipients, err)
	}
	directFile, err := messages.SendDirectFileMessage("U1", "U2", "F1", "C3")
	if err != nil || directFile == nil || directFile.FileID != "F1" {
		t.Fatalf("direct file mismatch: message=%+v err=%v", directFile, err)
	}
	groupFile, recipients, err := messages.SendGroupFileMessage("U1", "G1", "F2", "C4")
	if err != nil || groupFile == nil || groupFile.FileID != "F2" || len(recipients) != 2 {
		t.Fatalf("group file mismatch: message=%+v recipients=%+v err=%v", groupFile, recipients, err)
	}
	directHistory, err := messages.ListDirectMessages("U1", "U2", 40, 20)
	if err != nil || len(directHistory) != 1 || directHistory[0].ID != 40 || directHistory[0].Content != "direct" {
		t.Fatalf("direct history mismatch: messages=%+v err=%v", directHistory, err)
	}
	groupHistory, err := messages.ListGroupMessages("U1", "G1", 50, 20)
	if err != nil || len(groupHistory) != 1 || groupHistory[0].ID != 50 || groupHistory[0].Content != "group-before" {
		t.Fatalf("group history mismatch: messages=%+v err=%v", groupHistory, err)
	}
	groupAfter, err := messages.ListGroupMessagesAfter("U1", "G1", 60, 20)
	if err != nil || len(groupAfter) != 1 || groupAfter[0].ID != 61 || groupAfter[0].Content != "group-after" {
		t.Fatalf("group after mismatch: messages=%+v err=%v", groupAfter, err)
	}
	offline, err := messages.ListOfflineMessages("U1", 70, 20)
	if err != nil || len(offline) != 1 || offline[0].ID != 71 || offline[0].Content != "offline" {
		t.Fatalf("offline history mismatch: messages=%+v err=%v", offline, err)
	}
}

func BenchmarkMessageTransportDirectHistory(b *testing.B) {
	certFile, keyFile, caFile := writeRPCIdentity(b, gatewayServiceName)
	rpcCfg := config.InternalRPC{
		Enabled:              true,
		SharedSecret:         "benchmark-secret",
		MessageListenAddress: "127.0.0.1:0",
		DialTimeoutSeconds:   2,
		TLSEnabled:           true,
		TLSCertFile:          certFile,
		TLSKeyFile:           keyFile,
		TLSCAFile:            caFile,
		TLSServerName:        "localhost",
	}
	server, err := NewMessageRPCServer(rpcCfg, stubMessageApplication{})
	if err != nil {
		b.Fatalf("start benchmark message rpc: %v", err)
	}
	b.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		server.Close(ctx)
	})
	rpcCfg.MessageTarget = server.Address()

	for _, mode := range []string{"local", "grpc"} {
		b.Run(mode, func(b *testing.B) {
			transport, err := newMessageApplicationTransport(context.Background(), config.Message{Transport: mode}, rpcCfg, stubMessageApplication{})
			if err != nil {
				b.Fatalf("create %s benchmark transport: %v", mode, err)
			}
			b.Cleanup(transport.Close)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if _, err := transport.Application.ListDirectMessages("U1", "U2", 40, 20); err != nil {
					b.Fatalf("list direct messages: %v", err)
				}
			}
		})
	}
}
