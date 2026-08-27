package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/JekYUlll/Dipole/internal/application"
)

type agentSubscriptionDefinitionReaderV1 interface {
	GetDefinitionVersion(ctx context.Context, definitionUUID string, version uint64) (*application.AgentDefinitionVersionV1, error)
}

type PersistentAgentEventSubscriptionResolverV1 struct {
	store       application.AgentEventSubscriptionStoreV1
	definitions agentSubscriptionDefinitionReaderV1
	now         func() time.Time
}

func NewPersistentAgentEventSubscriptionResolverV1(store application.AgentEventSubscriptionStoreV1, definitions agentSubscriptionDefinitionReaderV1, now func() time.Time) (*PersistentAgentEventSubscriptionResolverV1, error) {
	if store == nil || definitions == nil {
		return nil, errors.New("Agent Event Subscription store and Definition reader are required")
	}
	if now == nil {
		now = time.Now
	}
	return &PersistentAgentEventSubscriptionResolverV1{store: store, definitions: definitions, now: now}, nil
}

func (r *PersistentAgentEventSubscriptionResolverV1) MatchEventSubscriptions(ctx context.Context, request application.AgentEventSubscriptionMatchRequestV1) ([]application.AgentEventSubscriptionV1, error) {
	request.TenantID, request.AgentUUID = strings.TrimSpace(request.TenantID), strings.TrimSpace(request.AgentUUID)
	request.EventType, request.ResourceType, request.ResourceID = strings.TrimSpace(request.EventType), strings.TrimSpace(request.ResourceType), strings.TrimSpace(request.ResourceID)
	if request.TenantID == "" || request.AgentUUID == "" || request.EventType == "" || request.ResourceType == "" || request.ResourceID == "" ||
		utf8.RuneCountInString(request.TenantID) > 64 || utf8.RuneCountInString(request.AgentUUID) > 24 ||
		utf8.RuneCountInString(request.EventType) > 64 || utf8.RuneCountInString(request.ResourceType) > 64 ||
		utf8.RuneCountInString(request.ResourceID) > 128 {
		return nil, application.ErrAgentSubscriptionInvalid
	}
	items, err := r.store.ListMatchingEventSubscriptions(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("list Agent Event Subscriptions: %w", err)
	}
	activeAt := r.now().UTC()
	for index := range items {
		item := items[index]
		if item.Validate() != nil || item.Status != application.AgentSubscriptionStatusActive || item.RevokedAt != nil ||
			item.TenantID != request.TenantID || item.AgentUUID != request.AgentUUID || item.EventType != request.EventType ||
			item.ResourceType != request.ResourceType || (item.ResourceID != "*" && item.ResourceID != request.ResourceID) {
			return nil, application.ErrAgentSubscriptionInvalid
		}
		definition, lookupErr := r.definitions.GetDefinitionVersion(ctx, item.DefinitionUUID, item.DefinitionVersion)
		if lookupErr != nil || !validSubscriptionDefinitionV1(definition, item, request, activeAt) {
			return nil, application.ErrAgentSubscriptionInvalid
		}
	}
	sort.Slice(items, func(left, right int) bool { return items[left].SubscriptionUUID < items[right].SubscriptionUUID })
	return items, nil
}

func validSubscriptionDefinitionV1(definition *application.AgentDefinitionVersionV1, item application.AgentEventSubscriptionV1, request application.AgentEventSubscriptionMatchRequestV1, activeAt time.Time) bool {
	if definition == nil || definition.Validate() != nil || definition.Status != application.AgentDefinitionStatusActive || definition.RevokedAt != nil ||
		definition.DefinitionUUID != item.DefinitionUUID || definition.Version != item.DefinitionVersion ||
		definition.TenantID != request.TenantID || definition.AgentUUID != request.AgentUUID || activeAt.Before(definition.ValidFrom) ||
		(definition.ExpiresAt != nil && !activeAt.Before(*definition.ExpiresAt)) {
		return false
	}
	hasReadPermission := false
	for _, permission := range definition.Permissions {
		if strings.TrimSpace(permission) == application.AgentPermissionConversationRead {
			hasReadPermission = true
			break
		}
	}
	if !hasReadPermission {
		return false
	}
	for _, scope := range definition.Scopes {
		if scope.ResourceType != request.ResourceType || (scope.ResourceID != "*" && scope.ResourceID != request.ResourceID) {
			continue
		}
		for _, action := range scope.Actions {
			if action = strings.TrimSpace(action); action == application.AgentResourceActionRead || action == application.AgentResourceWildcard {
				return true
			}
		}
	}
	return false
}
