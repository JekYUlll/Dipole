package gateway

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	agentv1 "github.com/JekYUlll/Dipole/api/gen/go/agent/v1"
	grpccommon "github.com/JekYUlll/Dipole/internal/transport/grpc/common"
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

type AgentSubscriptionConversationOption struct {
	ConversationKey string `json:"conversationKey"`
	EventType       string `json:"eventType"`
}

type AgentSubscriptionConversationOptions struct {
	Conversations []AgentSubscriptionConversationOption `json:"conversations"`
}

type AgentSubscriptionCreateInput struct {
	DefinitionID      string                  `json:"definitionId"`
	DefinitionVersion uint64                  `json:"definitionVersion"`
	ConversationKey   string                  `json:"conversationKey"`
	FilterKind        string                  `json:"filterKind"`
	Filter            AgentSubscriptionFilter `json:"filter"`
}

type AgentDefinitionCatalogItem struct {
	DefinitionID       string   `json:"definitionId"`
	Version            uint64   `json:"version"`
	AgentID            string   `json:"agentId"`
	ConversationScopes []string `json:"conversationScopes"`
	ValidFromUnixMS    int64    `json:"validFromUnixMs"`
	ExpiresAtUnixMS    int64    `json:"expiresAtUnixMs,omitempty"`
	CreatedAtUnixMS    int64    `json:"createdAtUnixMs"`
	UpdatedAtUnixMS    int64    `json:"updatedAtUnixMs"`
}

type AgentDefinitionCatalogPage struct {
	Definitions []AgentDefinitionCatalogItem `json:"definitions"`
	NextCursor  string                       `json:"nextCursor,omitempty"`
}

type AgentSubscriptionControlApplication interface {
	Create(ctx context.Context, principalUUID string, input AgentSubscriptionCreateInput) (*AgentSubscription, error)
	ListEligibleConversations(ctx context.Context, principalUUID, definitionID string, definitionVersion uint64) (*AgentSubscriptionConversationOptions, error)
	List(ctx context.Context, principalUUID, after string, limit int) (*AgentSubscriptionPage, error)
	Revoke(ctx context.Context, principalUUID, subscriptionID, reason string) (*AgentSubscription, error)
}

type AgentDefinitionCatalogApplication interface {
	CreateDefinition(ctx context.Context, principalUUID, profile string) (*AgentDefinitionCatalogItem, error)
	ListDefinitions(ctx context.Context, principalUUID, after string, limit int) (*AgentDefinitionCatalogPage, error)
}

type agentSubscriptionRPC interface {
	CreateEventSubscription(context.Context, *agentv1.CreateEventSubscriptionRequest, ...grpc.CallOption) (*agentv1.AgentEventSubscription, error)
	ListEligibleSubscriptionConversations(context.Context, *agentv1.ListEligibleSubscriptionConversationsRequest, ...grpc.CallOption) (*agentv1.ListEligibleSubscriptionConversationsResponse, error)
	ListEventSubscriptions(context.Context, *agentv1.ListEventSubscriptionsRequest, ...grpc.CallOption) (*agentv1.ListEventSubscriptionsResponse, error)
	RevokeEventSubscription(context.Context, *agentv1.RevokeEventSubscriptionRequest, ...grpc.CallOption) (*agentv1.AgentEventSubscription, error)
	CreateAgentDefinition(context.Context, *agentv1.CreateAgentDefinitionRequest, ...grpc.CallOption) (*agentv1.AgentDefinitionCatalogItem, error)
	ListAgentDefinitions(context.Context, *agentv1.ListAgentDefinitionsRequest, ...grpc.CallOption) (*agentv1.ListAgentDefinitionsResponse, error)
}

func (c *AgentSubscriptionControlClient) CreateDefinition(ctx context.Context, principalUUID, profile string) (*AgentDefinitionCatalogItem, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := c.rpc.CreateAgentDefinition(callCtx, &agentv1.CreateAgentDefinitionRequest{Context: grpccommon.RequestContextFrom(ctx, principalUUID, "dipole-gateway"), TenantId: c.tenantID, Profile: profile})
	if err != nil {
		return nil, mapAgentSubscriptionRPCError(err)
	}
	if response == nil || !validAgentSubscriptionPublicID(response.GetDefinitionId(), 64) || response.GetVersion() == 0 || !validAgentSubscriptionPublicID(response.GetAgentId(), 24) || len(response.GetConversationScopes()) != 1 || response.GetConversationScopes()[0] != "*" {
		return nil, ErrAgentSubscriptionUnavailable
	}
	return &AgentDefinitionCatalogItem{DefinitionID: response.GetDefinitionId(), Version: response.GetVersion(), AgentID: response.GetAgentId(), ConversationScopes: append([]string(nil), response.GetConversationScopes()...), ValidFromUnixMS: response.GetValidFromUnixMs(), ExpiresAtUnixMS: response.GetExpiresAtUnixMs(), CreatedAtUnixMS: response.GetCreatedAtUnixMs(), UpdatedAtUnixMS: response.GetUpdatedAtUnixMs()}, nil
}

func (c *AgentSubscriptionControlClient) ListEligibleConversations(ctx context.Context, principalUUID, definitionID string, definitionVersion uint64) (*AgentSubscriptionConversationOptions, error) {
	if !validAgentSubscriptionPublicID(definitionID, 64) || definitionVersion == 0 {
		return nil, ErrAgentSubscriptionInvalid
	}
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := c.rpc.ListEligibleSubscriptionConversations(callCtx, &agentv1.ListEligibleSubscriptionConversationsRequest{
		Context: grpccommon.RequestContextFrom(ctx, principalUUID, "dipole-gateway"), TenantId: c.tenantID,
		DefinitionId: definitionID, DefinitionVersion: definitionVersion,
	})
	if err != nil {
		return nil, mapAgentSubscriptionRPCError(err)
	}
	if response == nil {
		return nil, ErrAgentSubscriptionUnavailable
	}
	result := &AgentSubscriptionConversationOptions{Conversations: make([]AgentSubscriptionConversationOption, 0, len(response.GetConversations()))}
	seen := make(map[string]struct{}, len(response.GetConversations()))
	for _, raw := range response.GetConversations() {
		if raw == nil || !validAgentSubscriptionPublicID(raw.GetConversationKey(), 128) {
			return nil, ErrAgentSubscriptionUnavailable
		}
		expectedEventType, ok := agentSubscriptionConversationEventType(raw.GetConversationKey())
		if !ok || raw.GetEventType() != expectedEventType {
			return nil, ErrAgentSubscriptionUnavailable
		}
		if _, duplicated := seen[raw.GetConversationKey()]; duplicated {
			return nil, ErrAgentSubscriptionUnavailable
		}
		seen[raw.GetConversationKey()] = struct{}{}
		result.Conversations = append(result.Conversations, AgentSubscriptionConversationOption{ConversationKey: raw.GetConversationKey(), EventType: raw.GetEventType()})
	}
	return result, nil
}

func (c *AgentSubscriptionControlClient) Create(ctx context.Context, principalUUID string, input AgentSubscriptionCreateInput) (*AgentSubscription, error) {
	input.DefinitionID, input.ConversationKey, input.FilterKind = strings.TrimSpace(input.DefinitionID), strings.TrimSpace(input.ConversationKey), strings.TrimSpace(input.FilterKind)
	eventType, ok := agentSubscriptionConversationEventType(input.ConversationKey)
	if !validAgentSubscriptionPublicID(input.DefinitionID, 64) || input.DefinitionVersion == 0 || !ok {
		return nil, ErrAgentSubscriptionInvalid
	}
	filterJSON, err := canonicalGatewayAgentSubscriptionFilter(input.FilterKind, input.Filter)
	if err != nil {
		return nil, err
	}
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := c.rpc.CreateEventSubscription(callCtx, &agentv1.CreateEventSubscriptionRequest{
		Context: grpccommon.RequestContextFrom(ctx, principalUUID, "dipole-gateway"), TenantId: c.tenantID,
		DefinitionId: input.DefinitionID, DefinitionVersion: input.DefinitionVersion,
		EventType: eventType, ResourceType: "conversation", ResourceId: input.ConversationKey,
		FilterKind: input.FilterKind, FilterJson: filterJSON,
	})
	if err != nil {
		return nil, mapAgentSubscriptionRPCError(err)
	}
	item, err := agentSubscriptionFromProto(response, c.tenantID)
	if err != nil || item.DefinitionID != input.DefinitionID || item.DefinitionVersion != input.DefinitionVersion ||
		item.ResourceID != input.ConversationKey || item.EventType != eventType {
		return nil, ErrAgentSubscriptionUnavailable
	}
	return item, nil
}

func canonicalGatewayAgentSubscriptionFilter(kind string, filter AgentSubscriptionFilter) ([]byte, error) {
	switch kind {
	case "all":
		if len(filter.Terms) != 0 {
			return nil, ErrAgentSubscriptionInvalid
		}
		return []byte(`{}`), nil
	case "message_contains_any":
		if len(filter.Terms) == 0 || len(filter.Terms) > 32 {
			return nil, ErrAgentSubscriptionInvalid
		}
		terms := make([]string, 0, len(filter.Terms))
		seen := make(map[string]struct{}, len(filter.Terms))
		for _, raw := range filter.Terms {
			term := strings.TrimSpace(raw)
			if term == "" || len([]rune(term)) > 64 || term != raw {
				return nil, ErrAgentSubscriptionInvalid
			}
			if _, exists := seen[term]; exists {
				return nil, ErrAgentSubscriptionInvalid
			}
			seen[term] = struct{}{}
			terms = append(terms, term)
		}
		return json.Marshal(AgentSubscriptionFilter{Terms: terms})
	default:
		return nil, ErrAgentSubscriptionInvalid
	}
}

func agentSubscriptionConversationEventType(conversationKey string) (string, bool) {
	parts := strings.Split(conversationKey, ":")
	switch {
	case len(parts) == 3 && parts[0] == "direct" && parts[1] != "" && parts[2] != "":
		return "message.direct.created", true
	case len(parts) == 2 && parts[0] == "group" && parts[1] != "":
		return "message.group.created", true
	default:
		return "", false
	}
}

func (c *AgentSubscriptionControlClient) ListDefinitions(ctx context.Context, principalUUID, after string, limit int) (*AgentDefinitionCatalogPage, error) {
	afterID, afterVersion, err := decodeAgentDefinitionCursor(after)
	if err != nil {
		return nil, ErrAgentSubscriptionInvalid
	}
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := c.rpc.ListAgentDefinitions(callCtx, &agentv1.ListAgentDefinitionsRequest{
		Context: grpccommon.RequestContextFrom(ctx, principalUUID, "dipole-gateway"), TenantId: c.tenantID,
		AfterDefinitionId: afterID, AfterVersion: afterVersion, Limit: uint32(limit),
	})
	if err != nil {
		return nil, mapAgentSubscriptionRPCError(err)
	}
	if response == nil || (response.GetNextDefinitionId() == "") != (response.GetNextVersion() == 0) {
		return nil, ErrAgentSubscriptionUnavailable
	}
	page := &AgentDefinitionCatalogPage{Definitions: make([]AgentDefinitionCatalogItem, 0, len(response.GetDefinitions()))}
	for _, raw := range response.GetDefinitions() {
		if raw == nil || !validAgentSubscriptionPublicID(raw.GetDefinitionId(), 64) || raw.GetVersion() == 0 ||
			!validAgentSubscriptionPublicID(raw.GetAgentId(), 24) || len(raw.GetConversationScopes()) == 0 ||
			raw.GetValidFromUnixMs() <= 0 || raw.GetCreatedAtUnixMs() <= 0 || raw.GetUpdatedAtUnixMs() <= 0 {
			return nil, ErrAgentSubscriptionUnavailable
		}
		scopes := append([]string(nil), raw.GetConversationScopes()...)
		for _, scope := range scopes {
			if scope != "*" && !validAgentSubscriptionPublicID(scope, 128) {
				return nil, ErrAgentSubscriptionUnavailable
			}
		}
		page.Definitions = append(page.Definitions, AgentDefinitionCatalogItem{
			DefinitionID: raw.GetDefinitionId(), Version: raw.GetVersion(), AgentID: raw.GetAgentId(), ConversationScopes: scopes,
			ValidFromUnixMS: raw.GetValidFromUnixMs(), ExpiresAtUnixMS: raw.GetExpiresAtUnixMs(),
			CreatedAtUnixMS: raw.GetCreatedAtUnixMs(), UpdatedAtUnixMS: raw.GetUpdatedAtUnixMs(),
		})
	}
	if response.GetNextDefinitionId() != "" {
		page.NextCursor, err = encodeAgentDefinitionCursor(response.GetNextDefinitionId(), response.GetNextVersion())
		if err != nil {
			return nil, ErrAgentSubscriptionUnavailable
		}
	}
	return page, nil
}

func encodeAgentDefinitionCursor(definitionID string, version uint64) (string, error) {
	if !validAgentSubscriptionPublicID(definitionID, 64) || version == 0 {
		return "", ErrAgentSubscriptionInvalid
	}
	encoded, err := json.Marshal(struct {
		DefinitionID string `json:"definitionId"`
		Version      uint64 `json:"version"`
	}{DefinitionID: definitionID, Version: version})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

func decodeAgentDefinitionCursor(cursor string) (string, uint64, error) {
	if cursor == "" {
		return "", 0, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil || len(decoded) > 256 {
		return "", 0, ErrAgentSubscriptionInvalid
	}
	var value struct {
		DefinitionID string `json:"definitionId"`
		Version      uint64 `json:"version"`
	}
	if decodeStrictAgentSubscriptionJSON(decoded, &value) != nil || !validAgentSubscriptionPublicID(value.DefinitionID, 64) || value.Version == 0 {
		return "", 0, ErrAgentSubscriptionInvalid
	}
	canonical, err := encodeAgentDefinitionCursor(value.DefinitionID, value.Version)
	if err != nil || canonical != cursor {
		return "", 0, ErrAgentSubscriptionInvalid
	}
	return value.DefinitionID, value.Version, nil
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

func decodeStrictAgentSubscriptionBody(reader io.Reader, target any) error {
	raw, err := io.ReadAll(io.LimitReader(reader, 64*1024+1))
	if err != nil || len(raw) > 64*1024 {
		return ErrAgentSubscriptionInvalid
	}
	return decodeStrictAgentSubscriptionJSON(raw, target)
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
