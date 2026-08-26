package bootstrap

import (
	"context"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/model"
)

type rpcCoreStub struct{}

func (rpcCoreStub) GetUserByUUID(userUUID string) (*model.User, error) {
	return &model.User{UUID: userUUID, Nickname: "RPC User"}, nil
}
func (rpcCoreStub) CanSendDirectMessage(string, string) (bool, error) { return true, nil }
func (rpcCoreStub) GetGroupByUUID(groupUUID string) (*model.Group, error) {
	return &model.Group{UUID: groupUUID, Name: "RPC Group"}, nil
}
func (rpcCoreStub) GetGroupMember(groupUUID, userUUID string) (*model.GroupMember, error) {
	return &model.GroupMember{GroupUUID: groupUUID, UserUUID: userUUID}, nil
}
func (rpcCoreStub) ListGroupMembers(groupUUID string) ([]*model.GroupMember, error) {
	return []*model.GroupMember{{GroupUUID: groupUUID, UserUUID: "U1"}}, nil
}
func (rpcCoreStub) GetOwnedFile(uploaderUUID, fileUUID string) (*model.UploadedFile, error) {
	return &model.UploadedFile{UUID: fileUUID, UploaderUUID: uploaderUUID, FileName: "rpc-file"}, nil
}

func TestCoreRPCServerAndClientUseAuthenticatedNetworkChannel(t *testing.T) {
	cfg := config.InternalRPC{
		Enabled:            true,
		SharedSecret:       "test-secret",
		CoreListenAddress:  "127.0.0.1:0",
		DialTimeoutSeconds: 2,
	}
	server, err := NewCoreRPCServer(cfg, rpcCoreStub{})
	if err != nil {
		t.Fatalf("start core rpc server: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		server.Close(ctx)
	})

	cfg.CoreTarget = server.Address()
	client, connection, err := DialCoreCapability(context.Background(), cfg)
	if err != nil {
		t.Fatalf("dial core capability: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })

	user, err := client.GetUserByUUID("U100")
	if err != nil {
		t.Fatalf("get user through core rpc: %v", err)
	}
	if user == nil || user.UUID != "U100" || user.Nickname != "RPC User" {
		t.Fatalf("unexpected user: %#v", user)
	}
}

func TestInternalRPCRejectsMissingRuntimeCredentials(t *testing.T) {
	if _, err := NewCoreRPCServer(config.InternalRPC{Enabled: true, CoreListenAddress: "127.0.0.1:0"}, rpcCoreStub{}); err == nil {
		t.Fatal("expected core rpc server without shared secret to fail")
	}
	if _, _, err := DialCoreCapability(context.Background(), config.InternalRPC{Enabled: true, CoreTarget: "127.0.0.1:1"}); err == nil {
		t.Fatal("expected core rpc client without shared secret to fail")
	}
}
