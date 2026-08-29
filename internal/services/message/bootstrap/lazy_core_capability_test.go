package bootstrap

import (
	"context"
	"testing"
	"time"

	corev1 "github.com/JekYUlll/Dipole/api/gen/go/core/v1"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/model"
	platformrpc "github.com/JekYUlll/Dipole/internal/platform/rpc"
	coregrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/core"
	"google.golang.org/grpc"
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

	adapter, err := coregrpc.NewServer(messageCoreCapabilityStub{})
	if err != nil {
		t.Fatalf("create Core RPC adapter: %v", err)
	}
	server, err := platformrpc.NewServer(config.InternalRPC{
		Enabled:            true,
		SharedSecret:       "test-secret",
		CoreListenAddress:  "127.0.0.1:0",
		DialTimeoutSeconds: 1,
	}, "127.0.0.1:0", []string{"dipole-message"}, func(server *grpc.Server) {
		corev1.RegisterCoreCapabilityServiceServer(server, adapter)
	}, coregrpc.RestrictServiceMethods)
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

type messageCoreCapabilityStub struct{}

var _ application.CoreCapability = messageCoreCapabilityStub{}

func (messageCoreCapabilityStub) GetUserByUUID(userUUID string) (*model.User, error) {
	return &model.User{UUID: userUUID, Nickname: "RPC User"}, nil
}

func (messageCoreCapabilityStub) CanSendDirectMessage(string, string) (bool, error) {
	return true, nil
}

func (messageCoreCapabilityStub) GetGroupByUUID(groupUUID string) (*model.Group, error) {
	return &model.Group{UUID: groupUUID, Name: "RPC Group"}, nil
}

func (messageCoreCapabilityStub) GetGroupMember(groupUUID, userUUID string) (*model.GroupMember, error) {
	return &model.GroupMember{GroupUUID: groupUUID, UserUUID: userUUID}, nil
}

func (messageCoreCapabilityStub) ListGroupMembers(groupUUID string) ([]*model.GroupMember, error) {
	return []*model.GroupMember{{GroupUUID: groupUUID, UserUUID: "U1"}}, nil
}

func (messageCoreCapabilityStub) GetOwnedFile(uploaderUUID, fileUUID string) (*model.UploadedFile, error) {
	return &model.UploadedFile{UUID: fileUUID, UploaderUUID: uploaderUUID, FileName: "rpc-file"}, nil
}

func (messageCoreCapabilityStub) ListSearchConversationKeys(string) ([]string, error) {
	return nil, nil
}
