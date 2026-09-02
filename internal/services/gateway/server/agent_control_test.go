package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/platform/correlation"
)

func TestAgentTaskControlClientUsesTrustedServiceHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Dipole-Caller-Service") != "dipole-gateway" || request.Header.Get("X-Dipole-Service-Token") != "secret" ||
			request.Header.Get("X-Dipole-Principal-User-ID") != "U100" || request.Header.Get(correlation.RequestHeader) != "R1" || request.Header.Get(correlation.TraceHeader) != "T1" {
			t.Fatalf("unexpected trusted headers: %v", request.Header)
		}
		if request.URL.Path != "/internal/v1/agent/tasks/TASK-1/approvals/APR-1" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		var body map[string]string
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil || body["decision"] != "approved" {
			t.Fatalf("unexpected body: body=%v err=%v", body, err)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusAccepted)
		_, _ = writer.Write([]byte(`{"status":"resolution_requested"}`))
	}))
	defer server.Close()
	client, err := NewAgentTaskControlClient(server.URL, "secret", time.Second)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	ctx := correlation.WithContext(context.Background(), correlation.IDs{RequestID: "R1", TraceID: "T1"})
	result, err := client.ResolveApproval(ctx, "U100", "TASK-1", "APR-1", "approved")
	if err != nil || result.StatusCode != http.StatusAccepted {
		t.Fatalf("resolve approval: result=%+v err=%v", result, err)
	}
}

func TestAgentTaskControlClientStartsInteractiveTaskWithoutPrincipalField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/internal/v1/agent/tasks" || request.Header.Get("X-Dipole-Principal-User-ID") != "U100" {
			t.Fatalf("unexpected start request: path=%s headers=%v", request.URL.Path, request.Header)
		}
		var body map[string]string
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil || body["clientRequestId"] != "client-1" || body["goal"] != "Summarize unread work" || body["principalUserId"] != "" {
			t.Fatalf("unexpected start body: body=%v err=%v", body, err)
		}
		writer.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	client, _ := NewAgentTaskControlClient(server.URL, "secret", time.Second)
	result, err := client.StartTask(context.Background(), "U100", "client-1", "Summarize unread work")
	if err != nil || result.StatusCode != http.StatusAccepted {
		t.Fatalf("start task: result=%+v err=%v", result, err)
	}
}

func TestAgentTaskControlClientGetsRuntimeStatusWithTrustedPrincipal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/internal/v1/agent/status" || request.Header.Get("X-Dipole-Principal-User-ID") != "U100" {
			t.Fatalf("unexpected runtime status request: method=%s path=%s headers=%v", request.Method, request.URL.Path, request.Header)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"schemaVersion":"dipole.agent.runtime_status.v1","taskControlEnabled":true}`))
	}))
	defer server.Close()
	client, err := NewAgentTaskControlClient(server.URL, "secret", time.Second)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	result, err := client.GetRuntimeStatus(context.Background(), "U100")
	if err != nil || result.StatusCode != http.StatusOK || !bytes.Contains(result.Body, []byte(`"taskControlEnabled":true`)) {
		t.Fatalf("runtime status: result=%+v err=%v", result, err)
	}
}

func TestAgentTaskControlClientSeparatesTimelineQueryFromPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/internal/v1/agent/tasks/TASK-1/timeline" || request.URL.Query().Get("limit") != "50" || request.URL.Query().Get("after") != "42" {
			t.Fatalf("unexpected timeline request: path=%s query=%s", request.URL.Path, request.URL.RawQuery)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"schemaVersion":"dipole.agent.task_timeline.v1","taskId":"TASK-1","revision":0,"events":[],"nextCursor":""}`))
	}))
	defer server.Close()

	client, err := NewAgentTaskControlClient(server.URL, "secret", time.Second)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	result, err := client.GetTimeline(context.Background(), "U100", "TASK-1", "42", 50)
	if err != nil || result.StatusCode != http.StatusOK {
		t.Fatalf("get timeline: result=%+v err=%v", result, err)
	}
}

func TestAgentTaskControlClientForwardsStructuredInputWithoutPrincipalField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/internal/v1/agent/tasks/TASK-1/inputs/INPUT-1" || request.Header.Get("X-Dipole-Principal-User-ID") != "U100" {
			t.Fatalf("unexpected input request: path=%s headers=%v", request.URL.Path, request.Header)
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode input body: %v", err)
		}
		value, ok := body["value"].(map[string]any)
		if !ok || value["scope"] != "today" || body["principalUserId"] != nil {
			t.Fatalf("unexpected input body: %v", body)
		}
		writer.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	client, _ := NewAgentTaskControlClient(server.URL, "secret", time.Second)
	result, err := client.ProvideInput(context.Background(), "U100", "TASK-1", "INPUT-1", map[string]any{"scope": "today"})
	if err != nil || result.StatusCode != http.StatusAccepted {
		t.Fatalf("provide input: result=%+v err=%v", result, err)
	}
}
