package gateway

import (
	"context"
	"errors"
	"testing"
	"time"

	agentv1 "github.com/JekYUlll/Dipole/api/gen/go/agent/v1"
	"google.golang.org/grpc"
)

type gatewayAgentTaskInboxRPCStub struct {
	listRequest  *agentv1.ListOwnedAgentTasksRequest
	listResponse *agentv1.ListOwnedAgentTasksResponse
}

func (s *gatewayAgentTaskInboxRPCStub) ListOwnedAgentTasks(_ context.Context, request *agentv1.ListOwnedAgentTasksRequest, _ ...grpc.CallOption) (*agentv1.ListOwnedAgentTasksResponse, error) {
	s.listRequest = request
	return s.listResponse, nil
}

func TestAgentTaskInboxClientBindsPrincipalAndCanonicalCursor(t *testing.T) {
	now := time.UnixMilli(1_700_000_000_000).UTC()
	rpc := &gatewayAgentTaskInboxRPCStub{listResponse: &agentv1.ListOwnedAgentTasksResponse{
		Tasks: []*agentv1.AgentOwnedTask{{
			TaskId: "TASK-1", Status: "waiting_approval", Revision: 2, PendingKind: "approval",
			Goal: "Summarize unread work", UpdatedAtUnixMs: now.UnixMilli(),
		}},
		NextUpdatedAtUnixMs: now.UnixMilli(), NextTaskId: "TASK-1",
	}}
	client, err := NewAgentTaskInboxClient(rpc, "dipole", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	page, err := client.List(context.Background(), "U100", "", 20)
	if err != nil || len(page.Tasks) != 1 || page.NextCursor == "" || rpc.listRequest.GetContext().GetPrincipalUserId() != "U100" || rpc.listRequest.GetTenantId() != "dipole" {
		t.Fatalf("list page=%+v request=%+v err=%v", page, rpc.listRequest, err)
	}
	updatedAt, taskID, err := decodeAgentTaskInboxCursor(page.NextCursor)
	if err != nil || !updatedAt.Equal(now) || taskID != "TASK-1" {
		t.Fatalf("cursor updated=%s task=%s err=%v", updatedAt, taskID, err)
	}
	if _, err = client.List(context.Background(), "U100", page.NextCursor+"=", 20); !errors.Is(err, ErrAgentTaskInboxInvalid) {
		t.Fatalf("non-canonical cursor error = %v", err)
	}
}

func TestAgentTaskInboxClientRejectsPendingKindDrift(t *testing.T) {
	now := time.UnixMilli(1_700_000_000_000).UTC()
	rpc := &gatewayAgentTaskInboxRPCStub{listResponse: &agentv1.ListOwnedAgentTasksResponse{
		Tasks: []*agentv1.AgentOwnedTask{{
			TaskId: "TASK-1", Status: "waiting_input", Revision: 1, PendingKind: "approval",
			Goal: "Need input", UpdatedAtUnixMs: now.UnixMilli(),
		}},
	}}
	client, _ := NewAgentTaskInboxClient(rpc, "dipole", time.Second)
	if _, err := client.List(context.Background(), "U100", "", 20); !errors.Is(err, ErrAgentTaskInboxUnavailable) {
		t.Fatalf("pending kind drift error = %v", err)
	}
}
