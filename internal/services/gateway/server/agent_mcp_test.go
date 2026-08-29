package gateway

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	coreauth "github.com/JekYUlll/Dipole/internal/services/core/domain/auth"
)

func TestAgentMCPProxyInjectsTrustedIdentityAndPreservesProtocolResponse(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/internal/v1/agent/tasks/TASK-1/runs/RUN-1/mcp" {
			t.Fatalf("unexpected upstream path %q", request.URL.Path)
		}
		if request.Header.Get("X-Dipole-Caller-Service") != "dipole-gateway" ||
			request.Header.Get("X-Dipole-Service-Token") != "mcp-secret" ||
			request.Header.Get("X-Dipole-Principal-User-ID") != "U100" ||
			request.Header.Get("X-Dipole-OAuth-Resource") != "https://dipole.local/api/v1/agent/mcp" ||
			request.Header.Get("X-Dipole-OAuth-Scope") != "dipole.agent.mcp.read" {
			t.Fatalf("untrusted upstream identity headers: %v", request.Header)
		}
		if request.Header.Get("Authorization") != "" || request.Header.Get("Cookie") != "" {
			t.Fatalf("public credentials reached upstream: %v", request.Header)
		}
		writer.Header().Set("Content-Type", "text/event-stream")
		writer.Header().Set("Mcp-Session-Id", "S1")
		writer.WriteHeader(http.StatusAccepted)
		_, _ = writer.Write([]byte("event: message\ndata: accepted\n\n"))
	}))
	defer upstream.Close()

	proxy, err := NewAgentMCPProxy(upstream.URL, "mcp-secret", coreauth.AgentMCPResource)
	if err != nil {
		t.Fatalf("new Agent MCP proxy: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/public", strings.NewReader(`{"jsonrpc":"2.0"}`))
	request.Header.Set("Authorization", "Bearer public-token")
	request.Header.Set("Cookie", "session=public")
	request.Header.Set("X-Dipole-Caller-Service", "attacker")
	request.Header.Set("X-Dipole-Service-Token", "attacker-secret")
	request.Header.Set("X-Dipole-Principal-User-ID", "U999")
	response := httptest.NewRecorder()
	proxy.ServeMCP(response, request, "U100", "TASK-1", "RUN-1")
	result := response.Result()
	defer result.Body.Close()
	body, _ := io.ReadAll(result.Body)
	if result.StatusCode != http.StatusAccepted || result.Header.Get("Mcp-Session-Id") != "S1" || !strings.Contains(string(body), "data: accepted") {
		t.Fatalf("unexpected proxy response: code=%d headers=%v body=%q", result.StatusCode, result.Header, string(body))
	}
}

func TestAgentMCPProxyRejectsInvalidConfigurationAndIdentifiers(t *testing.T) {
	if _, err := NewAgentMCPProxy("ftp://agent", "secret", coreauth.AgentMCPResource); err == nil {
		t.Fatal("expected invalid target rejection")
	}
	if _, err := NewAgentMCPProxy("http://agent:8091", "", coreauth.AgentMCPResource); err == nil {
		t.Fatal("expected missing secret rejection")
	}
	if _, err := NewAgentMCPProxy("http://agent:8091", "secret", "mcp.example.com"); err == nil {
		t.Fatal("expected invalid resource rejection")
	}
	proxy, _ := NewAgentMCPProxy("http://agent:8091", "secret", coreauth.AgentMCPResource)
	response := httptest.NewRecorder()
	proxy.ServeMCP(response, httptest.NewRequest(http.MethodPost, "/public", nil), "U100", "bad/task", "RUN-1")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid identifier code=%d", response.Code)
	}
}
