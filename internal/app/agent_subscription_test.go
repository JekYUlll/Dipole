package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

type subscriptionStoreStub struct {
	items []application.AgentEventSubscriptionV1
}

func (s *subscriptionStoreStub) CreateEventSubscription(_ context.Context, item application.AgentEventSubscriptionV1) (bool, error) {
	for _, existing := range s.items {
		if existing.SubscriptionUUID == item.SubscriptionUUID {
			return false, nil
		}
	}
	s.items = append(s.items, item)
	return true, nil
}
func (s *subscriptionStoreStub) GetEventSubscription(_ context.Context, id string) (*application.AgentEventSubscriptionV1, error) {
	for _, item := range s.items {
		if item.SubscriptionUUID == id {
			copy := item
			return &copy, nil
		}
	}
	return nil, nil
}
func (s *subscriptionStoreStub) ListMatchingEventSubscriptions(context.Context, application.AgentEventSubscriptionMatchRequestV1) ([]application.AgentEventSubscriptionV1, error) {
	return append([]application.AgentEventSubscriptionV1(nil), s.items...), nil
}
func (s *subscriptionStoreStub) ListOwnedEventSubscriptions(_ context.Context, tenantID, ownerUUID, after string, limit int) ([]application.AgentEventSubscriptionV1, error) {
	var result []application.AgentEventSubscriptionV1
	for _, item := range s.items {
		if item.TenantID == tenantID && item.CreatedByUUID == ownerUUID && item.SubscriptionUUID > after {
			result = append(result, item)
		}
	}
	return result, nil
}
func (s *subscriptionStoreStub) RevokeEventSubscription(_ context.Context, id, actor, reason string, at time.Time) (bool, error) {
	for index := range s.items {
		if s.items[index].SubscriptionUUID != id || s.items[index].Status != application.AgentSubscriptionStatusActive {
			continue
		}
		s.items[index].Status = application.AgentSubscriptionStatusRevoked
		s.items[index].RevokedByUUID, s.items[index].RevokeReason = actor, reason
		s.items[index].RevokedAt, s.items[index].UpdatedAt = &at, at
		return true, nil
	}
	return false, nil
}

type subscriptionDefinitionReaderStub struct {
	definition application.AgentDefinitionVersionV1
}

type subscriptionConversationReaderStub struct {
	keys []string
	err  error
}

func (s subscriptionConversationReaderStub) ListSearchConversationKeys(string) ([]string, error) {
	return append([]string(nil), s.keys...), s.err
}

func readableSubscriptionConversations() subscriptionConversationReaderStub {
	return subscriptionConversationReaderStub{keys: []string{"group:G1", "group:G9", "direct:U100:U200"}}
}

func (s subscriptionDefinitionReaderStub) GetDefinitionVersion(context.Context, string, uint64) (*application.AgentDefinitionVersionV1, error) {
	copy := s.definition
	return &copy, nil
}

func TestPersistentAgentEventSubscriptionResolverAuthorizesDefinitionScope(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	store := &subscriptionStoreStub{items: []application.AgentEventSubscriptionV1{
		subscriptionFixture("SUB-B"), subscriptionFixture("SUB-A"),
	}}
	resolver, err := NewPersistentAgentEventSubscriptionResolverV1(store, subscriptionDefinitionReaderStub{definition: subscriptionDefinitionFixture(now)}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("new resolver: %v", err)
	}
	items, err := resolver.MatchEventSubscriptions(context.Background(), application.AgentEventSubscriptionMatchRequestV1{
		TenantID: "dipole", AgentUUID: "UAI", EventType: "message.group.created",
		ResourceType: "conversation", ResourceID: "group:G1",
	})
	if err != nil {
		t.Fatalf("match subscriptions: %v", err)
	}
	if len(items) != 2 || items[0].SubscriptionUUID != "SUB-A" || items[1].SubscriptionUUID != "SUB-B" {
		t.Fatalf("unexpected subscriptions: %+v", items)
	}
}

func TestPersistentAgentEventSubscriptionResolverFailsClosedForScopeOrDefinitionDrift(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		mutateItem func(*application.AgentEventSubscriptionV1)
		mutateDef  func(*application.AgentDefinitionVersionV1)
	}{
		{name: "tenant drift", mutateItem: func(item *application.AgentEventSubscriptionV1) { item.TenantID = "other" }},
		{name: "definition drift", mutateDef: func(def *application.AgentDefinitionVersionV1) { def.DefinitionUUID = "OTHER" }},
		{name: "permission denied", mutateDef: func(def *application.AgentDefinitionVersionV1) {
			def.Permissions = []string{application.AgentPermissionConversationList}
		}},
		{name: "scope denied", mutateDef: func(def *application.AgentDefinitionVersionV1) { def.Scopes[0].ResourceID = "group:G2" }},
		{name: "expired", mutateDef: func(def *application.AgentDefinitionVersionV1) { expired := now; def.ExpiresAt = &expired }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			item, definition := subscriptionFixture("SUB-1"), subscriptionDefinitionFixture(now)
			if test.mutateItem != nil {
				test.mutateItem(&item)
			}
			if test.mutateDef != nil {
				test.mutateDef(&definition)
			}
			resolver, _ := NewPersistentAgentEventSubscriptionResolverV1(&subscriptionStoreStub{items: []application.AgentEventSubscriptionV1{item}}, subscriptionDefinitionReaderStub{definition: definition}, func() time.Time { return now })
			_, err := resolver.MatchEventSubscriptions(context.Background(), application.AgentEventSubscriptionMatchRequestV1{
				TenantID: "dipole", AgentUUID: "UAI", EventType: "message.group.created", ResourceType: "conversation", ResourceID: "group:G1",
			})
			if err == nil {
				t.Fatal("expected fail-closed resolver error")
			}
		})
	}
}

func TestPersistentAgentEventSubscriptionResolverRejectsOversizedRequest(t *testing.T) {
	t.Parallel()

	resolver, _ := NewPersistentAgentEventSubscriptionResolverV1(
		&subscriptionStoreStub{}, subscriptionDefinitionReaderStub{}, time.Now,
	)
	_, err := resolver.MatchEventSubscriptions(context.Background(), application.AgentEventSubscriptionMatchRequestV1{
		TenantID: "dipole", AgentUUID: strings.Repeat("a", 25), EventType: "message.direct.created",
		ResourceType: "conversation", ResourceID: "group:G1",
	})
	if err == nil {
		t.Fatal("oversized Subscription request should be rejected")
	}
}

func TestPersistentAgentEventSubscriptionControlCreatesCanonicalReplay(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 28, 8, 0, 0, 0, time.UTC)
	store := &subscriptionStoreStub{}
	service, err := NewPersistentAgentEventSubscriptionControlV1(store, subscriptionDefinitionReaderStub{definition: subscriptionDefinitionFixture(now)}, readableSubscriptionConversations(), func() time.Time { return now })
	if err != nil {
		t.Fatalf("new control: %v", err)
	}
	request := application.AgentEventSubscriptionCreateRequestV1{
		TenantID: "dipole", DefinitionUUID: "DEF-1", DefinitionVersion: 1,
		EventType: "message.group.created", ResourceType: "conversation", ResourceID: "group:G1",
		FilterKind: application.AgentSubscriptionFilterMessageContainsAny,
		FilterJSON: json.RawMessage(`{"terms":[" Incident ","延期","incident"]}`),
	}
	created, err := service.Create(context.Background(), "U100", request)
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	if created.SubscriptionUUID == "" || string(created.FilterJSON) != `{"terms":["incident","延期"]}` || created.CreatedByUUID != "U100" {
		t.Fatalf("unexpected canonical subscription: %+v", created)
	}
	replayed, err := service.Create(context.Background(), "U100", request)
	if err != nil || replayed.SubscriptionUUID != created.SubscriptionUUID || len(store.items) != 1 {
		t.Fatalf("exact replay: item=%+v count=%d err=%v", replayed, len(store.items), err)
	}

	reordered := request
	reordered.FilterJSON = json.RawMessage(`{"terms":["延期","INCIDENT"]}`)
	replayed, err = service.Create(context.Background(), "U100", reordered)
	if err != nil || replayed.SubscriptionUUID != created.SubscriptionUUID || len(store.items) != 1 {
		t.Fatalf("set-equivalent replay: item=%+v count=%d err=%v", replayed, len(store.items), err)
	}
}

func TestPersistentAgentEventSubscriptionControlEnforcesOwnerAndScope(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 28, 8, 0, 0, 0, time.UTC)
	request := application.AgentEventSubscriptionCreateRequestV1{
		TenantID: "dipole", DefinitionUUID: "DEF-1", DefinitionVersion: 1,
		EventType: "message.group.created", ResourceType: "conversation", ResourceID: "group:G1",
		FilterKind: application.AgentSubscriptionFilterAll, FilterJSON: json.RawMessage(`{}`),
	}
	for _, test := range []struct {
		name      string
		principal string
		mutate    func(*application.AgentDefinitionVersionV1)
	}{
		{name: "different owner", principal: "U999"},
		{name: "different tenant", principal: "U100", mutate: func(value *application.AgentDefinitionVersionV1) { value.TenantID = "other" }},
		{name: "scope denied", principal: "U100", mutate: func(value *application.AgentDefinitionVersionV1) { value.Scopes[0].ResourceID = "group:G2" }},
		{name: "revoked definition", principal: "U100", mutate: func(value *application.AgentDefinitionVersionV1) {
			value.Status = application.AgentDefinitionStatusRevoked
			value.RevokedAt = &now
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			definition := subscriptionDefinitionFixture(now)
			if test.mutate != nil {
				test.mutate(&definition)
			}
			service, _ := NewPersistentAgentEventSubscriptionControlV1(&subscriptionStoreStub{}, subscriptionDefinitionReaderStub{definition: definition}, readableSubscriptionConversations(), func() time.Time { return now })
			if _, err := service.Create(context.Background(), test.principal, request); !errors.Is(err, application.ErrAgentSubscriptionDenied) {
				t.Fatalf("expected denied, got %v", err)
			}
		})
	}
}

func TestPersistentAgentEventSubscriptionControlRequiresReadableConversation(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 28, 8, 0, 0, 0, time.UTC)
	definition := subscriptionDefinitionFixture(now)
	definition.Scopes[0].ResourceID = "*"
	service, err := NewPersistentAgentEventSubscriptionControlV1(
		&subscriptionStoreStub{}, subscriptionDefinitionReaderStub{definition: definition},
		subscriptionConversationReaderStub{keys: []string{"group:G2"}}, func() time.Time { return now },
	)
	if err != nil {
		t.Fatalf("new control: %v", err)
	}
	_, err = service.Create(context.Background(), "U100", application.AgentEventSubscriptionCreateRequestV1{
		TenantID: "dipole", DefinitionUUID: "DEF-1", DefinitionVersion: 1,
		EventType: "message.group.created", ResourceType: "conversation", ResourceID: "group:G1",
		FilterKind: application.AgentSubscriptionFilterAll, FilterJSON: json.RawMessage(`{}`),
	})
	if !errors.Is(err, application.ErrAgentSubscriptionDenied) {
		t.Fatalf("unreadable conversation error = %v, want denied", err)
	}
}

func TestPersistentAgentEventSubscriptionControlListsEligibleConversationIntersection(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 28, 8, 0, 0, 0, time.UTC)
	definition := subscriptionDefinitionFixture(now)
	definition.Scopes = append(definition.Scopes, application.AgentResourceScopeV1{
		ResourceType: "conversation", ResourceID: "direct:U100:U200", Actions: []string{application.AgentResourceActionRead},
	})
	service, err := NewPersistentAgentEventSubscriptionControlV1(
		&subscriptionStoreStub{}, subscriptionDefinitionReaderStub{definition: definition}, readableSubscriptionConversations(), func() time.Time { return now },
	)
	if err != nil {
		t.Fatalf("new control: %v", err)
	}
	items, err := service.ListEligibleConversations(context.Background(), "U100", application.AgentSubscriptionConversationOptionsRequestV1{
		TenantID: "dipole", DefinitionUUID: "DEF-1", DefinitionVersion: 1,
	})
	if err != nil {
		t.Fatalf("list eligible conversations: %v", err)
	}
	if len(items) != 2 || items[0].ConversationKey != "direct:U100:U200" || items[0].EventType != "message.direct.created" ||
		items[1].ConversationKey != "group:G1" || items[1].EventType != "message.group.created" {
		t.Fatalf("unexpected eligible conversations: %+v", items)
	}
}

func TestPersistentAgentEventSubscriptionControlListsAndRevokesWithAuditReplay(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 28, 8, 0, 0, 0, time.UTC)
	store := &subscriptionStoreStub{}
	service, _ := NewPersistentAgentEventSubscriptionControlV1(store, subscriptionDefinitionReaderStub{definition: subscriptionDefinitionFixture(now)}, readableSubscriptionConversations(), func() time.Time { return now })
	request := application.AgentEventSubscriptionCreateRequestV1{
		TenantID: "dipole", DefinitionUUID: "DEF-1", DefinitionVersion: 1,
		EventType: "message.group.created", ResourceType: "conversation", ResourceID: "group:G1",
		FilterKind: application.AgentSubscriptionFilterAll, FilterJSON: json.RawMessage(`{}`),
	}
	created, _ := service.Create(context.Background(), "U100", request)
	page, err := service.List(context.Background(), "U100", application.AgentEventSubscriptionListRequestV1{TenantID: "dipole", Limit: 1})
	if err != nil || len(page.Subscriptions) != 1 || page.Subscriptions[0].SubscriptionUUID != created.SubscriptionUUID {
		t.Fatalf("list owned subscriptions: page=%+v err=%v", page, err)
	}
	revoked, err := service.Revoke(context.Background(), "U100", application.AgentEventSubscriptionRevokeRequestV1{TenantID: "dipole", SubscriptionUUID: created.SubscriptionUUID, Reason: "project retired"})
	if err != nil || revoked.Status != application.AgentSubscriptionStatusRevoked || revoked.RevokedByUUID != "U100" || revoked.RevokedAt == nil {
		t.Fatalf("revoke subscription: item=%+v err=%v", revoked, err)
	}
	if _, err := service.Revoke(context.Background(), "U100", application.AgentEventSubscriptionRevokeRequestV1{TenantID: "dipole", SubscriptionUUID: created.SubscriptionUUID, Reason: "project retired"}); err != nil {
		t.Fatalf("exact revoke replay: %v", err)
	}
	if _, err := service.Revoke(context.Background(), "U100", application.AgentEventSubscriptionRevokeRequestV1{TenantID: "dipole", SubscriptionUUID: created.SubscriptionUUID, Reason: "different"}); !errors.Is(err, application.ErrAgentSubscriptionConflict) {
		t.Fatalf("expected revoke conflict, got %v", err)
	}
	foreign, err := service.List(context.Background(), "U999", application.AgentEventSubscriptionListRequestV1{TenantID: "dipole", Limit: 10})
	if err != nil || len(foreign.Subscriptions) != 0 {
		t.Fatalf("foreign owner should receive an isolated empty page: page=%+v err=%v", foreign, err)
	}
}

func subscriptionFixture(id string) application.AgentEventSubscriptionV1 {
	return application.AgentEventSubscriptionV1{
		SubscriptionUUID: id, DefinitionUUID: "DEF-1", DefinitionVersion: 1,
		TenantID: "dipole", AgentUUID: "UAI", Status: application.AgentSubscriptionStatusActive,
		EventType: "message.group.created", ResourceType: "conversation", ResourceID: "group:G1",
		FilterKind: application.AgentSubscriptionFilterAll, FilterJSON: json.RawMessage(`{}`),
		CreatedByUUID: "U100",
	}
}

func subscriptionDefinitionFixture(now time.Time) application.AgentDefinitionVersionV1 {
	return application.AgentDefinitionVersionV1{
		DefinitionUUID: "DEF-1", Version: 1, TenantID: "dipole", OwnerUUID: "U100", AgentUUID: "UAI",
		Status: application.AgentDefinitionStatusActive, Permissions: []string{"conversation.read"},
		Scopes:    []application.AgentResourceScopeV1{{ResourceType: "conversation", ResourceID: "group:G1", Actions: []string{"read"}}},
		ValidFrom: now.Add(-time.Hour),
	}
}
