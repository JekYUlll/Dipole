package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var ErrAgentSubscriptionInvalid = errors.New("Agent Event Subscription is invalid")
var ErrAgentSubscriptionDenied = errors.New("Agent Event Subscription access denied")
var ErrAgentSubscriptionConflict = errors.New("Agent Event Subscription conflict")

type AgentSubscriptionStatusV1 string
type AgentSubscriptionFilterKindV1 string

const (
	AgentSubscriptionStatusActive  AgentSubscriptionStatusV1 = "active"
	AgentSubscriptionStatusRevoked AgentSubscriptionStatusV1 = "revoked"

	AgentSubscriptionFilterAll                AgentSubscriptionFilterKindV1 = "all"
	AgentSubscriptionFilterMessageContainsAny AgentSubscriptionFilterKindV1 = "message_contains_any"
)

type AgentEventSubscriptionV1 struct {
	SubscriptionUUID  string                        `json:"subscription_uuid"`
	DefinitionUUID    string                        `json:"definition_uuid"`
	DefinitionVersion uint64                        `json:"definition_version"`
	TenantID          string                        `json:"tenant_id"`
	AgentUUID         string                        `json:"agent_uuid"`
	Status            AgentSubscriptionStatusV1     `json:"status"`
	EventType         string                        `json:"event_type"`
	ResourceType      string                        `json:"resource_type"`
	ResourceID        string                        `json:"resource_id"`
	FilterKind        AgentSubscriptionFilterKindV1 `json:"filter_kind"`
	FilterJSON        json.RawMessage               `json:"filter"`
	CreatedByUUID     string                        `json:"created_by_uuid"`
	RevokedByUUID     string                        `json:"revoked_by_uuid,omitempty"`
	RevokeReason      string                        `json:"revoke_reason,omitempty"`
	CreatedAt         time.Time                     `json:"created_at,omitempty"`
	UpdatedAt         time.Time                     `json:"updated_at,omitempty"`
	RevokedAt         *time.Time                    `json:"revoked_at,omitempty"`
}

type AgentEventSubscriptionCreateRequestV1 struct {
	TenantID          string
	DefinitionUUID    string
	DefinitionVersion uint64
	EventType         string
	ResourceType      string
	ResourceID        string
	FilterKind        AgentSubscriptionFilterKindV1
	FilterJSON        json.RawMessage
}

type AgentEventSubscriptionListRequestV1 struct {
	TenantID  string
	AfterUUID string
	Limit     int
}

type AgentEventSubscriptionRevokeRequestV1 struct {
	TenantID         string
	SubscriptionUUID string
	Reason           string
}

type AgentEventSubscriptionPageV1 struct {
	Subscriptions []AgentEventSubscriptionV1
	NextCursor    string
}

type AgentSubscriptionConversationOptionV1 struct {
	ConversationKey string
	EventType       string
}

type AgentSubscriptionConversationOptionsRequestV1 struct {
	TenantID          string
	DefinitionUUID    string
	DefinitionVersion uint64
}

type AgentEventSubscriptionControlServiceV1 interface {
	Create(ctx context.Context, principalUUID string, request AgentEventSubscriptionCreateRequestV1) (*AgentEventSubscriptionV1, error)
	ListEligibleConversations(ctx context.Context, principalUUID string, request AgentSubscriptionConversationOptionsRequestV1) ([]AgentSubscriptionConversationOptionV1, error)
	List(ctx context.Context, principalUUID string, request AgentEventSubscriptionListRequestV1) (*AgentEventSubscriptionPageV1, error)
	Revoke(ctx context.Context, principalUUID string, request AgentEventSubscriptionRevokeRequestV1) (*AgentEventSubscriptionV1, error)
}

type AgentEventSubscriptionMatchRequestV1 struct {
	TenantID     string
	AgentUUID    string
	EventType    string
	ResourceType string
	ResourceID   string
}

type AgentEventSubscriptionResolverV1 interface {
	MatchEventSubscriptions(ctx context.Context, request AgentEventSubscriptionMatchRequestV1) ([]AgentEventSubscriptionV1, error)
}

type AgentEventSubscriptionStoreV1 interface {
	CreateEventSubscription(ctx context.Context, subscription AgentEventSubscriptionV1) (bool, error)
	GetEventSubscription(ctx context.Context, subscriptionUUID string) (*AgentEventSubscriptionV1, error)
	ListMatchingEventSubscriptions(ctx context.Context, request AgentEventSubscriptionMatchRequestV1) ([]AgentEventSubscriptionV1, error)
	ListOwnedEventSubscriptions(ctx context.Context, tenantID, ownerUUID, afterUUID string, limit int) ([]AgentEventSubscriptionV1, error)
	RevokeEventSubscription(ctx context.Context, subscriptionUUID, revokedByUUID, reason string, revokedAt time.Time) (bool, error)
}

func (s AgentEventSubscriptionV1) Validate() error {
	if anyBlank(s.SubscriptionUUID, s.DefinitionUUID, s.TenantID, s.AgentUUID, s.EventType, s.ResourceType, s.ResourceID, s.CreatedByUUID) ||
		s.DefinitionVersion == 0 || utf8.RuneCountInString(strings.TrimSpace(s.SubscriptionUUID)) > 64 ||
		utf8.RuneCountInString(strings.TrimSpace(s.DefinitionUUID)) > 64 || utf8.RuneCountInString(strings.TrimSpace(s.TenantID)) > 64 ||
		utf8.RuneCountInString(strings.TrimSpace(s.AgentUUID)) > 24 || utf8.RuneCountInString(strings.TrimSpace(s.EventType)) > 64 ||
		utf8.RuneCountInString(strings.TrimSpace(s.ResourceType)) > 64 || utf8.RuneCountInString(strings.TrimSpace(s.ResourceID)) > 128 ||
		utf8.RuneCountInString(strings.TrimSpace(s.CreatedByUUID)) > 24 || utf8.RuneCountInString(strings.TrimSpace(s.RevokedByUUID)) > 24 ||
		utf8.RuneCountInString(strings.TrimSpace(s.RevokeReason)) > 1000 ||
		(s.Status != AgentSubscriptionStatusActive && s.Status != AgentSubscriptionStatusRevoked) ||
		(s.Status == AgentSubscriptionStatusActive && (s.RevokedAt != nil || strings.TrimSpace(s.RevokedByUUID) != "" || strings.TrimSpace(s.RevokeReason) != "")) ||
		(s.Status == AgentSubscriptionStatusRevoked && (s.RevokedAt == nil || anyBlank(s.RevokedByUUID, s.RevokeReason))) {
		return ErrAgentSubscriptionInvalid
	}
	switch s.FilterKind {
	case AgentSubscriptionFilterAll:
		var filter struct{}
		if err := decodeStrictSubscriptionFilter(s.FilterJSON, &filter); err != nil {
			return err
		}
	case AgentSubscriptionFilterMessageContainsAny:
		var filter struct {
			Terms []string `json:"terms"`
		}
		if err := decodeStrictSubscriptionFilter(s.FilterJSON, &filter); err != nil || len(filter.Terms) == 0 || len(filter.Terms) > 32 {
			return ErrAgentSubscriptionInvalid
		}
		for _, term := range filter.Terms {
			term = strings.TrimSpace(term)
			if term == "" || utf8.RuneCountInString(term) > 64 || strings.IndexFunc(term, unicode.IsControl) >= 0 {
				return ErrAgentSubscriptionInvalid
			}
		}
	default:
		return ErrAgentSubscriptionInvalid
	}
	return nil
}

func AgentEventSubscriptionUUIDV1(request AgentEventSubscriptionCreateRequestV1, canonicalFilter json.RawMessage) (string, error) {
	payload := struct {
		Schema            string                        `json:"schema"`
		TenantID          string                        `json:"tenant_id"`
		DefinitionUUID    string                        `json:"definition_uuid"`
		DefinitionVersion uint64                        `json:"definition_version"`
		EventType         string                        `json:"event_type"`
		ResourceType      string                        `json:"resource_type"`
		ResourceID        string                        `json:"resource_id"`
		FilterKind        AgentSubscriptionFilterKindV1 `json:"filter_kind"`
		Filter            json.RawMessage               `json:"filter"`
	}{
		Schema: "dipole.agent.event-subscription.v1", TenantID: strings.TrimSpace(request.TenantID),
		DefinitionUUID: strings.TrimSpace(request.DefinitionUUID), DefinitionVersion: request.DefinitionVersion,
		EventType: strings.TrimSpace(request.EventType), ResourceType: strings.TrimSpace(request.ResourceType),
		ResourceID: strings.TrimSpace(request.ResourceID), FilterKind: request.FilterKind, Filter: canonicalFilter,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode Agent Event Subscription identity: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return fmt.Sprintf("%x", digest[:]), nil
}

func decodeStrictSubscriptionFilter(raw json.RawMessage, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("%w: %v", ErrAgentSubscriptionInvalid, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return ErrAgentSubscriptionInvalid
	}
	return nil
}
