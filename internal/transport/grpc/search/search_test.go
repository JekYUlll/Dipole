package searchgrpc

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	commonv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/common/v1"
	searchv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/search/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type searchApplicationStub struct {
	principal string
	text      string
	limit     int
	items     []*model.MessageSearchDocument
	err       error
}

func (s *searchApplicationStub) Search(principal, text string, limit int) ([]*model.MessageSearchDocument, error) {
	s.principal, s.text, s.limit = principal, text, limit
	return s.items, s.err
}

func TestRemoteClientImplementsSearchApplication(t *testing.T) {
	sentAt := time.Date(2026, time.August, 27, 12, 30, 0, 123000000, time.UTC)
	applicationStub := &searchApplicationStub{items: []*model.MessageSearchDocument{nil, {
		MessageUUID: "M1", ConversationKey: "group:G1", MessageSeq: 42, Revision: 3,
		SenderUUID: "U2", MessageType: model.MessageTypeText, Content: "migration plan", SentAt: sentAt,
	}}}
	rpc := newSearchBufconnClient(t, applicationStub)
	client, err := NewClient(rpc)
	if err != nil {
		t.Fatalf("new Search client: %v", err)
	}

	items, err := client.Search("U1", "migration", 25)
	if err != nil || len(items) != 1 {
		t.Fatalf("remote Search: items=%+v err=%v", items, err)
	}
	if applicationStub.principal != "U1" || applicationStub.text != "migration" || applicationStub.limit != 25 {
		t.Fatalf("unexpected request: principal=%q text=%q limit=%d", applicationStub.principal, applicationStub.text, applicationStub.limit)
	}
	if got := items[0]; got.MessageUUID != "M1" || got.ConversationKey != "group:G1" || got.MessageSeq != 42 || got.Revision != 3 || !got.SentAt.Equal(sentAt) {
		t.Fatalf("unexpected result mapping: %+v", got)
	}
}

func TestSearchServerRejectsMissingPrincipalAndNegativePageSize(t *testing.T) {
	rpc := newSearchBufconnClient(t, &searchApplicationStub{})
	_, err := rpc.SearchMessages(context.Background(), &searchv1.SearchMessagesRequest{
		Context: &commonv1.RequestContext{CallerService: "dipole-gateway"}, Query: "migration",
	})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected missing principal rejection, got %v", err)
	}
	_, err = rpc.SearchMessages(context.Background(), &searchv1.SearchMessagesRequest{
		Context: &commonv1.RequestContext{CallerService: "dipole-gateway", PrincipalUserId: "U1"}, Query: "migration", PageSize: -1,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected negative page size rejection, got %v", err)
	}
}

func TestSearchServerMapsRequiredTextAndInfrastructureErrors(t *testing.T) {
	applicationStub := &searchApplicationStub{err: application.ErrSearchTextRequired}
	rpc := newSearchBufconnClient(t, applicationStub)
	request := &searchv1.SearchMessagesRequest{
		Context: &commonv1.RequestContext{CallerService: "dipole-gateway", PrincipalUserId: "U1"},
	}
	_, err := rpc.SearchMessages(context.Background(), request)
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected required query rejection, got %v", err)
	}
	applicationStub.err = errors.New("Elasticsearch unavailable")
	request.Query = "migration"
	_, err = rpc.SearchMessages(context.Background(), request)
	if status.Code(err) != codes.Internal || status.Convert(err).Message() != "search service failed" {
		t.Fatalf("expected bounded infrastructure error, got %v", err)
	}
}

func TestSearchClientRequiresCallerService(t *testing.T) {
	if _, err := NewClientForService(newSearchBufconnClient(t, &searchApplicationStub{}), " "); err == nil {
		t.Fatal("expected empty caller service to fail")
	}
}

func newSearchBufconnClient(t *testing.T, applicationStub *searchApplicationStub) searchv1.SearchServiceClient {
	t.Helper()
	adapter, err := NewServer(applicationStub)
	if err != nil {
		t.Fatalf("new Search server: %v", err)
	}
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	searchv1.RegisterSearchServiceServer(server, adapter)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	connection, err := grpc.NewClient(
		"passthrough:///bufconn",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
	)
	if err != nil {
		t.Fatalf("new Search bufconn client: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	return searchv1.NewSearchServiceClient(connection)
}
