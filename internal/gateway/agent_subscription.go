package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	grpccommon "github.com/JekYUlll/Dipole/internal/transport/grpc/common"
	agentv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/agent/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrAgentSubscriptionInvalid     = errors.New("Agent Subscription request is invalid")
	ErrAgentSubscriptionDenied      = errors.New("Agent Subscription access denied")
	ErrAgentSubscriptionConflict    = errors.New("Agent Subscription changed concurrently")
	ErrAgentSubscriptionUnavailable = errors.New("Agent Subscription control is unavailable")
)

type AgentSubscriptionFilter struct {
	Terms []string `json:"terms,omitempty"`
}

type AgentSubscription struct {
	SubscriptionID    string                  `json:"subscriptionId"`
	DefinitionID      string                  `json:"definitionId"`
	DefinitionVersion uint64                  `json:"definitionVersion"`
	AgentID           string                  `json:"agentId"`
	EventType         string                  `json:"eventType"`
	ResourceType      string                  `json:"resourceType"`
	ResourceID        string                  `json:"resourceId"`
	FilterKind        string                  `json:"filterKind"`
	Filter            AgentSubscriptionFilter `json:"filter"`
	Status            string                  `json:"status"`
	CreatedByID       string                  `json:"createdById"`
	RevokedByID       string                  `json:"revokedById,omitempty"`
	RevokeReason      string                  `json:"revokeReason,omitempty"`
	CreatedAtUnixMS   int64                   `json:"createdAtUnixMs"`
	UpdatedAtUnixMS   int64                   `json:"updatedAtUnixMs"`
	RevokedAtUnixMS   int64                   `json:"revokedAtUnixMs,omitempty"`
}

type AgentSubscriptionPage struct {
	Subscriptions []AgentSubscription `json:"subscriptions"`
	NextCursor    string              `json:"nextCursor,omitempty"`
}

type AgentSubscriptionControlApplication interface {
	List(ctx context.Context, principalUUID, after string, limit int) (*AgentSubscriptionPage, error)
	Revoke(ctx context.Context, principalUUID, subscriptionID, reason string) (*AgentSubscription, error)
}

type agentSubscriptionRPC interface {
	ListEventSubscriptions(context.Context, *agentv1.ListEventSubscriptionsRequest, ...grpc.CallOption) (*agentv1.ListEventSubscriptionsResponse, error)
	RevokeEventSubscription(context.Context, *agentv1.RevokeEventSubscriptionRequest, ...grpc.CallOption) (*agentv1.AgentEventSubscription, error)
}

type AgentSubscriptionControlClient struct {
	rpc      agentSubscriptionRPC
	tenantID string
	timeout  time.Duration
}

func NewAgentSubscriptionControlClient(rpc agentSubscriptionRPC, tenantID string, timeout time.Duration) (*AgentSubscriptionControlClient, error) {
	tenantID = strings.TrimSpace(tenantID)
	if rpc == nil || tenantID == "" || len(tenantID) > 64 {
		return nil, ErrAgentSubscriptionInvalid
	}
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &AgentSubscriptionControlClient{rpc: rpc, tenantID: tenantID, timeout: timeout}, nil
}

func (c *AgentSubscriptionControlClient) List(ctx context.Context, principalUUID, after string, limit int) (*AgentSubscriptionPage, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := c.rpc.ListEventSubscriptions(callCtx, &agentv1.ListEventSubscriptionsRequest{
		Context: grpccommon.RequestContextFrom(ctx, principalUUID, "dipole-gateway"), TenantId: c.tenantID,
		AfterSubscriptionId: after, Limit: uint32(limit),
	})
	if err != nil {
		return nil, mapAgentSubscriptionRPCError(err)
	}
	if response == nil {
		return nil, ErrAgentSubscriptionUnavailable
	}
	page := &AgentSubscriptionPage{Subscriptions: make([]AgentSubscription, 0, len(response.GetSubscriptions())), NextCursor: response.GetNextCursor()}
	for _, raw := range response.GetSubscriptions() {
		item, parseErr := agentSubscriptionFromProto(raw, c.tenantID)
		if parseErr != nil {
			return nil, ErrAgentSubscriptionUnavailable
		}
		page.Subscriptions = append(page.Subscriptions, *item)
	}
	return page, nil
}

func (c *AgentSubscriptionControlClient) Revoke(ctx context.Context, principalUUID, subscriptionID, reason string) (*AgentSubscription, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := c.rpc.RevokeEventSubscription(callCtx, &agentv1.RevokeEventSubscriptionRequest{
		Context: grpccommon.RequestContextFrom(ctx, principalUUID, "dipole-gateway"), TenantId: c.tenantID,
		SubscriptionId: subscriptionID, Reason: reason,
	})
	if err != nil {
		return nil, mapAgentSubscriptionRPCError(err)
	}
	if response == nil {
		return nil, ErrAgentSubscriptionUnavailable
	}
	item, err := agentSubscriptionFromProto(response, c.tenantID)
	if err != nil {
		return nil, ErrAgentSubscriptionUnavailable
	}
	return item, nil
}

func agentSubscriptionFromProto(raw *agentv1.AgentEventSubscription, tenantID string) (*AgentSubscription, error) {
	if raw == nil || raw.GetTenantId() != tenantID || strings.TrimSpace(raw.GetSubscriptionId()) == "" ||
		strings.TrimSpace(raw.GetDefinitionId()) == "" || raw.GetDefinitionVersion() == 0 ||
		strings.TrimSpace(raw.GetAgentId()) == "" || strings.TrimSpace(raw.GetEventType()) == "" ||
		raw.GetResourceType() != "conversation" || strings.TrimSpace(raw.GetResourceId()) == "" ||
		strings.TrimSpace(raw.GetCreatedById()) == "" || (raw.GetStatus() != "active" && raw.GetStatus() != "revoked") {
		return nil, ErrAgentSubscriptionUnavailable
	}
	filter, err := parseAgentSubscriptionFilter(raw.GetFilterKind(), raw.GetFilterJson())
	if err != nil {
		return nil, err
	}
	if raw.GetStatus() == "active" && (raw.GetRevokedById() != "" || raw.GetRevokeReason() != "" || raw.GetRevokedAtUnixMs() != 0) {
		return nil, ErrAgentSubscriptionUnavailable
	}
	if raw.GetStatus() == "revoked" && (strings.TrimSpace(raw.GetRevokedById()) == "" || strings.TrimSpace(raw.GetRevokeReason()) == "" || raw.GetRevokedAtUnixMs() <= 0) {
		return nil, ErrAgentSubscriptionUnavailable
	}
	return &AgentSubscription{
		SubscriptionID: raw.GetSubscriptionId(), DefinitionID: raw.GetDefinitionId(), DefinitionVersion: raw.GetDefinitionVersion(),
		AgentID: raw.GetAgentId(), EventType: raw.GetEventType(), ResourceType: raw.GetResourceType(), ResourceID: raw.GetResourceId(),
		FilterKind: raw.GetFilterKind(), Filter: filter, Status: raw.GetStatus(), CreatedByID: raw.GetCreatedById(),
		RevokedByID: raw.GetRevokedById(), RevokeReason: raw.GetRevokeReason(), CreatedAtUnixMS: raw.GetCreatedAtUnixMs(),
		UpdatedAtUnixMS: raw.GetUpdatedAtUnixMs(), RevokedAtUnixMS: raw.GetRevokedAtUnixMs(),
	}, nil
}

func parseAgentSubscriptionFilter(kind string, raw []byte) (AgentSubscriptionFilter, error) {
	switch kind {
	case "all":
		var value struct{}
		if err := decodeStrictAgentSubscriptionJSON(raw, &value); err != nil {
			return AgentSubscriptionFilter{}, ErrAgentSubscriptionUnavailable
		}
		return AgentSubscriptionFilter{}, nil
	case "message_contains_any":
		var value AgentSubscriptionFilter
		if err := decodeStrictAgentSubscriptionJSON(raw, &value); err != nil || len(value.Terms) == 0 || len(value.Terms) > 32 {
			return AgentSubscriptionFilter{}, ErrAgentSubscriptionUnavailable
		}
		for _, term := range value.Terms {
			if strings.TrimSpace(term) == "" {
				return AgentSubscriptionFilter{}, ErrAgentSubscriptionUnavailable
			}
		}
		return value, nil
	default:
		return AgentSubscriptionFilter{}, ErrAgentSubscriptionUnavailable
	}
}

func decodeStrictAgentSubscriptionJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("Agent Subscription filter has trailing data")
	}
	return nil
}

func mapAgentSubscriptionRPCError(err error) error {
	switch status.Code(err) {
	case codes.InvalidArgument, codes.FailedPrecondition:
		return ErrAgentSubscriptionInvalid
	case codes.Unauthenticated, codes.PermissionDenied, codes.NotFound:
		return ErrAgentSubscriptionDenied
	case codes.Aborted, codes.AlreadyExists:
		return ErrAgentSubscriptionConflict
	case codes.Unavailable, codes.DeadlineExceeded, codes.Canceled:
		return ErrAgentSubscriptionUnavailable
	default:
		return ErrAgentSubscriptionUnavailable
	}
}

func AgentSubscriptionHTTPStatus(err error) int {
	switch {
	case errors.Is(err, ErrAgentSubscriptionInvalid):
		return 400
	case errors.Is(err, ErrAgentSubscriptionDenied):
		return 403
	case errors.Is(err, ErrAgentSubscriptionConflict):
		return 409
	default:
		return 503
	}
}
