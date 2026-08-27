package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

type subscriptionStoreStub struct {
	items []application.AgentEventSubscriptionV1
}

func (*subscriptionStoreStub) CreateEventSubscription(context.Context, application.AgentEventSubscriptionV1) error {
	return nil
}
func (s *subscriptionStoreStub) GetEventSubscription(context.Context, string) (*application.AgentEventSubscriptionV1, error) {
	if len(s.items) == 0 {
		return nil, nil
	}
	item := s.items[0]
	return &item, nil
}
func (s *subscriptionStoreStub) ListMatchingEventSubscriptions(context.Context, application.AgentEventSubscriptionMatchRequestV1) ([]application.AgentEventSubscriptionV1, error) {
	return append([]application.AgentEventSubscriptionV1(nil), s.items...), nil
}
func (*subscriptionStoreStub) RevokeEventSubscription(context.Context, string, time.Time) error {
	return nil
}

type subscriptionDefinitionReaderStub struct {
	definition application.AgentDefinitionVersionV1
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
		TenantID: "dipole", AgentUUID: "UAI", EventType: "message.direct.created",
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
				TenantID: "dipole", AgentUUID: "UAI", EventType: "message.direct.created", ResourceType: "conversation", ResourceID: "group:G1",
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

func subscriptionFixture(id string) application.AgentEventSubscriptionV1 {
	return application.AgentEventSubscriptionV1{
		SubscriptionUUID: id, DefinitionUUID: "DEF-1", DefinitionVersion: 1,
		TenantID: "dipole", AgentUUID: "UAI", Status: application.AgentSubscriptionStatusActive,
		EventType: "message.direct.created", ResourceType: "conversation", ResourceID: "group:G1",
		FilterKind: application.AgentSubscriptionFilterAll, FilterJSON: json.RawMessage(`{}`),
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
