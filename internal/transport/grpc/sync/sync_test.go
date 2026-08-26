package syncgrpc

import (
	"context"
	"net"
	"testing"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	syncv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/sync/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type stubSyncApplication struct {
	t *testing.T
}

func (s stubSyncApplication) List(userUUID string, afterSeq uint64, limit int) (*application.SyncPage, error) {
	if userUUID != "U1" || afterSeq != 100 || limit != 200 {
		s.t.Fatalf("unexpected sync query: user=%q after=%d limit=%d", userUUID, afterSeq, limit)
	}
	return &application.SyncPage{
		Items: []*model.SyncMessage{
			{SyncSeq: 101, ConversationKey: "direct:U1:U2", Message: &model.Message{ID: 7, UUID: "M7", Seq: 8, SenderUUID: "U2"}},
			nil,
		},
		NextSeq: 101,
		HasMore: true,
	}, nil
}

func TestRemoteClientImplementsSyncApplication(t *testing.T) {
	rpc := newBufconnRPCClient(t, stubSyncApplication{t: t})
	client, err := NewClient(rpc)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	page, err := client.List("U1", 100, 500)
	if err != nil {
		t.Fatalf("list sync messages: %v", err)
	}
	if page.NextSeq != 101 || !page.HasMore || len(page.Items) != 1 {
		t.Fatalf("unexpected page: %+v", page)
	}
	if page.Items[0].Message == nil || page.Items[0].Message.UUID != "M7" || page.Items[0].Message.Seq != 8 {
		t.Fatalf("unexpected message mapping: %+v", page.Items[0])
	}
}

func TestServerRejectsMissingPrincipal(t *testing.T) {
	rpc := newBufconnRPCClient(t, stubSyncApplication{t: t})
	_, err := rpc.ListSyncMessages(context.Background(), &syncv1.ListSyncMessagesRequest{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

func newBufconnRPCClient(t *testing.T, application stubSyncApplication) syncv1.SyncQueryServiceClient {
	t.Helper()
	adapter, err := NewServer(application)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	syncv1.RegisterSyncQueryServiceServer(server, adapter)
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
	return syncv1.NewSyncQueryServiceClient(connection)
}
