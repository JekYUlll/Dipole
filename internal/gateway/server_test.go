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
	"github.com/JekYUlll/Dipole/internal/service"
	"github.com/JekYUlll/Dipole/internal/store"
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

type gatewayCoreStub struct{}

type gatewayLimiterStub struct{}

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
}

func (s *gatewayAgentTaskStub) GetTask(_ context.Context, principalUUID, taskUUID string) (*AgentTaskControlResult, error) {
	s.principal, s.taskID = principalUUID, taskUUID
	return agentControlJSON(http.StatusOK, map[string]any{"taskId": taskUUID, "status": "running"}), nil
}

func (s *gatewayAgentTaskStub) CancelTask(_ context.Context, principalUUID, taskUUID, reason string) (*AgentTaskControlResult, error) {
	s.principal, s.taskID, s.reason = principalUUID, taskUUID, reason
	return agentControlJSON(http.StatusAccepted, map[string]any{"status": "cancellation_requested"}), nil
}

func (s *gatewayAgentTaskStub) ResolveApproval(_ context.Context, principalUUID, taskUUID, approvalUUID, decision string) (*AgentTaskControlResult, error) {
	s.principal, s.taskID, s.approvalID, s.decision = principalUUID, taskUUID, approvalUUID, decision
	return agentControlJSON(http.StatusAccepted, map[string]any{"status": "resolution_requested"}), nil
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
	gateway, err := NewServer(core.URL, Dependencies{
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

func TestGatewayRequiresRemoteDependencies(t *testing.T) {
	if _, err := NewServer("http://127.0.0.1:8081", Dependencies{Core: gatewayCoreStub{}, Limiter: gatewayLimiterStub{}}); err == nil {
		t.Fatal("expected missing message application to fail")
	}
	if _, err := NewServer("http://127.0.0.1:8081", Dependencies{Messages: gatewayMessageStub{}, Limiter: gatewayLimiterStub{}}); err == nil {
		t.Fatal("expected missing core capability to fail")
	}
	if _, err := NewServer("not-a-url", Dependencies{Messages: gatewayMessageStub{}, Core: gatewayCoreStub{}, Limiter: gatewayLimiterStub{}}); err == nil {
		t.Fatal("expected invalid core target to fail")
	}
	if _, err := NewServer("ftp://127.0.0.1", Dependencies{Messages: gatewayMessageStub{}, Core: gatewayCoreStub{}, Limiter: gatewayLimiterStub{}}); err == nil {
		t.Fatal("expected unsupported core target scheme to fail")
	}
}

func TestGatewayOwnsAuthenticatedSearchRoute(t *testing.T) {
	t.Chdir("../..")
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer mr.Close()
	previousRedis := store.RDB
	store.RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = store.RDB.Close()
		store.RDB = previousRedis
	})
	proxied := 0
	core := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		proxied++
		writer.WriteHeader(http.StatusTeapot)
	}))
	defer core.Close()
	search := &gatewaySearchStub{}
	gateway, err := NewServer(core.URL, Dependencies{
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
	token, err := service.NewTokenService().Issue(&model.User{UUID: "U1"})
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

func TestGatewayOwnsAuthenticatedAgentTaskControlRoutes(t *testing.T) {
	t.Chdir("../..")
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer mr.Close()
	previousRedis := store.RDB
	store.RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = store.RDB.Close(); store.RDB = previousRedis })
	proxied := 0
	core := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { proxied++; writer.WriteHeader(http.StatusTeapot) }))
	defer core.Close()
	tasks := &gatewayAgentTaskStub{}
	gateway, err := NewServer(core.URL, Dependencies{
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
	token, err := service.NewTokenService().Issue(&model.User{UUID: "U100"})
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
}
