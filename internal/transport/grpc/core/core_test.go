package coregrpc

import (
	"context"
	"net"
	"testing"

	agentv1 "github.com/JekYUlll/Dipole/api/gen/go/agent/v1"
	commonv1 "github.com/JekYUlll/Dipole/api/gen/go/common/v1"
	corev1 "github.com/JekYUlll/Dipole/api/gen/go/core/v1"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	grpcauth "github.com/JekYUlll/Dipole/internal/transport/grpc/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
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

func (stubCoreCapability) GetOwnedFile(uploaderUUID, fileUUID string) (*model.UploadedFile, error) {
	if uploaderUUID != "U1" {
		return nil, nil
	}
	return &model.UploadedFile{UUID: fileUUID, UploaderUUID: uploaderUUID, FileName: "design.pen", FileSize: 42, ContentType: "application/octet-stream", URL: "https://files.test/design.pen"}, nil
}

func (stubCoreCapability) ListOwnedFiles(uploaderUUID, beforeFileUUID string, limit int) (*application.OwnedFilePage, error) {
	if uploaderUUID != "U1" || limit != 2 || beforeFileUUID != "" {
		return &application.OwnedFilePage{}, nil
	}
	return &application.OwnedFilePage{Files: []*model.UploadedFile{
		{UUID: "F2", UploaderUUID: uploaderUUID, FileName: "second.txt", FileSize: 2},
		{UUID: "F1", UploaderUUID: uploaderUUID, FileName: "first.txt", FileSize: 1},
	}, NextCursor: "F1", HasMore: true}, nil
}

func (stubCoreCapability) ListSearchConversationKeys(userUUID string) ([]string, error) {
	return []string{"direct:" + userUUID + ":U2", "group:G1"}, nil
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
	file, err := client.GetOwnedFile("U1", "F1")
	if err != nil || file == nil || file.UUID != "F1" || file.FileName != "design.pen" || file.FileSize != 42 {
		t.Fatalf("unexpected file: %+v err=%v", file, err)
	}
	file, err = client.GetOwnedFile("U2", "F1")
	if err != nil || file != nil {
		t.Fatalf("expected hidden unowned file, got %+v err=%v", file, err)
	}
	page, err := client.ListOwnedFiles("U1", "", 2)
	if err != nil || len(page.Files) != 2 || page.Files[0].UUID != "F2" || !page.HasMore || page.NextCursor != "F1" {
		t.Fatalf("unexpected owned file page: %+v err=%v", page, err)
	}
	keys, err := client.ListSearchConversationKeys("U1")
	if err != nil || len(keys) != 2 || keys[0] != "direct:U1:U2" {
		t.Fatalf("unexpected Search scope: keys=%v err=%v", keys, err)
	}
}

func TestServerRejectsMissingCallerService(t *testing.T) {
	rpc := newBufconnRPCClient(t, stubCoreCapability{})
	_, err := rpc.GetUser(context.Background(), &corev1.GetUserRequest{UserId: "U1"})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

func TestServerRejectsMissingSearchPrincipal(t *testing.T) {
	rpc := newBufconnRPCClient(t, stubCoreCapability{})
	_, err := rpc.ListSearchConversationKeys(context.Background(), &corev1.ListSearchConversationKeysRequest{
		Context: &commonv1.RequestContext{CallerService: "dipole-search"},
	})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected missing principal to be Unauthenticated, got %v", err)
	}
}

func TestClientRequiresCallerService(t *testing.T) {
	if _, err := NewClientForService(newBufconnRPCClient(t, stubCoreCapability{}), " "); err == nil {
		t.Fatal("expected empty caller service to fail")
	}
}

func TestRestrictServiceMethodsAllowsBoundAgentWriteRPCs(t *testing.T) {
	interceptor, err := grpcauth.NewUnaryServerInterceptor("secret", "dipole-agent")
	if err != nil {
		t.Fatalf("new auth interceptor: %v", err)
	}
	allowed := []string{
		agentv1.AgentCapabilityService_ConsumeApproval_FullMethodName,
		agentv1.AgentCapabilityService_ResolveApprovalGrant_FullMethodName,
		agentv1.AgentCapabilityService_ExecuteMcpMessageCommand_FullMethodName,
	}
	for _, method := range allowed {
		t.Run(method, func(t *testing.T) {
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
				"x-dipole-caller-service", "dipole-agent", "x-dipole-service-token", "secret",
			))
			_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: method}, func(ctx context.Context, request any) (any, error) {
				return RestrictServiceMethods(ctx, request, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) { return "ok", nil })
			})
			if err != nil {
				t.Fatalf("allow %s: %v", method, err)
			}
		})
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"x-dipole-caller-service", "dipole-agent", "x-dipole-service-token", "secret",
	))
	_, err = interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: corev1.CoreCapabilityService_GetUser_FullMethodName}, func(ctx context.Context, request any) (any, error) {
		return RestrictServiceMethods(ctx, request, &grpc.UnaryServerInfo{FullMethod: corev1.CoreCapabilityService_GetUser_FullMethodName}, func(context.Context, any) (any, error) { return "ok", nil })
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("deny unrelated agent RPC code = %s, want %s", status.Code(err), codes.PermissionDenied)
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
