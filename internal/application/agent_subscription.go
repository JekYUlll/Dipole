package application

import (
	"bytes"
	"context"
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
	CreatedAt         time.Time                     `json:"created_at,omitempty"`
	RevokedAt         *time.Time                    `json:"revoked_at,omitempty"`
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
	CreateEventSubscription(ctx context.Context, subscription AgentEventSubscriptionV1) error
	GetEventSubscription(ctx context.Context, subscriptionUUID string) (*AgentEventSubscriptionV1, error)
	ListMatchingEventSubscriptions(ctx context.Context, request AgentEventSubscriptionMatchRequestV1) ([]AgentEventSubscriptionV1, error)
	RevokeEventSubscription(ctx context.Context, subscriptionUUID string, revokedAt time.Time) error
}

func (s AgentEventSubscriptionV1) Validate() error {
	if anyBlank(s.SubscriptionUUID, s.DefinitionUUID, s.TenantID, s.AgentUUID, s.EventType, s.ResourceType, s.ResourceID) ||
		s.DefinitionVersion == 0 || utf8.RuneCountInString(strings.TrimSpace(s.SubscriptionUUID)) > 64 ||
		utf8.RuneCountInString(strings.TrimSpace(s.DefinitionUUID)) > 64 || utf8.RuneCountInString(strings.TrimSpace(s.TenantID)) > 64 ||
		utf8.RuneCountInString(strings.TrimSpace(s.AgentUUID)) > 24 || utf8.RuneCountInString(strings.TrimSpace(s.EventType)) > 64 ||
		utf8.RuneCountInString(strings.TrimSpace(s.ResourceType)) > 64 || utf8.RuneCountInString(strings.TrimSpace(s.ResourceID)) > 128 ||
		(s.Status != AgentSubscriptionStatusActive && s.Status != AgentSubscriptionStatusRevoked) {
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
