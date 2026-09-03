package gateway

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	agentv1 "github.com/JekYUlll/Dipole/api/gen/go/agent/v1"
	grpccommon "github.com/JekYUlll/Dipole/internal/transport/grpc/common"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrAgentTaskInboxInvalid     = errors.New("Agent Task inbox request is invalid")
	ErrAgentTaskInboxDenied      = errors.New("Agent Task inbox access denied")
	ErrAgentTaskInboxUnavailable = errors.New("Agent Task inbox is unavailable")
)

type AgentOwnedTask struct {
	TaskID          string `json:"taskId"`
	Status          string `json:"status"`
	Revision        uint64 `json:"revision"`
	PendingKind     string `json:"pendingKind,omitempty"`
	Goal            string `json:"goal"`
	UpdatedAtUnixMS int64  `json:"updatedAtUnixMs"`
}

type AgentTaskInboxPage struct {
	Tasks      []AgentOwnedTask `json:"tasks"`
	NextCursor string           `json:"nextCursor,omitempty"`
}

type AgentTaskInboxApplication interface {
	List(ctx context.Context, principalUUID, after string, limit int) (*AgentTaskInboxPage, error)
}

type agentTaskInboxRPC interface {
	ListOwnedAgentTasks(context.Context, *agentv1.ListOwnedAgentTasksRequest, ...grpc.CallOption) (*agentv1.ListOwnedAgentTasksResponse, error)
}

type AgentTaskInboxClient struct {
	rpc      agentTaskInboxRPC
	tenantID string
	timeout  time.Duration
}

func NewAgentTaskInboxClient(rpc agentTaskInboxRPC, tenantID string, timeout time.Duration) (*AgentTaskInboxClient, error) {
	tenantID = strings.TrimSpace(tenantID)
	if rpc == nil || tenantID == "" || len([]rune(tenantID)) > 64 {
		return nil, ErrAgentTaskInboxInvalid
	}
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &AgentTaskInboxClient{rpc: rpc, tenantID: tenantID, timeout: timeout}, nil
}

func (c *AgentTaskInboxClient) List(ctx context.Context, principalUUID, after string, limit int) (*AgentTaskInboxPage, error) {
	afterUpdatedAt, afterTaskID, err := decodeAgentTaskInboxCursor(after)
	if err != nil || strings.TrimSpace(principalUUID) == "" || limit < 1 || limit > 100 {
		return nil, ErrAgentTaskInboxInvalid
	}
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	request := &agentv1.ListOwnedAgentTasksRequest{
		Context: grpccommon.RequestContextFrom(ctx, principalUUID, "dipole-gateway"), TenantId: c.tenantID,
		AfterTaskId: afterTaskID, Limit: uint32(limit),
	}
	if !afterUpdatedAt.IsZero() {
		request.AfterUpdatedAtUnixMs = afterUpdatedAt.UnixMilli()
	}
	response, err := c.rpc.ListOwnedAgentTasks(callCtx, request)
	if err != nil {
		return nil, mapAgentTaskInboxRPCError(err)
	}
	if response == nil || (response.GetNextUpdatedAtUnixMs() == 0) != (response.GetNextTaskId() == "") {
		return nil, ErrAgentTaskInboxUnavailable
	}
	page := &AgentTaskInboxPage{Tasks: make([]AgentOwnedTask, 0, len(response.GetTasks()))}
	for _, raw := range response.GetTasks() {
		item, mapErr := agentOwnedTaskFromProto(raw)
		if mapErr != nil {
			return nil, ErrAgentTaskInboxUnavailable
		}
		page.Tasks = append(page.Tasks, item)
	}
	if response.GetNextTaskId() != "" {
		page.NextCursor, err = encodeAgentTaskInboxCursor(time.UnixMilli(response.GetNextUpdatedAtUnixMs()).UTC(), response.GetNextTaskId())
		if err != nil {
			return nil, ErrAgentTaskInboxUnavailable
		}
	}
	return page, nil
}

func agentOwnedTaskFromProto(raw *agentv1.AgentOwnedTask) (AgentOwnedTask, error) {
	if raw == nil || !validAgentSubscriptionPublicID(raw.GetTaskId(), 64) || !validAgentTaskInboxStatus(raw.GetStatus()) ||
		raw.GetUpdatedAtUnixMs() <= 0 || len([]rune(raw.GetGoal())) > 200 {
		return AgentOwnedTask{}, ErrAgentTaskInboxUnavailable
	}
	pendingKind := strings.TrimSpace(raw.GetPendingKind())
	if pendingKind != "" && pendingKind != "input" && pendingKind != "approval" {
		return AgentOwnedTask{}, ErrAgentTaskInboxUnavailable
	}
	if (raw.GetStatus() == "waiting_input" && pendingKind != "input") || (raw.GetStatus() == "waiting_approval" && pendingKind != "approval") {
		return AgentOwnedTask{}, ErrAgentTaskInboxUnavailable
	}
	return AgentOwnedTask{
		TaskID: raw.GetTaskId(), Status: raw.GetStatus(), Revision: raw.GetRevision(),
		PendingKind: pendingKind, Goal: raw.GetGoal(), UpdatedAtUnixMS: raw.GetUpdatedAtUnixMs(),
	}, nil
}

func validAgentTaskInboxStatus(value string) bool {
	switch value {
	case "created", "running", "waiting_input", "waiting_approval", "completed", "failed", "cancelled":
		return true
	default:
		return false
	}
}

func encodeAgentTaskInboxCursor(updatedAt time.Time, taskID string) (string, error) {
	if updatedAt.IsZero() || !validAgentSubscriptionPublicID(taskID, 64) {
		return "", ErrAgentTaskInboxInvalid
	}
	encoded, err := json.Marshal(struct {
		UpdatedAtUnixMS int64  `json:"updatedAtUnixMs"`
		TaskID          string `json:"taskId"`
	}{UpdatedAtUnixMS: updatedAt.UTC().UnixMilli(), TaskID: taskID})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

func decodeAgentTaskInboxCursor(cursor string) (time.Time, string, error) {
	if cursor == "" {
		return time.Time{}, "", nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil || len(decoded) > 256 {
		return time.Time{}, "", ErrAgentTaskInboxInvalid
	}
	var value struct {
		UpdatedAtUnixMS int64  `json:"updatedAtUnixMs"`
		TaskID          string `json:"taskId"`
	}
	if decodeStrictAgentSubscriptionJSON(decoded, &value) != nil || value.UpdatedAtUnixMS <= 0 || !validAgentSubscriptionPublicID(value.TaskID, 64) {
		return time.Time{}, "", ErrAgentTaskInboxInvalid
	}
	updatedAt := time.UnixMilli(value.UpdatedAtUnixMS).UTC()
	canonical, err := encodeAgentTaskInboxCursor(updatedAt, value.TaskID)
	if err != nil || canonical != cursor {
		return time.Time{}, "", ErrAgentTaskInboxInvalid
	}
	return updatedAt, value.TaskID, nil
}

func mapAgentTaskInboxRPCError(err error) error {
	switch status.Code(err) {
	case codes.InvalidArgument, codes.FailedPrecondition:
		return ErrAgentTaskInboxInvalid
	case codes.PermissionDenied, codes.Unauthenticated:
		return ErrAgentTaskInboxDenied
	default:
		return ErrAgentTaskInboxUnavailable
	}
}
