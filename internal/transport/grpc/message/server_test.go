package messagegrpc

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	commonv1 "github.com/JekYUlll/Dipole/api/gen/go/common/v1"
	messagev1 "github.com/JekYUlll/Dipole/api/gen/go/message/v1"
	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/platform/correlation"
	grpccommon "github.com/JekYUlll/Dipole/internal/transport/grpc/common"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type stubMessageApplication struct {
	sendDirect          func(senderUUID, targetUUID, content, clientMessageID string) (*model.Message, error)
	listDirectBeforeSeq func(currentUserUUID, targetUUID string, cursor uint64, limit int) ([]*model.Message, error)
	listDirectAfterSeq  func(currentUserUUID, targetUUID string, cursor uint64, limit int) ([]*model.Message, error)
	listGroup           func(currentUserUUID, groupUUID string, cursor uint, limit int, after bool) ([]*model.Message, error)
	listGroupBeforeSeq  func(currentUserUUID, groupUUID string, cursor uint64, limit int) ([]*model.Message, error)
	commandContext      correlation.IDs
	receipt             func(senderUUID, clientMessageID string) (*applicationPort.MessageCommandReceipt, error)
}

func (s *stubMessageApplication) GetMessageCommandReceipt(senderUUID, clientMessageID string) (*applicationPort.MessageCommandReceipt, error) {
	if s.receipt == nil {
		return &applicationPort.MessageCommandReceipt{Status: applicationPort.MessageCommandReceiptStatusAbsent}, nil
	}
	return s.receipt(senderUUID, clientMessageID)
}

func (s *stubMessageApplication) SendDirectMessageContext(ctx context.Context, senderUUID, targetUUID, content, clientMessageID string) (*model.Message, error) {
	s.commandContext = correlation.FromContext(ctx)
	return s.SendDirectMessage(senderUUID, targetUUID, content, clientMessageID)
}

func (s *stubMessageApplication) SendGroupMessageContext(_ context.Context, senderUUID, groupUUID, content, clientMessageID string) (*model.Message, []string, error) {
	return s.SendGroupMessage(senderUUID, groupUUID, content, clientMessageID)
}

func (s *stubMessageApplication) SendDirectFileMessageContext(_ context.Context, senderUUID, targetUUID, fileUUID, clientMessageID string) (*model.Message, error) {
	return s.SendDirectFileMessage(senderUUID, targetUUID, fileUUID, clientMessageID)
}

func (s *stubMessageApplication) SendGroupFileMessageContext(_ context.Context, senderUUID, groupUUID, fileUUID, clientMessageID string) (*model.Message, []string, error) {
	return s.SendGroupFileMessage(senderUUID, groupUUID, fileUUID, clientMessageID)
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

func (s *stubMessageApplication) ListDirectMessagesBeforeSeq(userUUID, targetUUID string, beforeSeq uint64, limit int) ([]*model.Message, error) {
	if s.listDirectBeforeSeq == nil {
		return nil, nil
	}
	return s.listDirectBeforeSeq(userUUID, targetUUID, beforeSeq, limit)
}

func (s *stubMessageApplication) ListDirectMessagesAfterSeq(userUUID, targetUUID string, afterSeq uint64, limit int) ([]*model.Message, error) {
	if s.listDirectAfterSeq == nil {
		return nil, nil
	}
	return s.listDirectAfterSeq(userUUID, targetUUID, afterSeq, limit)
}

func (s *stubMessageApplication) ListGroupMessages(userUUID, groupUUID string, beforeID uint, limit int) ([]*model.Message, error) {
	return s.listGroup(userUUID, groupUUID, beforeID, limit, false)
}

func (s *stubMessageApplication) ListGroupMessagesBeforeSeq(userUUID, groupUUID string, beforeSeq uint64, limit int) ([]*model.Message, error) {
	if s.listGroupBeforeSeq != nil {
		return s.listGroupBeforeSeq(userUUID, groupUUID, beforeSeq, limit)
	}
	return s.listGroup(userUUID, groupUUID, uint(beforeSeq), limit, false)
}

func (s *stubMessageApplication) ListGroupMessagesAfter(userUUID, groupUUID string, afterID uint, limit int) ([]*model.Message, error) {
	return s.listGroup(userUUID, groupUUID, afterID, limit, true)
}

func (s *stubMessageApplication) ListGroupMessagesAfterSeq(userUUID, groupUUID string, afterSeq uint64, limit int) ([]*model.Message, error) {
	return s.listGroup(userUUID, groupUUID, uint(afterSeq), limit, true)
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
		Context: &commonv1.RequestContext{
			PrincipalUserId: " U100 ", CallerService: "dipole-gateway", RequestId: "grpc-request-1", TraceId: "grpc-trace-1",
		},
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
	if application.commandContext.RequestID != "grpc-request-1" || application.commandContext.TraceID != "grpc-trace-1" {
		t.Fatalf("unexpected command context: %+v", application.commandContext)
	}
}

func TestServerGetsSenderScopedMessageCommandReceipt(t *testing.T) {
	t.Parallel()

	application := &stubMessageApplication{
		sendDirect: func(string, string, string, string) (*model.Message, error) { return nil, nil },
		listGroup:  emptyGroupList,
		receipt: func(senderUUID, clientMessageID string) (*applicationPort.MessageCommandReceipt, error) {
			if senderUUID != "U100" || clientMessageID != "C100" {
				t.Fatalf("unexpected receipt query sender=%q client=%q", senderUUID, clientMessageID)
			}
			return &applicationPort.MessageCommandReceipt{
				Status:  applicationPort.MessageCommandReceiptStatusCommitted,
				Message: &model.Message{UUID: "M100", SenderUUID: senderUUID, ClientMessageID: clientMessageID, Content: "hello"},
			}, nil
		},
	}
	client := newBufconnClient(t, application)
	response, err := client.GetMessageCommandReceipt(context.Background(), &messagev1.GetMessageCommandReceiptRequest{
		Context: grpccommon.RequestContext(" U100 ", "dipole-agent"), ClientMessageId: " C100 ",
	})
	if err != nil || response.GetStatus() != messagev1.MessageCommandReceiptStatus_MESSAGE_COMMAND_RECEIPT_STATUS_COMMITTED || response.GetMessage().GetServerMessageId() != "M100" {
		t.Fatalf("receipt response=%+v err=%v", response, err)
	}
}

func TestServerRejectsInvalidMessageCommandReceiptQuery(t *testing.T) {
	t.Parallel()

	application := &stubMessageApplication{
		sendDirect: func(string, string, string, string) (*model.Message, error) { return nil, nil },
		listGroup:  emptyGroupList,
		receipt: func(string, string) (*applicationPort.MessageCommandReceipt, error) {
			t.Fatal("invalid receipt query reached application")
			return nil, nil
		},
	}
	client := newBufconnClient(t, application)
	_, err := client.GetMessageCommandReceipt(context.Background(), &messagev1.GetMessageCommandReceiptRequest{
		Context: grpccommon.RequestContext("U100", "dipole-agent"),
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("blank client message ID code=%s err=%v", status.Code(err), err)
	}
	_, err = client.GetMessageCommandReceipt(context.Background(), &messagev1.GetMessageCommandReceiptRequest{ClientMessageId: "C100"})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("missing principal code=%s err=%v", status.Code(err), err)
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
		Context:  grpccommon.RequestContext("U100", "dipole-gateway"),
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

func TestClientAndServerPreserveDirectSequenceCursors(t *testing.T) {
	application := &stubMessageApplication{
		sendDirect: func(string, string, string, string) (*model.Message, error) { return nil, nil },
		listGroup:  emptyGroupList,
		listDirectBeforeSeq: func(userUUID, targetUUID string, cursor uint64, limit int) ([]*model.Message, error) {
			if userUUID != "U100" || targetUUID != "U200" || cursor != 41 || limit != 20 {
				t.Fatalf("unexpected direct Seq query: user=%q target=%q cursor=%d limit=%d", userUUID, targetUUID, cursor, limit)
			}
			return []*model.Message{{Seq: 40, UUID: "MD40"}}, nil
		},
		listDirectAfterSeq: func(userUUID, targetUUID string, cursor uint64, limit int) ([]*model.Message, error) {
			if userUUID != "U100" || targetUUID != "U200" || cursor != 41 || limit != 20 {
				t.Fatalf("unexpected direct after Seq query: user=%q target=%q cursor=%d limit=%d", userUUID, targetUUID, cursor, limit)
			}
			return []*model.Message{{Seq: 42, UUID: "MD42"}}, nil
		},
		listGroupBeforeSeq: func(userUUID, groupUUID string, cursor uint64, limit int) ([]*model.Message, error) {
			if userUUID != "U100" || groupUUID != "G1" || cursor != 51 || limit != 25 {
				t.Fatalf("unexpected group Seq query: user=%q group=%q cursor=%d limit=%d", userUUID, groupUUID, cursor, limit)
			}
			return []*model.Message{{Seq: 50, UUID: "MG50"}}, nil
		},
	}
	remote, err := NewClient(newBufconnClient(t, application))
	if err != nil {
		t.Fatalf("new remote client: %v", err)
	}
	direct, err := remote.ListDirectMessagesBeforeSeq("U100", "U200", 41, 20)
	if err != nil || len(direct) != 1 || direct[0].Seq != 40 {
		t.Fatalf("unexpected direct response=%+v err=%v", direct, err)
	}
	directAfter, err := remote.ListDirectMessagesAfterSeq("U100", "U200", 41, 20)
	if err != nil || len(directAfter) != 1 || directAfter[0].Seq != 42 {
		t.Fatalf("unexpected direct after response=%+v err=%v", directAfter, err)
	}
	group, err := remote.ListGroupMessagesBeforeSeq("U100", "G1", 51, 25)
	if err != nil || len(group) != 1 || group[0].Seq != 50 {
		t.Fatalf("unexpected group response=%+v err=%v", group, err)
	}
}

func TestServerRejectsMixedDirectCursorDomains(t *testing.T) {
	application := &stubMessageApplication{
		sendDirect: func(string, string, string, string) (*model.Message, error) { return nil, nil },
		listGroup:  emptyGroupList,
	}
	client := newBufconnClient(t, application)
	beforeSeq := uint64(41)
	_, err := client.ListDirectHistory(context.Background(), &messagev1.ListDirectHistoryRequest{
		Context: grpccommon.RequestContext("U100", "dipole-gateway"), TargetUserId: "U200",
		BeforeId: 20, BeforeSequence: &beforeSeq,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
	afterSeq := uint64(42)
	_, err = client.ListDirectHistory(context.Background(), &messagev1.ListDirectHistoryRequest{
		Context: grpccommon.RequestContext("U100", "dipole-gateway"), TargetUserId: "U200",
		BeforeSequence: &beforeSeq, AfterSequence: &afterSeq,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument for two sequence directions, got %v", err)
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
		Context: grpccommon.RequestContext("U100", "dipole-gateway"),
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

func TestClientGetsMessageCommandReceiptAcrossRPC(t *testing.T) {
	t.Parallel()

	application := &stubMessageApplication{
		sendDirect: func(string, string, string, string) (*model.Message, error) { return nil, nil },
		listGroup:  emptyGroupList,
		receipt: func(senderUUID, clientMessageID string) (*applicationPort.MessageCommandReceipt, error) {
			return &applicationPort.MessageCommandReceipt{
				Status:  applicationPort.MessageCommandReceiptStatusCommitted,
				Message: &model.Message{UUID: "M100", SenderUUID: senderUUID, ClientMessageID: clientMessageID},
			}, nil
		},
	}
	remote, err := NewClientForService(newBufconnClient(t, application), "dipole-agent")
	if err != nil {
		t.Fatalf("new remote client: %v", err)
	}
	receipt, err := remote.GetMessageCommandReceipt("U100", "C100")
	if err != nil || receipt.Status != applicationPort.MessageCommandReceiptStatusCommitted || receipt.Message.UUID != "M100" {
		t.Fatalf("remote receipt=%+v err=%v", receipt, err)
	}
	if _, err := remote.GetMessageCommandReceipt("U100", " "); !errors.Is(err, applicationPort.ErrMessageClientMessageIDInvalid) {
		t.Fatalf("remote invalid receipt query error=%v", err)
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
