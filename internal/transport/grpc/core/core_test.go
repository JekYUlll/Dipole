package coregrpc

import (
	"context"
	"net"
	"testing"

	"github.com/JekYUlll/Dipole/internal/model"
	corev1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/core/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type stubCoreCapability struct{}

func (stubCoreCapability) GetUserByUUID(userUUID string) (*model.User, error) {
	return &model.User{UUID: userUUID, Nickname: "Alice", UserType: model.UserTypeAssistant, Status: model.UserStatusNormal}, nil
}

func (stubCoreCapability) CanSendDirectMessage(userUUID, friendUUID string) (bool, error) {
	return userUUID == "U1" && friendUUID == "U2", nil
}

func (stubCoreCapability) GetGroupByUUID(groupUUID string) (*model.Group, error) {
	return &model.Group{UUID: groupUUID, Name: "Project", OwnerUUID: "U1", MemberCount: 2, Status: model.GroupStatusNormal}, nil
}

func (stubCoreCapability) GetGroupMember(groupUUID, userUUID string) (*model.GroupMember, error) {
	return &model.GroupMember{GroupUUID: groupUUID, UserUUID: userUUID, Role: model.GroupMemberRoleOwner}, nil
}

func (stubCoreCapability) ListGroupMembers(groupUUID string) ([]*model.GroupMember, error) {
	return []*model.GroupMember{{GroupUUID: groupUUID, UserUUID: "U1"}, nil, {GroupUUID: groupUUID, UserUUID: "U2"}}, nil
}

func TestRemoteClientImplementsCoreCapability(t *testing.T) {
	rpc := newBufconnRPCClient(t, stubCoreCapability{})
	client, err := NewClient(rpc)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	user, err := client.GetUserByUUID("U2")
	if err != nil || user == nil || user.UUID != "U2" || !user.IsAssistant() {
		t.Fatalf("unexpected user: %+v err=%v", user, err)
	}
	allowed, err := client.CanSendDirectMessage("U1", "U2")
	if err != nil || !allowed {
		t.Fatalf("unexpected direct authorization: allowed=%t err=%v", allowed, err)
	}
	group, err := client.GetGroupByUUID("G1")
	if err != nil || group == nil || group.MemberCount != 2 {
		t.Fatalf("unexpected group: %+v err=%v", group, err)
	}
	member, err := client.GetGroupMember("G1", "U1")
	if err != nil || member == nil || member.Role != model.GroupMemberRoleOwner {
		t.Fatalf("unexpected member: %+v err=%v", member, err)
	}
	members, err := client.ListGroupMembers("G1")
	if err != nil || len(members) != 2 || members[1].UserUUID != "U2" {
		t.Fatalf("unexpected members: %+v err=%v", members, err)
	}
}

func TestServerRejectsMissingCallerService(t *testing.T) {
	rpc := newBufconnRPCClient(t, stubCoreCapability{})
	_, err := rpc.GetUser(context.Background(), &corev1.GetUserRequest{UserId: "U1"})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

func newBufconnRPCClient(t *testing.T, capability stubCoreCapability) corev1.CoreCapabilityServiceClient {
	t.Helper()
	adapter, err := NewServer(capability)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	corev1.RegisterCoreCapabilityServiceServer(server, adapter)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	connection, err := grpc.NewClient(
		"passthrough:///bufconn",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
	)
	if err != nil {
		t.Fatalf("new grpc client: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	return corev1.NewCoreCapabilityServiceClient(connection)
}
