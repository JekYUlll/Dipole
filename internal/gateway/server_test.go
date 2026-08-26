package gateway

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
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
func (gatewayMessageStub) ListGroupMessages(string, string, uint, int) ([]*model.Message, error) {
	return nil, nil
}
func (gatewayMessageStub) ListGroupMessagesAfter(string, string, uint, int) ([]*model.Message, error) {
	return nil, nil
}
func (gatewayMessageStub) ListOfflineMessages(string, uint, int) ([]*model.Message, error) {
	return nil, nil
}

type gatewayCoreStub struct{}

type gatewayLimiterStub struct{}

func (gatewayLimiterStub) AllowMessageSend(string) (bool, time.Duration) { return true, 0 }

func (gatewayCoreStub) GetUserByUUID(userUUID string) (*model.User, error) {
	return &model.User{UUID: userUUID}, nil
}
func (gatewayCoreStub) CanSendDirectMessage(string, string) (bool, error)         { return true, nil }
func (gatewayCoreStub) GetGroupByUUID(string) (*model.Group, error)               { return nil, nil }
func (gatewayCoreStub) GetGroupMember(string, string) (*model.GroupMember, error) { return nil, nil }
func (gatewayCoreStub) ListGroupMembers(string) ([]*model.GroupMember, error)     { return nil, nil }
func (gatewayCoreStub) GetOwnedFile(string, string) (*model.UploadedFile, error)  { return nil, nil }

var _ application.MessageApplication = gatewayMessageStub{}
var _ application.CoreCapability = gatewayCoreStub{}

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
