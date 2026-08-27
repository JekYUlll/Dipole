package gateway

import (
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
