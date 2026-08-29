package gateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/platform/cache"
	coreauth "github.com/JekYUlll/Dipole/internal/services/core/domain/auth"
)

type gatewayMessageStub struct{}

func (gatewayMessageStub) SendDirectMessage(string, string, string, string) (*model.Message, error) {
	return nil, nil
}
func (gatewayMessageStub) SendGroupMessage(string, string, string, string) (*model.Message, []string, error) {
	return nil, nil, nil
}
func (gatewayMessageStub) SendDirectFileMessage(string, string, string, string) (*model.Message, error) {
	return nil, nil
}
func (gatewayMessageStub) SendGroupFileMessage(string, string, string, string) (*model.Message, []string, error) {
	return nil, nil, nil
}
func (gatewayMessageStub) ListDirectMessages(string, string, uint, int) ([]*model.Message, error) {
	return nil, nil
}
func (gatewayMessageStub) ListDirectMessagesBeforeSeq(string, string, uint64, int) ([]*model.Message, error) {
	return nil, nil
}
func (gatewayMessageStub) ListDirectMessagesAfterSeq(string, string, uint64, int) ([]*model.Message, error) {
	return nil, nil
}
func (gatewayMessageStub) ListGroupMessages(string, string, uint, int) ([]*model.Message, error) {
	return nil, nil
}
func (gatewayMessageStub) ListGroupMessagesBeforeSeq(string, string, uint64, int) ([]*model.Message, error) {
	return nil, nil
}
func (gatewayMessageStub) ListGroupMessagesAfter(string, string, uint, int) ([]*model.Message, error) {
	return nil, nil
}
func (gatewayMessageStub) ListGroupMessagesAfterSeq(string, string, uint64, int) ([]*model.Message, error) {
	return nil, nil
}
func (gatewayMessageStub) ListOfflineMessages(string, uint, int) ([]*model.Message, error) {
	return nil, nil
}

type gatewaySyncStub struct{}

func (gatewaySyncStub) List(string, uint64, int) (*application.SyncPage, error) {
	return &application.SyncPage{}, nil
}
func (gatewaySyncStub) GetCheckpoint(string, string) (*model.DeviceSyncCheckpoint, error) {
	return &model.DeviceSyncCheckpoint{}, nil
}
func (gatewaySyncStub) AdvanceCheckpoint(string, string, uint64) (*model.DeviceSyncCheckpoint, error) {
	return &model.DeviceSyncCheckpoint{}, nil
}
func (gatewaySyncStub) ListGroupCheckpoints(string, string, []string) ([]*model.GroupSyncCheckpoint, error) {
	return nil, nil
}
func (gatewaySyncStub) AdvanceGroupCheckpoint(string, string, string, uint64) (*model.GroupSyncCheckpoint, error) {
	return &model.GroupSyncCheckpoint{}, nil
}

type gatewayCoreStub struct{}

type gatewayLimiterStub struct{}

func newTestGatewayServer(coreTarget string, dependencies Dependencies) (*Server, error) {
	if dependencies.TokenResolver == nil {
		dependencies.TokenResolver = coreauth.NewTokenService()
	}
	return NewServerWithDependencies(coreTarget, dependencies)
}

type gatewaySearchStub struct {
	principal string
	text      string
	limit     int
}

type gatewayAgentTaskStub struct {
	principal  string
	taskID     string
	approvalID string
	decision   string
	reason     string
	requestID  string
	input      any
}

type gatewayAgentMCPStub struct {
	principal string
	taskID    string
	runID     string
	calls     int
}

type gatewayAgentSubscriptionStub struct {
	principal         string
	after             string
	limit             int
	subscriptionID    string
	reason            string
	listCalls         int
	revokeCalls       int
	createCalls       int
	optionsCalls      int
	createInput       AgentSubscriptionCreateInput
	definitionID      string
	definitionVersion uint64
}

type gatewayAgentDefinitionStub struct {
	principal, after string
	limit            int
}

type gatewayAgentMemoryStub struct {
	principal, after, memoryID, reason          string
	content, compactContent                     string
	expectedVersion                             uint32
	limit, listCalls, revokeCalls, correctCalls int
}

func (s *gatewayAgentMemoryStub) List(_ context.Context, principalUUID, after string, limit int) (*AgentMemoryPage, error) {
	s.principal, s.after, s.limit = principalUUID, after, limit
	s.listCalls++
	return &AgentMemoryPage{Memories: []AgentMemory{{MemoryID: "MEM-1", Status: "active", Content: "Owner is Alice"}}, NextCursor: "CURSOR-1"}, nil
}

func (s *gatewayAgentMemoryStub) Revoke(_ context.Context, principalUUID, memoryID, reason string) (*AgentMemory, error) {
	s.principal, s.memoryID, s.reason = principalUUID, memoryID, reason
	s.revokeCalls++
	return &AgentMemory{MemoryID: memoryID, Status: "revoked", RevokedByID: principalUUID, RevokeReason: reason}, nil
}

func (s *gatewayAgentMemoryStub) Correct(_ context.Context, principalUUID, memoryID string, expectedVersion uint32, content, compactContent, reason string) (*AgentMemoryCorrection, error) {
	s.principal, s.memoryID, s.expectedVersion = principalUUID, memoryID, expectedVersion
	s.content, s.compactContent, s.reason = content, compactContent, reason
	s.correctCalls++
	return &AgentMemoryCorrection{
		Previous:  AgentMemory{MemoryID: memoryID, MemoryRootID: memoryID, MemoryVersion: expectedVersion, Status: "revoked"},
		Corrected: AgentMemory{MemoryID: "MEM-2", MemoryRootID: memoryID, MemoryVersion: expectedVersion + 1, SupersedesID: memoryID, Status: "active", Content: content},
	}, nil
}

func (s *gatewayAgentMemoryStub) PromoteCandidate(_ context.Context, principalUUID, candidateID, candidateSHA256, reviewID string) (*AgentMemory, error) {
	return &AgentMemory{MemoryID: "MEM-CAND-1", AgentID: "UAI", MemoryType: "observational", Status: "active", ResourceType: "conversation", ResourceID: "group:G1", Content: "Decision", CompactContent: "Decision", Priority: 60, Provenance: AgentMemoryProvenance{SourceType: "memory_candidate", SourceID: candidateID, Sequence: reviewID}, ValidFromUnixMS: 1700000000000, CreatedAtUnixMS: 1700000000000, MemoryRootID: "MEM-CAND-1", MemoryVersion: 1}, nil
}

func (s *gatewayAgentDefinitionStub) ListDefinitions(_ context.Context, principalUUID, after string, limit int) (*AgentDefinitionCatalogPage, error) {
	s.principal, s.after, s.limit = principalUUID, after, limit
	return &AgentDefinitionCatalogPage{Definitions: []AgentDefinitionCatalogItem{{
		DefinitionID: "DEF-1", Version: 7, AgentID: "UAI", ConversationScopes: []string{"group:G123"},
		ValidFromUnixMS: 1_000, CreatedAtUnixMS: 1_000, UpdatedAtUnixMS: 2_000,
	}}}, nil
}

func (s *gatewayAgentSubscriptionStub) List(_ context.Context, principalUUID, after string, limit int) (*AgentSubscriptionPage, error) {
	s.principal, s.after, s.limit = principalUUID, after, limit
	s.listCalls++
	return &AgentSubscriptionPage{Subscriptions: []AgentSubscription{{
		SubscriptionID: "SUB-1", DefinitionID: "DEF-1", DefinitionVersion: 7, AgentID: "UAI",
		EventType: "message.created", ResourceType: "conversation", ResourceID: "group:G123",
		FilterKind: "all", Filter: AgentSubscriptionFilter{}, Status: "active", CreatedByID: principalUUID,
	}}, NextCursor: "SUB-1"}, nil
}

func (s *gatewayAgentSubscriptionStub) ListEligibleConversations(_ context.Context, principalUUID, definitionID string, definitionVersion uint64) (*AgentSubscriptionConversationOptions, error) {
	s.principal, s.definitionID, s.definitionVersion = principalUUID, definitionID, definitionVersion
	s.optionsCalls++
	return &AgentSubscriptionConversationOptions{Conversations: []AgentSubscriptionConversationOption{{ConversationKey: "group:G123", EventType: "message.group.created"}}}, nil
}

func (s *gatewayAgentSubscriptionStub) Create(_ context.Context, principalUUID string, input AgentSubscriptionCreateInput) (*AgentSubscription, error) {
	s.principal, s.createInput = principalUUID, input
	s.createCalls++
	return &AgentSubscription{
		SubscriptionID: "SUB-CREATED", DefinitionID: input.DefinitionID, DefinitionVersion: input.DefinitionVersion, AgentID: "UAI",
		EventType: "message.group.created", ResourceType: "conversation", ResourceID: input.ConversationKey,
		FilterKind: input.FilterKind, Filter: input.Filter, Status: "active", CreatedByID: principalUUID,
	}, nil
}

func (s *gatewayAgentSubscriptionStub) Revoke(_ context.Context, principalUUID, subscriptionID, reason string) (*AgentSubscription, error) {
	s.principal, s.subscriptionID, s.reason = principalUUID, subscriptionID, reason
	s.revokeCalls++
	return &AgentSubscription{SubscriptionID: subscriptionID, DefinitionID: "DEF-1", DefinitionVersion: 7, AgentID: "UAI",
		EventType: "message.created", ResourceType: "conversation", ResourceID: "group:G123", FilterKind: "all",
		Filter: AgentSubscriptionFilter{}, Status: "revoked", CreatedByID: principalUUID, RevokedByID: principalUUID, RevokeReason: reason}, nil
}

func (s *gatewayAgentMCPStub) ServeMCP(writer http.ResponseWriter, _ *http.Request, principalUUID, taskUUID, runUUID string) {
	s.principal, s.taskID, s.runID = principalUUID, taskUUID, runUUID
	s.calls++
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Mcp-Session-Id", "S1")
	writer.WriteHeader(http.StatusAccepted)
	_, _ = writer.Write([]byte("event: message\ndata: accepted\n\n"))
}

type gatewayAgentMCPLimiterStub struct {
	allowedCalls int
	retryAfter   time.Duration
	principals   []string
}

func (s *gatewayAgentMCPLimiterStub) AllowAgentMCP(principalUUID string) (bool, time.Duration) {
	s.principals = append(s.principals, principalUUID)
	return len(s.principals) <= s.allowedCalls, s.retryAfter
}

func (s *gatewayAgentTaskStub) GetTask(_ context.Context, principalUUID, taskUUID string) (*AgentTaskControlResult, error) {
	s.principal, s.taskID = principalUUID, taskUUID
	return agentControlJSON(http.StatusOK, map[string]any{"taskId": taskUUID, "status": "running"}), nil
}

func (s *gatewayAgentTaskStub) GetTimeline(_ context.Context, principalUUID, taskUUID, after string, limit int) (*AgentTaskControlResult, error) {
	s.principal, s.taskID = principalUUID, taskUUID
	return agentControlJSON(http.StatusOK, map[string]any{"taskId": taskUUID, "after": after, "limit": limit, "events": []any{}}), nil
}

func (s *gatewayAgentTaskStub) CancelTask(_ context.Context, principalUUID, taskUUID, reason string) (*AgentTaskControlResult, error) {
	s.principal, s.taskID, s.reason = principalUUID, taskUUID, reason
	return agentControlJSON(http.StatusAccepted, map[string]any{"status": "cancellation_requested"}), nil
}

func (s *gatewayAgentTaskStub) ResolveApproval(_ context.Context, principalUUID, taskUUID, approvalUUID, decision string) (*AgentTaskControlResult, error) {
	s.principal, s.taskID, s.approvalID, s.decision = principalUUID, taskUUID, approvalUUID, decision
	return agentControlJSON(http.StatusAccepted, map[string]any{"status": "resolution_requested"}), nil
}

func (s *gatewayAgentTaskStub) ProvideInput(_ context.Context, principalUUID, taskUUID, requestUUID string, value any) (*AgentTaskControlResult, error) {
	s.principal, s.taskID, s.requestID, s.input = principalUUID, taskUUID, requestUUID, value
	return agentControlJSON(http.StatusAccepted, map[string]any{"status": "input_accepted"}), nil
}

func agentControlJSON(status int, value any) *AgentTaskControlResult {
	body, _ := json.Marshal(value)
	return &AgentTaskControlResult{StatusCode: status, Body: body, ContentType: "application/json"}
}

func (s *gatewaySearchStub) Search(principal, text string, limit int) ([]*model.MessageSearchDocument, error) {
	s.principal, s.text, s.limit = principal, text, limit
	return []*model.MessageSearchDocument{{
		MessageUUID: "M1", ConversationKey: "direct:U1:U2", MessageSeq: 1, Revision: 1,
		SenderUUID: "U2", MessageType: model.MessageTypeText, Content: text, SentAt: time.Unix(1, 0),
	}}, nil
}

func (gatewayLimiterStub) AllowMessageSend(string) (bool, time.Duration) { return true, 0 }

func (gatewayCoreStub) GetUserByUUID(userUUID string) (*model.User, error) {
	return &model.User{UUID: userUUID}, nil
}
func (gatewayCoreStub) CanSendDirectMessage(string, string) (bool, error)         { return true, nil }
func (gatewayCoreStub) GetGroupByUUID(string) (*model.Group, error)               { return nil, nil }
func (gatewayCoreStub) GetGroupMember(string, string) (*model.GroupMember, error) { return nil, nil }
func (gatewayCoreStub) ListGroupMembers(string) ([]*model.GroupMember, error)     { return nil, nil }
func (gatewayCoreStub) GetOwnedFile(string, string) (*model.UploadedFile, error)  { return nil, nil }
func (gatewayCoreStub) ListSearchConversationKeys(string) ([]string, error)       { return nil, nil }

var _ application.MessageApplication = gatewayMessageStub{}
var _ application.CoreCapability = gatewayCoreStub{}
var _ application.SearchApplication = (*gatewaySearchStub)(nil)

func TestGatewayOwnsHealthAndProxiesCoreHTTP(t *testing.T) {
	core := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-Core-Path", request.URL.Path)
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte("core-response"))
	}))
	defer core.Close()
	gateway, err := newTestGatewayServer(core.URL, Dependencies{
		Messages: gatewayMessageStub{},
		Core:     gatewayCoreStub{},
		Limiter:  gatewayLimiterStub{},
	})
	if err != nil {
		t.Fatalf("new gateway: %v", err)
	}

	health := httptest.NewRecorder()
	gateway.Engine().ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/health", nil))
	if health.Code != http.StatusOK || health.Header().Get("X-Core-Path") != "" {
		t.Fatalf("gateway health should stay local: code=%d headers=%v", health.Code, health.Header())
	}

	public := httptest.NewServer(gateway.Engine())
	defer public.Close()
	request, err := http.NewRequest(http.MethodPost, public.URL+"/api/v1/contacts?limit=20", nil)
	if err != nil {
		t.Fatalf("new proxied request: %v", err)
	}
	proxied, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("proxy request: %v", err)
	}
	defer proxied.Body.Close()
	body, err := io.ReadAll(proxied.Body)
	if err != nil {
		t.Fatalf("read proxy response: %v", err)
	}
	if proxied.StatusCode != http.StatusCreated || proxied.Header.Get("X-Core-Path") != "/api/v1/contacts" || string(body) != "core-response" {
		t.Fatalf("unexpected proxy response: code=%d headers=%v body=%q", proxied.StatusCode, proxied.Header, string(body))
	}
}

func TestGatewayOwnsMessageAndSyncReadRoutes(t *testing.T) {
	core := httptest.NewServer(http.NotFoundHandler())
	defer core.Close()
	gateway, err := newTestGatewayServer(core.URL, Dependencies{
		Messages: gatewayMessageStub{}, Sync: gatewaySyncStub{}, Core: gatewayCoreStub{}, Limiter: gatewayLimiterStub{},
	})
	if err != nil {
		t.Fatalf("new gateway: %v", err)
	}

	routes := map[string]bool{}
	for _, route := range gateway.Engine().Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, route := range []string{
		http.MethodGet + " /api/v1/messages/offline",
		http.MethodGet + " /api/v1/messages/direct/:target_uuid",
		http.MethodGet + " /api/v1/messages/group/:group_uuid",
		http.MethodGet + " /api/v1/sync",
		http.MethodGet + " /api/v1/sync/checkpoint",
		http.MethodPatch + " /api/v1/sync/checkpoint",
		http.MethodPost + " /api/v1/sync/comparison",
		http.MethodGet + " /api/v1/sync/groups/checkpoints",
		http.MethodPatch + " /api/v1/sync/groups/:group_uuid/checkpoint",
	} {
		if !routes[route] {
			t.Fatalf("gateway route missing: %s", route)
		}
	}
}

func TestGatewayRequiresRemoteDependencies(t *testing.T) {
	if _, err := newTestGatewayServer("http://127.0.0.1:8081", Dependencies{Core: gatewayCoreStub{}, Limiter: gatewayLimiterStub{}}); err == nil {
		t.Fatal("expected missing message application to fail")
	}
	if _, err := newTestGatewayServer("http://127.0.0.1:8081", Dependencies{Messages: gatewayMessageStub{}, Limiter: gatewayLimiterStub{}}); err == nil {
		t.Fatal("expected missing core capability to fail")
	}
	if _, err := newTestGatewayServer("not-a-url", Dependencies{Messages: gatewayMessageStub{}, Core: gatewayCoreStub{}, Limiter: gatewayLimiterStub{}}); err == nil {
		t.Fatal("expected invalid core target to fail")
	}
	if _, err := newTestGatewayServer("ftp://127.0.0.1", Dependencies{Messages: gatewayMessageStub{}, Core: gatewayCoreStub{}, Limiter: gatewayLimiterStub{}}); err == nil {
		t.Fatal("expected unsupported core target scheme to fail")
	}
}

func TestGatewayOwnsAuthenticatedSearchRoute(t *testing.T) {
	t.Chdir("../../../..")
	t.Setenv("DIPOLE_CONFIG_FILE", "configs/config.dist.yaml")
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer mr.Close()
	previousRedis := cache.RDB
	cache.RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = cache.RDB.Close()
		cache.RDB = previousRedis
	})
	proxied := 0
	core := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		proxied++
		writer.WriteHeader(http.StatusTeapot)
	}))
	defer core.Close()
	search := &gatewaySearchStub{}
	gateway, err := newTestGatewayServer(core.URL, Dependencies{
		Messages: gatewayMessageStub{}, Core: gatewayCoreStub{}, Search: search, Limiter: gatewayLimiterStub{},
	})
	if err != nil {
		t.Fatalf("new gateway: %v", err)
	}

	unauthorized := httptest.NewRecorder()
	gateway.Engine().ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/messages/search?q=migration", nil))
	if unauthorized.Code != http.StatusUnauthorized || proxied != 0 {
		t.Fatalf("unauthorized Search: code=%d proxied=%d", unauthorized.Code, proxied)
	}
	token, err := coreauth.NewTokenService().Issue(&model.User{UUID: "U1"})
	if err != nil {
		t.Fatalf("issue gateway test token: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/messages/search?q=migration&limit=12", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	gateway.Engine().ServeHTTP(response, request)
	if response.Code != http.StatusOK || proxied != 0 || search.principal != "U1" || search.text != "migration" || search.limit != 12 {
		t.Fatalf("owned Search: code=%d proxied=%d principal=%q text=%q limit=%d body=%s", response.Code, proxied, search.principal, search.text, search.limit, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"message_id":"M1"`) {
		t.Fatalf("unexpected Search response: %s", response.Body.String())
	}
}

func TestGatewayOwnsAuthenticatedAgentSubscriptionListAndRevoke(t *testing.T) {
	t.Chdir("../../../..")
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer mr.Close()
	previousRedis := cache.RDB
	cache.RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = cache.RDB.Close()
		cache.RDB = previousRedis
	})
	proxied := 0
	core := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		proxied++
		writer.WriteHeader(http.StatusTeapot)
	}))
	defer core.Close()
	subscriptions := &gatewayAgentSubscriptionStub{}
	gateway, err := newTestGatewayServer(core.URL, Dependencies{
		Messages: gatewayMessageStub{}, Core: gatewayCoreStub{}, AgentSubscriptions: subscriptions, Limiter: gatewayLimiterStub{},
	})
	if err != nil {
		t.Fatalf("new gateway: %v", err)
	}

	unauthorized := httptest.NewRecorder()
	gateway.Engine().ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/agent/subscriptions", nil))
	if unauthorized.Code != http.StatusUnauthorized || proxied != 0 || subscriptions.listCalls != 0 {
		t.Fatalf("unauthorized list: code=%d proxied=%d calls=%d", unauthorized.Code, proxied, subscriptions.listCalls)
	}
	token, err := coreauth.NewTokenService().Issue(&model.User{UUID: "U100"})
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/agent/subscriptions?after=SUB-0&limit=20", nil)
	listRequest.Header.Set("Authorization", "Bearer "+token)
	listResponse := httptest.NewRecorder()
	gateway.Engine().ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK || proxied != 0 || subscriptions.principal != "U100" || subscriptions.after != "SUB-0" || subscriptions.limit != 20 {
		t.Fatalf("list: code=%d proxied=%d stub=%+v body=%s", listResponse.Code, proxied, subscriptions, listResponse.Body.String())
	}
	if !strings.Contains(listResponse.Body.String(), `"subscriptionId":"SUB-1"`) || !strings.Contains(listResponse.Body.String(), `"nextCursor":"SUB-1"`) {
		t.Fatalf("unexpected list response: %s", listResponse.Body.String())
	}

	optionsRequest := httptest.NewRequest(http.MethodGet, "/api/v1/agent/subscriptions/options?definitionId=DEF-1&definitionVersion=7", nil)
	optionsRequest.Header.Set("Authorization", "Bearer "+token)
	optionsResponse := httptest.NewRecorder()
	gateway.Engine().ServeHTTP(optionsResponse, optionsRequest)
	if optionsResponse.Code != http.StatusOK || subscriptions.optionsCalls != 1 || subscriptions.principal != "U100" ||
		subscriptions.definitionID != "DEF-1" || subscriptions.definitionVersion != 7 ||
		!strings.Contains(optionsResponse.Body.String(), `"conversationKey":"group:G123"`) {
		t.Fatalf("options: code=%d stub=%+v body=%s", optionsResponse.Code, subscriptions, optionsResponse.Body.String())
	}

	createRequest := httptest.NewRequest(http.MethodPost, "/api/v1/agent/subscriptions", strings.NewReader(`{"definitionId":"DEF-1","definitionVersion":7,"conversationKey":"group:G123","filterKind":"message_contains_any","filter":{"terms":["事故","延期"]}}`))
	createRequest.Header.Set("Authorization", "Bearer "+token)
	createRequest.Header.Set("Content-Type", "application/json")
	createResponse := httptest.NewRecorder()
	gateway.Engine().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusOK || subscriptions.createCalls != 1 || subscriptions.createInput.ConversationKey != "group:G123" ||
		!strings.Contains(createResponse.Body.String(), `"subscriptionId":"SUB-CREATED"`) {
		t.Fatalf("create: code=%d stub=%+v body=%s", createResponse.Code, subscriptions, createResponse.Body.String())
	}

	revokeRequest := httptest.NewRequest(http.MethodPost, "/api/v1/agent/subscriptions/SUB-1/revoke", strings.NewReader(`{"reason":"project archived"}`))
	revokeRequest.Header.Set("Authorization", "Bearer "+token)
	revokeRequest.Header.Set("Content-Type", "application/json")
	revokeResponse := httptest.NewRecorder()
	gateway.Engine().ServeHTTP(revokeResponse, revokeRequest)
	if revokeResponse.Code != http.StatusOK || subscriptions.subscriptionID != "SUB-1" || subscriptions.reason != "project archived" || subscriptions.revokeCalls != 1 {
		t.Fatalf("revoke: code=%d stub=%+v body=%s", revokeResponse.Code, subscriptions, revokeResponse.Body.String())
	}
}

func TestGatewayOwnsAuthenticatedAgentMemoryControl(t *testing.T) {
	t.Chdir("../../../..")
	mr, _ := miniredis.Run()
	defer mr.Close()
	previousRedis := cache.RDB
	cache.RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = cache.RDB.Close(); cache.RDB = previousRedis })
	core := httptest.NewServer(http.NotFoundHandler())
	defer core.Close()
	memories := &gatewayAgentMemoryStub{}
	gateway, err := newTestGatewayServer(core.URL, Dependencies{Messages: gatewayMessageStub{}, Core: gatewayCoreStub{}, AgentMemories: memories, Limiter: gatewayLimiterStub{}})
	if err != nil {
		t.Fatal(err)
	}
	unauthorized := httptest.NewRecorder()
	gateway.Engine().ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/agent/memories", nil))
	if unauthorized.Code != http.StatusUnauthorized || memories.listCalls != 0 {
		t.Fatalf("unauthorized list code=%d calls=%d", unauthorized.Code, memories.listCalls)
	}
	token, _ := coreauth.NewTokenService().Issue(&model.User{UUID: "U100"})
	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/agent/memories?after=CURSOR-0&limit=20", nil)
	listRequest.Header.Set("Authorization", "Bearer "+token)
	listResponse := httptest.NewRecorder()
	gateway.Engine().ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK || memories.principal != "U100" || memories.after != "CURSOR-0" || memories.limit != 20 ||
		!strings.Contains(listResponse.Body.String(), `"memoryId":"MEM-1"`) {
		t.Fatalf("list code=%d stub=%+v body=%s", listResponse.Code, memories, listResponse.Body.String())
	}
	forged := httptest.NewRequest(http.MethodPost, "/api/v1/agent/memories/MEM-1/revoke", strings.NewReader(`{"reason":"outdated","principalUserId":"U999"}`))
	forged.Header.Set("Authorization", "Bearer "+token)
	forged.Header.Set("Content-Type", "application/json")
	forgedResponse := httptest.NewRecorder()
	gateway.Engine().ServeHTTP(forgedResponse, forged)
	if forgedResponse.Code != http.StatusBadRequest || memories.revokeCalls != 0 {
		t.Fatalf("forged revoke code=%d calls=%d", forgedResponse.Code, memories.revokeCalls)
	}
	revoke := httptest.NewRequest(http.MethodPost, "/api/v1/agent/memories/MEM-1/revoke", strings.NewReader(`{"reason":"outdated"}`))
	revoke.Header.Set("Authorization", "Bearer "+token)
	revoke.Header.Set("Content-Type", "application/json")
	revokeResponse := httptest.NewRecorder()
	gateway.Engine().ServeHTTP(revokeResponse, revoke)
	if revokeResponse.Code != http.StatusOK || memories.principal != "U100" || memories.memoryID != "MEM-1" || memories.reason != "outdated" || memories.revokeCalls != 1 {
		t.Fatalf("revoke code=%d stub=%+v body=%s", revokeResponse.Code, memories, revokeResponse.Body.String())
	}
	forgedCorrection := httptest.NewRequest(http.MethodPost, "/api/v1/agent/memories/MEM-1/correct", strings.NewReader(`{"expectedVersion":1,"content":"Owner is Bob","compactContent":"Owner: Bob","reason":"fix owner","principalUserId":"U999"}`))
	forgedCorrection.Header.Set("Authorization", "Bearer "+token)
	forgedCorrection.Header.Set("Content-Type", "application/json")
	forgedCorrectionResponse := httptest.NewRecorder()
	gateway.Engine().ServeHTTP(forgedCorrectionResponse, forgedCorrection)
	if forgedCorrectionResponse.Code != http.StatusBadRequest || memories.correctCalls != 0 {
		t.Fatalf("forged correction code=%d calls=%d", forgedCorrectionResponse.Code, memories.correctCalls)
	}
	correction := httptest.NewRequest(http.MethodPost, "/api/v1/agent/memories/MEM-1/correct", strings.NewReader(`{"expectedVersion":1,"content":"Owner is Bob","compactContent":"Owner: Bob","reason":"fix owner"}`))
	correction.Header.Set("Authorization", "Bearer "+token)
	correction.Header.Set("Content-Type", "application/json")
	correctionResponse := httptest.NewRecorder()
	gateway.Engine().ServeHTTP(correctionResponse, correction)
	if correctionResponse.Code != http.StatusOK || memories.principal != "U100" || memories.expectedVersion != 1 || memories.content != "Owner is Bob" || memories.reason != "fix owner" || memories.correctCalls != 1 {
		t.Fatalf("correction code=%d stub=%+v body=%s", correctionResponse.Code, memories, correctionResponse.Body.String())
	}
}

func TestGatewayOwnsAuthenticatedAgentDefinitionCatalog(t *testing.T) {
	t.Chdir("../../../..")
	mr, _ := miniredis.Run()
	defer mr.Close()
	previousRedis := cache.RDB
	cache.RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = cache.RDB.Close(); cache.RDB = previousRedis })
	core := httptest.NewServer(http.NotFoundHandler())
	defer core.Close()
	catalog := &gatewayAgentDefinitionStub{}
	gateway, _ := newTestGatewayServer(core.URL, Dependencies{Messages: gatewayMessageStub{}, Core: gatewayCoreStub{}, AgentDefinitions: catalog, Limiter: gatewayLimiterStub{}})
	unauthorized := httptest.NewRecorder()
	gateway.Engine().ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/agent/definitions", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized code=%d", unauthorized.Code)
	}
	token, _ := coreauth.NewTokenService().Issue(&model.User{UUID: "U100"})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/agent/definitions?limit=20", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	gateway.Engine().ServeHTTP(response, request)
	if response.Code != http.StatusOK || catalog.principal != "U100" || catalog.limit != 20 || !strings.Contains(response.Body.String(), `"definitionId":"DEF-1"`) {
		t.Fatalf("catalog code=%d stub=%+v body=%s", response.Code, catalog, response.Body.String())
	}
}

func TestGatewayRejectsInvalidAgentSubscriptionControlInput(t *testing.T) {
	t.Chdir("../../../..")
	mr, _ := miniredis.Run()
	defer mr.Close()
	previousRedis := cache.RDB
	cache.RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = cache.RDB.Close()
		cache.RDB = previousRedis
	})
	core := httptest.NewServer(http.NotFoundHandler())
	defer core.Close()
	subscriptions := &gatewayAgentSubscriptionStub{}
	gateway, _ := newTestGatewayServer(core.URL, Dependencies{Messages: gatewayMessageStub{}, Core: gatewayCoreStub{}, AgentSubscriptions: subscriptions, Limiter: gatewayLimiterStub{}})
	token, _ := coreauth.NewTokenService().Issue(&model.User{UUID: "U100"})

	for _, target := range []string{
		"/api/v1/agent/subscriptions?limit=101",
		"/api/v1/agent/subscriptions?after=%20",
	} {
		request := httptest.NewRequest(http.MethodGet, target, nil)
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		gateway.Engine().ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("expected bad request for %s, got %d: %s", target, response.Code, response.Body.String())
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/agent/subscriptions/SUB-1/revoke", strings.NewReader(`{"reason":" "}`))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	gateway.Engine().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || subscriptions.listCalls != 0 || subscriptions.revokeCalls != 0 {
		t.Fatalf("invalid input reached application: code=%d stub=%+v", response.Code, subscriptions)
	}
	for _, invalid := range []struct {
		target string
		body   string
	}{
		{target: "/api/v1/agent/subscriptions/options?definitionId=DEF-1&definitionVersion=0"},
		{target: "/api/v1/agent/subscriptions", body: `{"definitionId":"DEF-1","definitionVersion":7,"conversationKey":"group:G123","filterKind":"all","filter":{},"principalUserId":"U999"}`},
	} {
		method := http.MethodGet
		var body io.Reader
		if invalid.body != "" {
			method, body = http.MethodPost, strings.NewReader(invalid.body)
		}
		request := httptest.NewRequest(method, invalid.target, body)
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		gateway.Engine().ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("expected bad request for %s, got %d: %s", invalid.target, response.Code, response.Body.String())
		}
	}
	if subscriptions.optionsCalls != 0 || subscriptions.createCalls != 0 {
		t.Fatalf("invalid create authority reached application: %+v", subscriptions)
	}
}

func TestGatewayOwnsAuthenticatedAgentTaskControlRoutes(t *testing.T) {
	t.Chdir("../../../..")
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer mr.Close()
	previousRedis := cache.RDB
	cache.RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = cache.RDB.Close(); cache.RDB = previousRedis })
	proxied := 0
	core := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { proxied++; writer.WriteHeader(http.StatusTeapot) }))
	defer core.Close()
	tasks := &gatewayAgentTaskStub{}
	gateway, err := newTestGatewayServer(core.URL, Dependencies{
		Messages: gatewayMessageStub{}, Core: gatewayCoreStub{}, AgentTasks: tasks, Limiter: gatewayLimiterStub{},
	})
	if err != nil {
		t.Fatalf("new gateway: %v", err)
	}

	unauthorized := httptest.NewRecorder()
	gateway.Engine().ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/agent/tasks/TASK-1", nil))
	if unauthorized.Code != http.StatusUnauthorized || proxied != 0 {
		t.Fatalf("unauthorized control: code=%d proxied=%d", unauthorized.Code, proxied)
	}
	token, err := coreauth.NewTokenService().Issue(&model.User{UUID: "U100"})
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/v1/agent/tasks/TASK-1/approvals/APR-1", strings.NewReader(`{"decision":"approved","principal_user_id":"U999"}`))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	gateway.Engine().ServeHTTP(response, request)
	if response.Code != http.StatusAccepted || proxied != 0 || tasks.principal != "U100" || tasks.taskID != "TASK-1" || tasks.approvalID != "APR-1" || tasks.decision != "approved" {
		t.Fatalf("Agent control: code=%d proxied=%d tasks=%+v body=%s", response.Code, proxied, tasks, response.Body.String())
	}
	inputRequest := httptest.NewRequest(http.MethodPost, "/api/v1/agent/tasks/TASK-1/inputs/INPUT-1", strings.NewReader(`{"value":{"scope":"today"},"principal_user_id":"U999"}`))
	inputRequest.Header.Set("Authorization", "Bearer "+token)
	inputRequest.Header.Set("Content-Type", "application/json")
	inputResponse := httptest.NewRecorder()
	gateway.Engine().ServeHTTP(inputResponse, inputRequest)
	if inputResponse.Code != http.StatusAccepted || tasks.principal != "U100" || tasks.requestID != "INPUT-1" {
		t.Fatalf("Agent input: code=%d tasks=%+v body=%s", inputResponse.Code, tasks, inputResponse.Body.String())
	}
}

func TestGatewayOwnsAuthenticatedAgentMCPRoute(t *testing.T) {
	t.Chdir("../../../..")
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer mr.Close()
	previousRedis := cache.RDB
	cache.RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = cache.RDB.Close(); cache.RDB = previousRedis })
	core := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusTeapot) }))
	defer core.Close()
	mcp := &gatewayAgentMCPStub{}
	gateway, err := newTestGatewayServer(core.URL, Dependencies{
		Messages: gatewayMessageStub{}, Core: gatewayCoreStub{}, AgentMCP: mcp, Limiter: gatewayLimiterStub{},
	})
	if err != nil {
		t.Fatalf("new gateway: %v", err)
	}

	path := "/api/v1/agent/tasks/TASK-1/runs/RUN-1/mcp"
	unauthorized := httptest.NewRecorder()
	gateway.Engine().ServeHTTP(unauthorized, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"jsonrpc":"2.0"}`)))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized MCP code=%d", unauthorized.Code)
	}
	sessionToken, err := coreauth.NewTokenService().Issue(&model.User{UUID: "U100"})
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	confusedRequest := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"jsonrpc":"2.0"}`))
	confusedRequest.Header.Set("Authorization", "Bearer "+sessionToken)
	confusedResponse := httptest.NewRecorder()
	gateway.Engine().ServeHTTP(confusedResponse, confusedRequest)
	if confusedResponse.Code != http.StatusUnauthorized || mcp.calls != 0 {
		t.Fatalf("ordinary session token reached MCP: code=%d calls=%d", confusedResponse.Code, mcp.calls)
	}
	token, err := coreauth.NewTokenService().IssueAgentMCPAccessToken("U100", coreauth.AgentMCPResource, []string{coreauth.AgentMCPReadScope}, true)
	if err != nil {
		t.Fatalf("issue Agent MCP token: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"jsonrpc":"2.0","principal_user_id":"U999"}`))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	gateway.Engine().ServeHTTP(response, request)
	if response.Code != http.StatusAccepted || response.Header().Get("Mcp-Session-Id") != "S1" ||
		mcp.principal != "U100" || mcp.taskID != "TASK-1" || mcp.runID != "RUN-1" {
		t.Fatalf("MCP route: code=%d headers=%v binding=%+v body=%s", response.Code, response.Header(), mcp, response.Body.String())
	}
}

func TestGatewayRateLimitsAgentMCPByAuthenticatedPrincipalAndAllowsDeleteCleanup(t *testing.T) {
	t.Chdir("../../../..")
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer mr.Close()
	previousRedis := cache.RDB
	cache.RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = cache.RDB.Close(); cache.RDB = previousRedis })
	core := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusTeapot) }))
	defer core.Close()
	mcp := &gatewayAgentMCPStub{}
	limiter := &gatewayAgentMCPLimiterStub{allowedCalls: 1, retryAfter: 1500 * time.Millisecond}
	gateway, err := newTestGatewayServer(core.URL, Dependencies{
		Messages: gatewayMessageStub{}, Core: gatewayCoreStub{}, AgentMCP: mcp,
		Limiter: gatewayLimiterStub{}, AgentMCPLimiter: limiter,
	})
	if err != nil {
		t.Fatalf("new gateway: %v", err)
	}
	token, err := coreauth.NewTokenService().IssueAgentMCPAccessToken("U100", coreauth.AgentMCPResource, []string{coreauth.AgentMCPReadScope}, true)
	if err != nil {
		t.Fatalf("issue Agent MCP token: %v", err)
	}
	firstPath := "/api/v1/agent/tasks/TASK-1/runs/RUN-1/mcp"
	firstPost := httptest.NewRequest(http.MethodPost, firstPath, strings.NewReader(`{"jsonrpc":"2.0"}`))
	firstPost.Header.Set("Authorization", "Bearer "+token)
	firstResponse := httptest.NewRecorder()
	gateway.Engine().ServeHTTP(firstResponse, firstPost)
	if firstResponse.Code != http.StatusAccepted || mcp.calls != 1 {
		t.Fatalf("first MCP request: code=%d calls=%d", firstResponse.Code, mcp.calls)
	}
	path := "/api/v1/agent/tasks/TASK-2/runs/RUN-2/mcp"
	post := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"jsonrpc":"2.0"}`))
	post.Header.Set("Authorization", "Bearer "+token)
	postResponse := httptest.NewRecorder()
	gateway.Engine().ServeHTTP(postResponse, post)
	if postResponse.Code != http.StatusTooManyRequests || postResponse.Header().Get("Retry-After") != "2" || mcp.calls != 1 || len(limiter.principals) != 2 || limiter.principals[0] != "U100" || limiter.principals[1] != "U100" {
		t.Fatalf("limited MCP: code=%d retry=%q calls=%d principals=%v body=%s", postResponse.Code, postResponse.Header().Get("Retry-After"), mcp.calls, limiter.principals, postResponse.Body.String())
	}
	deleteRequest := httptest.NewRequest(http.MethodDelete, path, nil)
	deleteRequest.Header.Set("Authorization", "Bearer "+token)
	deleteResponse := httptest.NewRecorder()
	gateway.Engine().ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusAccepted || mcp.calls != 2 || len(limiter.principals) != 2 {
		t.Fatalf("MCP cleanup: code=%d calls=%d principals=%v", deleteResponse.Code, mcp.calls, limiter.principals)
	}
}
