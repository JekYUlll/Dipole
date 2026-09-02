package gateway

import (
	"context"
	"errors"
	"testing"
	"time"

	agentv1 "github.com/JekYUlll/Dipole/api/gen/go/agent/v1"
	"github.com/JekYUlll/Dipole/internal/platform/correlation"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type agentSubscriptionRPCStub struct {
	createRequest     *agentv1.CreateEventSubscriptionRequest
	optionsRequest    *agentv1.ListEligibleSubscriptionConversationsRequest
	listRequest       *agentv1.ListEventSubscriptionsRequest
	revokeRequest     *agentv1.RevokeEventSubscriptionRequest
	listError         error
	nilList           bool
	definitionRequest *agentv1.ListAgentDefinitionsRequest
}

func (s *agentSubscriptionRPCStub) CreateEventSubscription(_ context.Context, request *agentv1.CreateEventSubscriptionRequest, _ ...grpc.CallOption) (*agentv1.AgentEventSubscription, error) {
	s.createRequest = request
	return &agentv1.AgentEventSubscription{
		SubscriptionId: "SUB-CREATED", DefinitionId: request.GetDefinitionId(), DefinitionVersion: request.GetDefinitionVersion(),
		TenantId: request.GetTenantId(), AgentId: "UAI", EventType: request.GetEventType(),
		ResourceType: request.GetResourceType(), ResourceId: request.GetResourceId(), FilterKind: request.GetFilterKind(), FilterJson: request.GetFilterJson(),
		Status: "active", CreatedById: request.GetContext().GetPrincipalUserId(), CreatedAtUnixMs: 1_000, UpdatedAtUnixMs: 1_000,
	}, nil
}

func (s *agentSubscriptionRPCStub) ListEligibleSubscriptionConversations(_ context.Context, request *agentv1.ListEligibleSubscriptionConversationsRequest, _ ...grpc.CallOption) (*agentv1.ListEligibleSubscriptionConversationsResponse, error) {
	s.optionsRequest = request
	return &agentv1.ListEligibleSubscriptionConversationsResponse{Conversations: []*agentv1.AgentSubscriptionConversationOption{{
		ConversationKey: "group:G123", EventType: "message.group.created",
	}}}, nil
}

func (s *agentSubscriptionRPCStub) ListAgentDefinitions(_ context.Context, request *agentv1.ListAgentDefinitionsRequest, _ ...grpc.CallOption) (*agentv1.ListAgentDefinitionsResponse, error) {
	s.definitionRequest = request
	return &agentv1.ListAgentDefinitionsResponse{
		Definitions: []*agentv1.AgentDefinitionCatalogItem{{
			DefinitionId: "DEF-1", Version: 7, AgentId: "UAI", ConversationScopes: []string{"*", "group:G123"},
			ValidFromUnixMs: 1_000, CreatedAtUnixMs: 1_000, UpdatedAtUnixMs: 2_000,
		}},
		NextDefinitionId: "DEF-1", NextVersion: 7,
	}, nil
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
	options, err := client.ListEligibleConversations(ctx, "U100", "DEF-1", 7)
	if err != nil || len(options.Conversations) != 1 || options.Conversations[0].ConversationKey != "group:G123" ||
		rpc.optionsRequest.GetContext().GetPrincipalUserId() != "U100" {
		t.Fatalf("eligible conversations: options=%+v request=%+v err=%v", options, rpc.optionsRequest, err)
	}
	created, err := client.Create(ctx, "U100", AgentSubscriptionCreateInput{
		DefinitionID: "DEF-1", DefinitionVersion: 7, ConversationKey: "group:G123",
		FilterKind: "message_contains_any", Filter: AgentSubscriptionFilter{Terms: []string{"事故", "延期"}},
	})
	if err != nil || created.SubscriptionID != "SUB-CREATED" || rpc.createRequest.GetTenantId() != "dipole" ||
		rpc.createRequest.GetEventType() != "message.group.created" || rpc.createRequest.GetResourceType() != "conversation" {
		t.Fatalf("create subscription: item=%+v request=%+v err=%v", created, rpc.createRequest, err)
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

func TestAgentDefinitionCatalogClientUsesOpaqueCompositeCursor(t *testing.T) {
	rpc := &agentSubscriptionRPCStub{}
	client, _ := NewAgentSubscriptionControlClient(rpc, "dipole", time.Second)
	cursor, err := encodeAgentDefinitionCursor("DEF-0", 3)
	if err != nil {
		t.Fatal(err)
	}
	page, err := client.ListDefinitions(context.Background(), "U100", cursor, 20)
	if err != nil {
		t.Fatalf("list definitions: %v", err)
	}
	if rpc.definitionRequest.GetContext().GetPrincipalUserId() != "U100" || rpc.definitionRequest.GetTenantId() != "dipole" ||
		rpc.definitionRequest.GetAfterDefinitionId() != "DEF-0" || rpc.definitionRequest.GetAfterVersion() != 3 ||
		len(page.Definitions) != 1 || page.Definitions[0].ConversationScopes[1] != "group:G123" || page.NextCursor == "" {
		t.Fatalf("unexpected catalog request=%+v page=%+v", rpc.definitionRequest, page)
	}
	if _, _, err := decodeAgentDefinitionCursor(page.NextCursor); err != nil {
		t.Fatalf("decode response cursor: %v", err)
	}
	if _, err := client.ListDefinitions(context.Background(), "U100", "not-a-cursor", 20); !errors.Is(err, ErrAgentSubscriptionInvalid) {
		t.Fatalf("invalid cursor error = %v", err)
	}
}
