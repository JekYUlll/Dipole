package agentapplication

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/JekYUlll/Dipole/internal/application"
)

type AgentSubscriptionDefinitionReaderV1 interface {
	GetDefinitionVersion(ctx context.Context, definitionUUID string, version uint64) (*application.AgentDefinitionVersionV1, error)
}

type AgentSubscriptionConversationReaderV1 interface {
	ListSearchConversationKeys(userUUID string) ([]string, error)
}

type PersistentAgentEventSubscriptionResolverV1 struct {
	store       application.AgentEventSubscriptionStoreV1
	definitions AgentSubscriptionDefinitionReaderV1
	now         func() time.Time
}

type PersistentAgentEventSubscriptionControlV1 struct {
	store         application.AgentEventSubscriptionStoreV1
	definitions   AgentSubscriptionDefinitionReaderV1
	conversations AgentSubscriptionConversationReaderV1
	now           func() time.Time
}

func NewPersistentAgentEventSubscriptionControlV1(store application.AgentEventSubscriptionStoreV1, definitions AgentSubscriptionDefinitionReaderV1, conversations AgentSubscriptionConversationReaderV1, now func() time.Time) (*PersistentAgentEventSubscriptionControlV1, error) {
	if store == nil || definitions == nil || conversations == nil {
		return nil, errors.New("Agent Event Subscription store, Definition reader and conversation reader are required")
	}
	if now == nil {
		now = time.Now
	}
	return &PersistentAgentEventSubscriptionControlV1{store: store, definitions: definitions, conversations: conversations, now: now}, nil
}

func (s *PersistentAgentEventSubscriptionControlV1) Create(ctx context.Context, principalUUID string, request application.AgentEventSubscriptionCreateRequestV1) (*application.AgentEventSubscriptionV1, error) {
	principalUUID = strings.TrimSpace(principalUUID)
	request.TenantID, request.DefinitionUUID = strings.TrimSpace(request.TenantID), strings.TrimSpace(request.DefinitionUUID)
	request.EventType, request.ResourceType, request.ResourceID = strings.TrimSpace(request.EventType), strings.TrimSpace(request.ResourceType), strings.TrimSpace(request.ResourceID)
	if anySubscriptionControlBlankV1(principalUUID, request.TenantID, request.DefinitionUUID, request.EventType, request.ResourceType, request.ResourceID) ||
		request.DefinitionVersion == 0 || utf8.RuneCountInString(principalUUID) > 24 || request.ResourceType != "conversation" {
		return nil, application.ErrAgentSubscriptionInvalid
	}
	canonicalFilter, err := canonicalSubscriptionFilterV1(request.FilterKind, request.FilterJSON)
	if err != nil {
		return nil, err
	}
	request.FilterJSON = canonicalFilter
	definition, err := s.definitions.GetDefinitionVersion(ctx, request.DefinitionUUID, request.DefinitionVersion)
	if err != nil || definition == nil {
		return nil, application.ErrAgentSubscriptionDenied
	}
	item := application.AgentEventSubscriptionV1{
		DefinitionUUID: request.DefinitionUUID, DefinitionVersion: request.DefinitionVersion,
		TenantID: request.TenantID, AgentUUID: definition.AgentUUID, Status: application.AgentSubscriptionStatusActive,
		EventType: request.EventType, ResourceType: request.ResourceType, ResourceID: request.ResourceID,
		FilterKind: request.FilterKind, FilterJSON: canonicalFilter, CreatedByUUID: principalUUID,
	}
	item.SubscriptionUUID, err = application.AgentEventSubscriptionUUIDV1(request, canonicalFilter)
	if err != nil || item.Validate() != nil {
		return nil, application.ErrAgentSubscriptionInvalid
	}
	matchRequest := application.AgentEventSubscriptionMatchRequestV1{
		TenantID: request.TenantID, AgentUUID: definition.AgentUUID, EventType: request.EventType,
		ResourceType: request.ResourceType, ResourceID: request.ResourceID,
	}
	if definition.OwnerUUID != principalUUID || !ValidSubscriptionDefinitionV1(definition, item, matchRequest, s.now().UTC()) {
		return nil, application.ErrAgentSubscriptionDenied
	}
	expectedEventType, validKey := subscriptionConversationEventTypeV1(request.ResourceID)
	if !validKey || request.EventType != expectedEventType {
		return nil, application.ErrAgentSubscriptionInvalid
	}
	readable, err := s.readableConversationSetV1(principalUUID)
	if err != nil {
		return nil, err
	}
	if _, ok := readable[request.ResourceID]; !ok {
		return nil, application.ErrAgentSubscriptionDenied
	}
	_, err = s.store.CreateEventSubscription(ctx, item)
	if err != nil {
		return nil, fmt.Errorf("create Agent Event Subscription: %w", err)
	}
	stored, err := s.store.GetEventSubscription(ctx, item.SubscriptionUUID)
	if err != nil {
		return nil, fmt.Errorf("load Agent Event Subscription: %w", err)
	}
	if stored == nil || !sameAgentEventSubscriptionCreationV1(*stored, item) {
		return nil, application.ErrAgentSubscriptionConflict
	}
	return stored, nil
}

func (s *PersistentAgentEventSubscriptionControlV1) ListEligibleConversations(ctx context.Context, principalUUID string, request application.AgentSubscriptionConversationOptionsRequestV1) ([]application.AgentSubscriptionConversationOptionV1, error) {
	principalUUID, request.TenantID, request.DefinitionUUID = strings.TrimSpace(principalUUID), strings.TrimSpace(request.TenantID), strings.TrimSpace(request.DefinitionUUID)
	if anySubscriptionControlBlankV1(principalUUID, request.TenantID, request.DefinitionUUID) || request.DefinitionVersion == 0 ||
		utf8.RuneCountInString(principalUUID) > 24 || utf8.RuneCountInString(request.TenantID) > 64 || utf8.RuneCountInString(request.DefinitionUUID) > 64 {
		return nil, application.ErrAgentSubscriptionInvalid
	}
	definition, err := s.definitions.GetDefinitionVersion(ctx, request.DefinitionUUID, request.DefinitionVersion)
	if err != nil || definition == nil || definition.OwnerUUID != principalUUID {
		return nil, application.ErrAgentSubscriptionDenied
	}
	readable, err := s.readableConversationSetV1(principalUUID)
	if err != nil {
		return nil, err
	}
	items := make([]application.AgentSubscriptionConversationOptionV1, 0, len(readable))
	for conversationKey := range readable {
		eventType, ok := subscriptionConversationEventTypeV1(conversationKey)
		if !ok {
			return nil, application.ErrAgentSubscriptionConflict
		}
		item := application.AgentEventSubscriptionV1{
			DefinitionUUID: request.DefinitionUUID, DefinitionVersion: request.DefinitionVersion,
			TenantID: request.TenantID, AgentUUID: definition.AgentUUID,
			EventType: eventType, ResourceType: "conversation", ResourceID: conversationKey,
		}
		match := application.AgentEventSubscriptionMatchRequestV1{
			TenantID: request.TenantID, AgentUUID: definition.AgentUUID, EventType: eventType,
			ResourceType: "conversation", ResourceID: conversationKey,
		}
		if ValidSubscriptionDefinitionV1(definition, item, match, s.now().UTC()) {
			items = append(items, application.AgentSubscriptionConversationOptionV1{ConversationKey: conversationKey, EventType: eventType})
		}
	}
	sort.Slice(items, func(left, right int) bool { return items[left].ConversationKey < items[right].ConversationKey })
	return items, nil
}

func (s *PersistentAgentEventSubscriptionControlV1) readableConversationSetV1(principalUUID string) (map[string]struct{}, error) {
	keys, err := s.conversations.ListSearchConversationKeys(principalUUID)
	if err != nil {
		return nil, fmt.Errorf("list readable conversations: %w", err)
	}
	readable := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if _, ok := subscriptionConversationEventTypeV1(key); !ok {
			return nil, application.ErrAgentSubscriptionConflict
		}
		readable[key] = struct{}{}
	}
	return readable, nil
}

func subscriptionConversationEventTypeV1(conversationKey string) (string, bool) {
	if utf8.RuneCountInString(conversationKey) > 128 || strings.TrimSpace(conversationKey) != conversationKey {
		return "", false
	}
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

func (s *PersistentAgentEventSubscriptionControlV1) List(ctx context.Context, principalUUID string, request application.AgentEventSubscriptionListRequestV1) (*application.AgentEventSubscriptionPageV1, error) {
	principalUUID, request.TenantID, request.AfterUUID = strings.TrimSpace(principalUUID), strings.TrimSpace(request.TenantID), strings.TrimSpace(request.AfterUUID)
	if anySubscriptionControlBlankV1(principalUUID, request.TenantID) || utf8.RuneCountInString(principalUUID) > 24 ||
		utf8.RuneCountInString(request.TenantID) > 64 || utf8.RuneCountInString(request.AfterUUID) > 64 || request.Limit < 0 || request.Limit > 100 {
		return nil, application.ErrAgentSubscriptionInvalid
	}
	if request.Limit == 0 {
		request.Limit = 50
	}
	items, err := s.store.ListOwnedEventSubscriptions(ctx, request.TenantID, principalUUID, request.AfterUUID, request.Limit+1)
	if err != nil {
		return nil, fmt.Errorf("list owned Agent Event Subscriptions: %w", err)
	}
	sort.Slice(items, func(left, right int) bool { return items[left].SubscriptionUUID < items[right].SubscriptionUUID })
	page := &application.AgentEventSubscriptionPageV1{Subscriptions: items}
	if len(page.Subscriptions) > request.Limit {
		page.Subscriptions = page.Subscriptions[:request.Limit]
		page.NextCursor = page.Subscriptions[len(page.Subscriptions)-1].SubscriptionUUID
	}
	for _, item := range page.Subscriptions {
		if item.Validate() != nil || item.TenantID != request.TenantID || item.CreatedByUUID != principalUUID {
			return nil, application.ErrAgentSubscriptionConflict
		}
	}
	return page, nil
}

func (s *PersistentAgentEventSubscriptionControlV1) Revoke(ctx context.Context, principalUUID string, request application.AgentEventSubscriptionRevokeRequestV1) (*application.AgentEventSubscriptionV1, error) {
	principalUUID, request.TenantID = strings.TrimSpace(principalUUID), strings.TrimSpace(request.TenantID)
	request.SubscriptionUUID, request.Reason = strings.TrimSpace(request.SubscriptionUUID), strings.TrimSpace(request.Reason)
	if anySubscriptionControlBlankV1(principalUUID, request.TenantID, request.SubscriptionUUID, request.Reason) ||
		utf8.RuneCountInString(principalUUID) > 24 || utf8.RuneCountInString(request.TenantID) > 64 ||
		utf8.RuneCountInString(request.SubscriptionUUID) > 64 || utf8.RuneCountInString(request.Reason) > 1000 {
		return nil, application.ErrAgentSubscriptionInvalid
	}
	item, err := s.store.GetEventSubscription(ctx, request.SubscriptionUUID)
	if err != nil {
		return nil, fmt.Errorf("get Agent Event Subscription: %w", err)
	}
	if item == nil || item.TenantID != request.TenantID {
		return nil, application.ErrAgentSubscriptionDenied
	}
	definition, err := s.definitions.GetDefinitionVersion(ctx, item.DefinitionUUID, item.DefinitionVersion)
	if err != nil || definition == nil || definition.Validate() != nil || definition.TenantID != request.TenantID ||
		definition.AgentUUID != item.AgentUUID || definition.OwnerUUID != principalUUID || item.CreatedByUUID != principalUUID {
		return nil, application.ErrAgentSubscriptionDenied
	}
	if item.Status == application.AgentSubscriptionStatusRevoked {
		if item.RevokedByUUID == principalUUID && item.RevokeReason == request.Reason {
			return item, nil
		}
		return nil, application.ErrAgentSubscriptionConflict
	}
	if item.Validate() != nil {
		return nil, application.ErrAgentSubscriptionConflict
	}
	revokedAt := s.now().UTC()
	_, err = s.store.RevokeEventSubscription(ctx, item.SubscriptionUUID, principalUUID, request.Reason, revokedAt)
	if err != nil {
		return nil, fmt.Errorf("revoke Agent Event Subscription: %w", err)
	}
	stored, loadErr := s.store.GetEventSubscription(ctx, item.SubscriptionUUID)
	if loadErr != nil {
		return nil, fmt.Errorf("load revoked Agent Event Subscription: %w", loadErr)
	}
	if stored == nil || stored.Status != application.AgentSubscriptionStatusRevoked || stored.RevokedByUUID != principalUUID || stored.RevokeReason != request.Reason {
		return nil, application.ErrAgentSubscriptionConflict
	}
	return stored, nil
}

func canonicalSubscriptionFilterV1(kind application.AgentSubscriptionFilterKindV1, raw json.RawMessage) (json.RawMessage, error) {
	switch kind {
	case application.AgentSubscriptionFilterAll:
		var value struct{}
		if err := decodeStrictSubscriptionControlFilterV1(raw, &value); err != nil {
			return nil, application.ErrAgentSubscriptionInvalid
		}
		return json.RawMessage(`{}`), nil
	case application.AgentSubscriptionFilterMessageContainsAny:
		var value struct {
			Terms []string `json:"terms"`
		}
		if err := decodeStrictSubscriptionControlFilterV1(raw, &value); err != nil || len(value.Terms) == 0 || len(value.Terms) > 32 {
			return nil, application.ErrAgentSubscriptionInvalid
		}
		unique := make(map[string]struct{}, len(value.Terms))
		terms := make([]string, 0, len(value.Terms))
		for _, term := range value.Terms {
			term = strings.ToLower(strings.TrimSpace(term))
			if term == "" || utf8.RuneCountInString(term) > 64 || strings.IndexFunc(term, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
				return nil, application.ErrAgentSubscriptionInvalid
			}
			if _, exists := unique[term]; exists {
				continue
			}
			unique[term] = struct{}{}
			terms = append(terms, term)
		}
		sort.Strings(terms)
		encoded, err := json.Marshal(struct {
			Terms []string `json:"terms"`
		}{Terms: terms})
		if err != nil {
			return nil, application.ErrAgentSubscriptionInvalid
		}
		return encoded, nil
	default:
		return nil, application.ErrAgentSubscriptionInvalid
	}
}

func decodeStrictSubscriptionControlFilterV1(raw json.RawMessage, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return application.ErrAgentSubscriptionInvalid
	}
	return nil
}

func sameAgentEventSubscriptionCreationV1(left, right application.AgentEventSubscriptionV1) bool {
	return left.SubscriptionUUID == right.SubscriptionUUID && left.DefinitionUUID == right.DefinitionUUID && left.DefinitionVersion == right.DefinitionVersion &&
		left.TenantID == right.TenantID && left.AgentUUID == right.AgentUUID && left.Status == right.Status && left.EventType == right.EventType &&
		left.ResourceType == right.ResourceType && left.ResourceID == right.ResourceID && left.FilterKind == right.FilterKind && bytes.Equal(left.FilterJSON, right.FilterJSON) &&
		left.CreatedByUUID == right.CreatedByUUID && left.RevokedAt == nil
}

func anySubscriptionControlBlankV1(values ...string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return true
		}
	}
	return false
}

func NewPersistentAgentEventSubscriptionResolverV1(store application.AgentEventSubscriptionStoreV1, definitions AgentSubscriptionDefinitionReaderV1, now func() time.Time) (*PersistentAgentEventSubscriptionResolverV1, error) {
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
		if lookupErr != nil || !ValidSubscriptionDefinitionV1(definition, item, request, activeAt) {
			return nil, application.ErrAgentSubscriptionInvalid
		}
	}
	sort.Slice(items, func(left, right int) bool { return items[left].SubscriptionUUID < items[right].SubscriptionUUID })
	return items, nil
}

func ValidSubscriptionDefinitionV1(definition *application.AgentDefinitionVersionV1, item application.AgentEventSubscriptionV1, request application.AgentEventSubscriptionMatchRequestV1, activeAt time.Time) bool {
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
