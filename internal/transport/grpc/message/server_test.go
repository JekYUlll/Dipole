package messagegrpc

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	messagev1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/message/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type stubMessageApplication struct {
	sendDirect func(senderUUID, targetUUID, content, clientMessageID string) (*model.Message, error)
	listGroup  func(currentUserUUID, groupUUID string, cursor uint, limit int, after bool) ([]*model.Message, error)
}

func (s *stubMessageApplication) SendDirectMessage(senderUUID, targetUUID, content, clientMessageID string) (*model.Message, error) {
	return s.sendDirect(senderUUID, targetUUID, content, clientMessageID)
}

func (s *stubMessageApplication) SendGroupMessage(string, string, string, string) (*model.Message, []string, error) {
	return nil, nil, nil
}

func (s *stubMessageApplication) SendDirectFileMessage(string, string, string, string) (*model.Message, error) {
	return nil, nil
}

func (s *stubMessageApplication) SendGroupFileMessage(string, string, string, string) (*model.Message, []string, error) {
	return nil, nil, nil
}

func (s *stubMessageApplication) ListDirectMessages(string, string, uint, int) ([]*model.Message, error) {
	return nil, nil
}

func (s *stubMessageApplication) ListGroupMessages(userUUID, groupUUID string, beforeID uint, limit int) ([]*model.Message, error) {
	return s.listGroup(userUUID, groupUUID, beforeID, limit, false)
}

func (s *stubMessageApplication) ListGroupMessagesAfter(userUUID, groupUUID string, afterID uint, limit int) ([]*model.Message, error) {
	return s.listGroup(userUUID, groupUUID, afterID, limit, true)
}

func (s *stubMessageApplication) ListOfflineMessages(string, uint, int) ([]*model.Message, error) {
	return nil, nil
}

func TestServerSendDirectTextOverBufconn(t *testing.T) {
	sentAt := time.Date(2026, time.August, 26, 12, 30, 0, 0, time.UTC)
	application := &stubMessageApplication{
		sendDirect: func(senderUUID, targetUUID, content, clientMessageID string) (*model.Message, error) {
			if senderUUID != "U100" || targetUUID != "U200" || content != "hello" || clientMessageID != "C100" {
				t.Fatalf("unexpected command: sender=%q target=%q content=%q client=%q", senderUUID, targetUUID, content, clientMessageID)
			}
			return &model.Message{ID: 42, UUID: "M42", ClientMessageID: "C100", SenderUUID: senderUUID, TargetUUID: targetUUID, Content: content, SentAt: sentAt}, nil
		},
		listGroup: emptyGroupList,
	}
	client := newBufconnClient(t, application)

	response, err := client.SendDirectText(context.Background(), &messagev1.SendDirectTextRequest{
		Context:         &messagev1.InvocationContext{PrincipalUserId: " U100 ", DeviceId: "web", RequestId: "R1"},
		TargetUserId:    "U200",
		Content:         "hello",
		ClientMessageId: "C100",
	})
	if err != nil {
		t.Fatalf("send direct text: %v", err)
	}
	if response.GetMessage().GetServerMessageId() != "M42" || response.GetMessage().GetId() != 42 {
		t.Fatalf("unexpected response: %+v", response.GetMessage())
	}
	if !response.GetMessage().GetSentAt().AsTime().Equal(sentAt) {
		t.Fatalf("unexpected sent_at: %v", response.GetMessage().GetSentAt())
	}
}

func TestServerRejectsMissingPrincipal(t *testing.T) {
	application := &stubMessageApplication{
		sendDirect: func(string, string, string, string) (*model.Message, error) {
			t.Fatal("application must not be called without a principal")
			return nil, nil
		},
		listGroup: emptyGroupList,
	}
	client := newBufconnClient(t, application)

	_, err := client.SendDirectText(context.Background(), &messagev1.SendDirectTextRequest{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

func TestServerDispatchesAfterCursor(t *testing.T) {
	application := &stubMessageApplication{
		sendDirect: func(string, string, string, string) (*model.Message, error) { return nil, nil },
		listGroup: func(userUUID, groupUUID string, cursor uint, limit int, after bool) ([]*model.Message, error) {
			if userUUID != "U100" || groupUUID != "G1" || cursor != 77 || limit != 25 || !after {
				t.Fatalf("unexpected history query: user=%q group=%q cursor=%d limit=%d after=%t", userUUID, groupUUID, cursor, limit, after)
			}
			return []*model.Message{{ID: 78, UUID: "M78"}, nil, {ID: 79, UUID: "M79"}}, nil
		},
	}
	client := newBufconnClient(t, application)

	response, err := client.ListGroupHistory(context.Background(), &messagev1.ListGroupHistoryRequest{
		Context:  &messagev1.InvocationContext{PrincipalUserId: "U100"},
		GroupId:  "G1",
		Cursor:   &messagev1.ListGroupHistoryRequest_AfterId{AfterId: 77},
		PageSize: 25,
	})
	if err != nil {
		t.Fatalf("list group history: %v", err)
	}
	if len(response.GetMessages()) != 2 || response.GetFirstId() != 78 || response.GetLastId() != 79 {
		t.Fatalf("unexpected history response: %+v", response)
	}
}

func TestServerMapsIdempotencyConflict(t *testing.T) {
	application := &stubMessageApplication{
		sendDirect: func(string, string, string, string) (*model.Message, error) {
			return nil, applicationPort.ErrMessageIdempotencyConflict
		},
		listGroup: emptyGroupList,
	}
	client := newBufconnClient(t, application)

	_, err := client.SendDirectText(context.Background(), &messagev1.SendDirectTextRequest{
		Context: &messagev1.InvocationContext{PrincipalUserId: "U100"},
	})
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("expected AlreadyExists, got %v", err)
	}
}

func TestClientPreservesDomainErrorAcrossRPC(t *testing.T) {
	application := &stubMessageApplication{
		sendDirect: func(string, string, string, string) (*model.Message, error) {
			return nil, applicationPort.ErrMessageFriendRequired
		},
		listGroup: emptyGroupList,
	}
	remote, err := NewClient(newBufconnClient(t, application))
	if err != nil {
		t.Fatalf("new remote client: %v", err)
	}

	_, err = remote.SendDirectMessage("U100", "U200", "hello", "C100")
	if !errors.Is(err, applicationPort.ErrMessageFriendRequired) {
		t.Fatalf("expected friendship domain error, got %v", err)
	}
}

func newBufconnClient(t *testing.T, application *stubMessageApplication) messagev1.MessageServiceClient {
	t.Helper()
	serverAdapter, err := NewServer(application)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	messagev1.RegisterMessageServiceServer(grpcServer, serverAdapter)
	go func() {
		_ = grpcServer.Serve(listener)
	}()
	t.Cleanup(grpcServer.Stop)

	connection, err := grpc.NewClient(
		"passthrough:///bufconn",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
	)
	if err != nil {
		t.Fatalf("new grpc client: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	return messagev1.NewMessageServiceClient(connection)
}

func emptyGroupList(string, string, uint, int, bool) ([]*model.Message, error) {
	return nil, nil
}
