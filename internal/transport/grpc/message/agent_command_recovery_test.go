package messagegrpc

import (
	"context"
	"net"
	"sync/atomic"
	"testing"

	messagev1 "github.com/JekYUlll/Dipole/api/gen/go/message/v1"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	agentapplication "github.com/JekYUlll/Dipole/internal/services/agent/application"
	grpcauth "github.com/JekYUlll/Dipole/internal/transport/grpc/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type postCommitUnavailableMessageServer struct {
	messagev1.UnimplementedMessageServiceServer
	delegate messagev1.MessageServiceServer
	failOnce atomic.Bool
}

func (s *postCommitUnavailableMessageServer) SendSystemDirectMessage(ctx context.Context, request *messagev1.SendSystemDirectMessageRequest) (*messagev1.SendMessageResponse, error) {
	response, err := s.delegate.SendSystemDirectMessage(ctx, request)
	if err != nil {
		return nil, err
	}
	if s.failOnce.CompareAndSwap(false, true) {
		return nil, status.Error(codes.Unavailable, "response lost after Message commit")
	}
	return response, nil
}

func (s *postCommitUnavailableMessageServer) GetMessageCommandReceipt(ctx context.Context, request *messagev1.GetMessageCommandReceiptRequest) (*messagev1.GetMessageCommandReceiptResponse, error) {
	return s.delegate.GetMessageCommandReceipt(ctx, request)
}

func TestCoreAgentMessageCommandRecoversCommittedMessageAfterGRPCResponseLoss(t *testing.T) {
	t.Parallel()

	var message *model.Message
	var commits atomic.Int32
	applicationService := &stubMessageApplication{
		sendDirect: func(string, string, string, string) (*model.Message, error) { return nil, nil },
		listGroup:  emptyGroupList,
		sendSystemCommand: func(_ context.Context, senderUUID, targetUUID, content, clientMessageID string) (*model.Message, error) {
			if commits.Add(1) != 1 {
				t.Fatal("Message command was committed more than once")
			}
			message = &model.Message{
				UUID:            "M-RECOVERED",
				SenderUUID:      senderUUID,
				TargetUUID:      targetUUID,
				TargetType:      model.MessageTargetDirect,
				ConversationKey: model.DirectConversationKey(senderUUID, targetUUID),
				ClientMessageID: clientMessageID,
				MessageType:     model.MessageTypeSystem,
				Content:         content,
			}
			return message, nil
		},
		receipt: func(senderUUID, clientMessageID string) (*application.MessageCommandReceipt, error) {
			if message == nil {
				return &application.MessageCommandReceipt{Status: application.MessageCommandReceiptStatusAbsent}, nil
			}
			if senderUUID != message.SenderUUID || clientMessageID != message.ClientMessageID {
				t.Fatalf("receipt binding drift: sender=%q client_message_id=%q", senderUUID, clientMessageID)
			}
			return &application.MessageCommandReceipt{
				Status:  application.MessageCommandReceiptStatusCommitted,
				Message: message,
			}, nil
		},
	}

	messageServer, err := NewServer(applicationService)
	if err != nil {
		t.Fatalf("new Message server: %v", err)
	}
	remote, closeRemote := newCoreMessageClientThroughResponseLoss(t, &postCommitUnavailableMessageServer{delegate: messageServer})
	defer closeRemote()

	commands, err := agentapplication.NewLocalAgentCommandV1(remote)
	if err != nil {
		t.Fatalf("new Agent command service: %v", err)
	}
	invocation := application.AgentInvocationV1{
		TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI",
		Permissions: []string{application.AgentPermissionMessageWrite},
		ResourceScopes: []application.AgentResourceScopeV1{{
			ResourceType: application.AgentResourceTypeConversation,
			ResourceID:   model.DirectConversationKey("U100", "UAI"),
			Actions:      []string{application.AgentResourceActionWrite},
		}},
	}
	result, err := commands.SendMessage(context.Background(), application.AgentMessageCommandV1{
		CommandID: "tool:response-loss", Kind: application.AgentMessageCommandSystemMessageV1,
		Invocation: invocation, Content: "durable notice",
	})
	if err != nil {
		t.Fatalf("recover committed Message command: %v", err)
	}
	if result == nil || result.UUID != "M-RECOVERED" || result.MessageType != model.MessageTypeSystem || commits.Load() != 1 {
		t.Fatalf("unexpected recovered message=%+v commits=%d", result, commits.Load())
	}
}

func newCoreMessageClientThroughResponseLoss(t *testing.T, server messagev1.MessageServiceServer) (*Client, func()) {
	t.Helper()
	serverAuth, err := grpcauth.NewUnaryServerInterceptor("test-secret", "dipole-core")
	if err != nil {
		t.Fatalf("new Message server auth: %v", err)
	}
	clientAuth, err := grpcauth.NewUnaryClientInterceptor(grpcauth.Credentials{Service: "dipole-core", Secret: "test-secret"})
	if err != nil {
		t.Fatalf("new Core Message client auth: %v", err)
	}
	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(serverAuth))
	messagev1.RegisterMessageServiceServer(grpcServer, server)
	go func() { _ = grpcServer.Serve(listener) }()

	connection, err := grpc.NewClient(
		"passthrough:///bufconn",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(clientAuth),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
	)
	if err != nil {
		grpcServer.Stop()
		t.Fatalf("new Core Message gRPC client: %v", err)
	}
	remote, err := NewClientForService(messagev1.NewMessageServiceClient(connection), "dipole-core")
	if err != nil {
		_ = connection.Close()
		grpcServer.Stop()
		t.Fatalf("new Core Message adapter: %v", err)
	}
	return remote, func() {
		_ = connection.Close()
		grpcServer.Stop()
	}
}
