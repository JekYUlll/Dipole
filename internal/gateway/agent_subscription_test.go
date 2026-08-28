package gateway

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/platform/correlation"
	agentv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/agent/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type agentSubscriptionRPCStub struct {
	listRequest   *agentv1.ListEventSubscriptionsRequest
	revokeRequest *agentv1.RevokeEventSubscriptionRequest
	listError     error
	nilList       bool
}

func (s *agentSubscriptionRPCStub) ListEventSubscriptions(_ context.Context, request *agentv1.ListEventSubscriptionsRequest, _ ...grpc.CallOption) (*agentv1.ListEventSubscriptionsResponse, error) {
	s.listRequest = request
	if s.listError != nil {
		return nil, s.listError
	}
	if s.nilList {
		return nil, nil
	}
	return &agentv1.ListEventSubscriptionsResponse{
		Subscriptions: []*agentv1.AgentEventSubscription{{
			SubscriptionId: "SUB-1", DefinitionId: "DEF-1", DefinitionVersion: 7,
			TenantId: "dipole", AgentId: "UAI", EventType: "message.created",
			ResourceType: "conversation", ResourceId: "group:G123",
			FilterKind: "message_contains_any", FilterJson: []byte(`{"terms":["事故","延期"]}`),
			Status: "active", CreatedById: "U100", CreatedAtUnixMs: 1_000, UpdatedAtUnixMs: 2_000,
		}},
		NextCursor: "SUB-1",
	}, nil
}

func (s *agentSubscriptionRPCStub) RevokeEventSubscription(_ context.Context, request *agentv1.RevokeEventSubscriptionRequest, _ ...grpc.CallOption) (*agentv1.AgentEventSubscription, error) {
	s.revokeRequest = request
	return &agentv1.AgentEventSubscription{
		SubscriptionId: request.GetSubscriptionId(), DefinitionId: "DEF-1", DefinitionVersion: 7,
		TenantId: "dipole", AgentId: "UAI", EventType: "message.created",
		ResourceType: "conversation", ResourceId: "group:G123", FilterKind: "all", FilterJson: []byte(`{}`),
		Status: "revoked", CreatedById: "U100", RevokedById: "U100", RevokeReason: request.GetReason(),
		CreatedAtUnixMs: 1_000, UpdatedAtUnixMs: 3_000, RevokedAtUnixMs: 3_000,
	}, nil
}

func TestAgentSubscriptionControlClientBindsTrustedOwnerAndCorrelation(t *testing.T) {
	rpc := &agentSubscriptionRPCStub{}
	client, err := NewAgentSubscriptionControlClient(rpc, "dipole", time.Second)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	ctx := correlation.WithContext(context.Background(), correlation.IDs{RequestID: "REQ-1", TraceID: "TRACE-1"})
	page, err := client.List(ctx, "U100", "SUB-0", 20)
	if err != nil {
		t.Fatalf("list subscriptions: %v", err)
	}
	requestContext := rpc.listRequest.GetContext()
	if requestContext.GetCallerService() != "dipole-gateway" || requestContext.GetPrincipalUserId() != "U100" ||
		requestContext.GetRequestId() != "REQ-1" || requestContext.GetTraceId() != "TRACE-1" ||
		rpc.listRequest.GetTenantId() != "dipole" || rpc.listRequest.GetAfterSubscriptionId() != "SUB-0" || rpc.listRequest.GetLimit() != 20 {
		t.Fatalf("unexpected trusted list request: %+v", rpc.listRequest)
	}
	if len(page.Subscriptions) != 1 || page.NextCursor != "SUB-1" || page.Subscriptions[0].Filter.Terms[1] != "延期" {
		t.Fatalf("unexpected page: %+v", page)
	}
	item, err := client.Revoke(ctx, "U100", "SUB-1", "project archived")
	if err != nil || item.Status != "revoked" || rpc.revokeRequest.GetReason() != "project archived" {
		t.Fatalf("revoke subscription: item=%+v request=%+v err=%v", item, rpc.revokeRequest, err)
	}
}

func TestAgentSubscriptionControlClientMapsSafeHTTPStatus(t *testing.T) {
	client, _ := NewAgentSubscriptionControlClient(&agentSubscriptionRPCStub{listError: status.Error(codes.PermissionDenied, "internal detail")}, "dipole", time.Second)
	_, err := client.List(context.Background(), "U100", "", 20)
	if !errors.Is(err, ErrAgentSubscriptionDenied) || AgentSubscriptionHTTPStatus(err) != 403 || err.Error() == "internal detail" {
		t.Fatalf("unexpected mapped error: %v", err)
	}
}

func TestAgentSubscriptionControlClientRejectsEmptyRPCResponse(t *testing.T) {
	client, _ := NewAgentSubscriptionControlClient(&agentSubscriptionRPCStub{nilList: true}, "dipole", time.Second)
	if _, err := client.List(context.Background(), "U100", "", 20); !errors.Is(err, ErrAgentSubscriptionUnavailable) {
		t.Fatalf("expected empty RPC response to fail closed, got %v", err)
	}
}
