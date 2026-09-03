package agentapplication

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

type agentDefinitionCatalogStoreStub struct {
	items                          []application.AgentDefinitionVersionV1
	tenant, owner, afterDefinition string
	afterVersion                   uint64
	limit                          int
	activeAt                       time.Time
	err                            error
}

func (s *agentDefinitionCatalogStoreStub) CreateDefinitionVersion(_ context.Context, definition application.AgentDefinitionVersionV1) error {
	for _, item := range s.items {
		if item.DefinitionUUID == definition.DefinitionUUID && item.Version == definition.Version {
			return errors.New("duplicate Definition")
		}
	}
	s.items = append(s.items, definition)
	return nil
}

func (s *agentDefinitionCatalogStoreStub) GetDefinitionVersion(_ context.Context, definitionUUID string, version uint64) (*application.AgentDefinitionVersionV1, error) {
	for index := range s.items {
		if s.items[index].DefinitionUUID == definitionUUID && s.items[index].Version == version {
			item := s.items[index]
			return &item, nil
		}
	}
	return nil, nil
}

func (s *agentDefinitionCatalogStoreStub) ListOwnedActiveDefinitions(_ context.Context, tenant, owner, afterDefinition string, afterVersion uint64, activeAt time.Time, limit int) ([]application.AgentDefinitionVersionV1, error) {
	s.tenant, s.owner, s.afterDefinition, s.afterVersion, s.activeAt, s.limit = tenant, owner, afterDefinition, afterVersion, activeAt, limit
	return append([]application.AgentDefinitionVersionV1(nil), s.items...), s.err
}

func TestAgentDefinitionCatalogListsOwnedEligibleVersions(t *testing.T) {
	now := time.Unix(1000, 0).UTC()
	store := &agentDefinitionCatalogStoreStub{items: []application.AgentDefinitionVersionV1{
		definitionCatalogFixture("DEF-1", 2, now),
		definitionCatalogFixture("DEF-2", 1, now),
	}}
	store.items[0].Scopes = append(store.items[0].Scopes,
		application.AgentResourceScopeV1{ResourceType: "conversation", ResourceID: "group:G1", Actions: []string{"read"}},
		application.AgentResourceScopeV1{ResourceType: "file", ResourceID: "*", Actions: []string{"read"}},
	)
	service, err := NewPersistentAgentDefinitionCatalogV1(store, "UAI", func() time.Time { return now })
	if err != nil {
		t.Fatalf("new Definition catalog: %v", err)
	}
	page, err := service.List(context.Background(), "U100", application.AgentDefinitionCatalogListRequestV1{
		TenantID: "dipole", Limit: 1,
	})
	if err != nil {
		t.Fatalf("list Definition catalog: %v", err)
	}
	if store.tenant != "dipole" || store.owner != "U100" || store.limit != 2 || !store.activeAt.Equal(now) {
		t.Fatalf("unexpected Store request: %+v", store)
	}
	if len(page.Definitions) != 1 || page.Definitions[0].DefinitionUUID != "DEF-1" || page.Definitions[0].Version != 2 ||
		len(page.Definitions[0].ConversationScopes) != 2 || page.Definitions[0].ConversationScopes[0] != "*" || page.Definitions[0].ConversationScopes[1] != "group:G1" ||
		page.NextDefinitionUUID != "DEF-1" || page.NextVersion != 2 {
		t.Fatalf("unexpected Definition page: %+v", page)
	}
}

func TestAgentDefinitionCatalogFailsClosedOnAuthorityDrift(t *testing.T) {
	now := time.Unix(1000, 0).UTC()
	tests := []struct {
		name   string
		mutate func(*application.AgentDefinitionVersionV1)
	}{
		{name: "foreign owner", mutate: func(item *application.AgentDefinitionVersionV1) { item.OwnerUUID = "U200" }},
		{name: "expired", mutate: func(item *application.AgentDefinitionVersionV1) { expired := now; item.ExpiresAt = &expired }},
		{name: "missing permission", mutate: func(item *application.AgentDefinitionVersionV1) { item.Permissions = []string{"conversation.list"} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			item := definitionCatalogFixture("DEF-1", 1, now)
			test.mutate(&item)
			service, _ := NewPersistentAgentDefinitionCatalogV1(&agentDefinitionCatalogStoreStub{items: []application.AgentDefinitionVersionV1{item}}, "UAI", func() time.Time { return now })
			if _, err := service.List(context.Background(), "U100", application.AgentDefinitionCatalogListRequestV1{TenantID: "dipole", Limit: 10}); err != application.ErrAgentDefinitionCatalogConflict {
				t.Fatalf("error = %v, want conflict", err)
			}
		})
	}
}

func TestAgentDefinitionCatalogRejectsPartialCursor(t *testing.T) {
	service, _ := NewPersistentAgentDefinitionCatalogV1(&agentDefinitionCatalogStoreStub{}, "UAI", time.Now)
	if _, err := service.List(context.Background(), "U100", application.AgentDefinitionCatalogListRequestV1{TenantID: "dipole", AfterDefinitionUUID: "DEF-1"}); err != application.ErrAgentDefinitionCatalogInvalid {
		t.Fatalf("error = %v, want invalid", err)
	}
}

func TestAgentDefinitionCatalogCreatesOwnerScopedReadDefinitionIdempotently(t *testing.T) {
	now := time.Unix(1000, 0).UTC()
	store := &agentDefinitionCatalogStoreStub{}
	service, err := NewPersistentAgentDefinitionCatalogV1(store, "UAI", func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.Create(context.Background(), "U100", application.AgentDefinitionCatalogCreateRequestV1{TenantID: "dipole"})
	if err != nil {
		t.Fatalf("create owner Definition: %v", err)
	}
	replayed, err := service.Create(context.Background(), "U100", application.AgentDefinitionCatalogCreateRequestV1{TenantID: "dipole"})
	if err != nil || first.DefinitionUUID != replayed.DefinitionUUID || len(store.items) != 1 || first.OwnerUUID != "U100" || first.AgentUUID != "UAI" {
		t.Fatalf("owner Definition replay drifted: first=%+v replay=%+v items=%+v err=%v", first, replayed, store.items, err)
	}
	foreign, err := service.Create(context.Background(), "U200", application.AgentDefinitionCatalogCreateRequestV1{TenantID: "dipole"})
	if err != nil || foreign.DefinitionUUID == first.DefinitionUUID || len(store.items) != 2 {
		t.Fatalf("foreign owner Definition isolation failed: foreign=%+v items=%+v err=%v", foreign, store.items, err)
	}
	if _, err := service.Create(context.Background(), "", application.AgentDefinitionCatalogCreateRequestV1{TenantID: "dipole"}); err != application.ErrAgentDefinitionCatalogInvalid {
		t.Fatalf("missing owner error = %v", err)
	}
}

func TestAgentDefinitionCatalogCreatesExplicitSubscriptionAutoReplyDefinition(t *testing.T) {
	now := time.Unix(1000, 0).UTC()
	store := &agentDefinitionCatalogStoreStub{}
	service, err := NewPersistentAgentDefinitionCatalogV1(store, "UAI", func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	readOnly, err := service.Create(context.Background(), "U100", application.AgentDefinitionCatalogCreateRequestV1{TenantID: "dipole"})
	if err != nil {
		t.Fatalf("create read-only Definition: %v", err)
	}
	autoReply, err := service.Create(context.Background(), "U100", application.AgentDefinitionCatalogCreateRequestV1{TenantID: "dipole", Profile: application.AgentDefinitionCatalogProfileSubscriptionAutoReply})
	if err != nil {
		t.Fatalf("create subscription auto-reply Definition: %v", err)
	}
	replayed, err := service.Create(context.Background(), "U100", application.AgentDefinitionCatalogCreateRequestV1{TenantID: "dipole", Profile: application.AgentDefinitionCatalogProfileSubscriptionAutoReply})
	if err != nil {
		t.Fatalf("replay subscription auto-reply Definition: %v", err)
	}
	if autoReply.DefinitionUUID == readOnly.DefinitionUUID || replayed.DefinitionUUID != autoReply.DefinitionUUID || len(store.items) != 2 {
		t.Fatalf("auto-reply Definition identity drifted: read=%+v auto=%+v replay=%+v", readOnly, autoReply, replayed)
	}
	if len(autoReply.Permissions) != 2 || autoReply.Permissions[0] != application.AgentPermissionConversationRead || autoReply.Permissions[1] != application.AgentPermissionMessageWrite {
		t.Fatalf("auto-reply permissions = %#v", autoReply.Permissions)
	}
	if len(autoReply.Scopes) != 1 || autoReply.Scopes[0].ResourceType != application.AgentResourceTypeConversation || autoReply.Scopes[0].ResourceID != application.AgentResourceWildcard || len(autoReply.Scopes[0].Actions) != 2 || autoReply.Scopes[0].Actions[0] != application.AgentResourceActionRead || autoReply.Scopes[0].Actions[1] != application.AgentResourceActionWrite {
		t.Fatalf("auto-reply scopes = %#v", autoReply.Scopes)
	}
}

func TestAgentDefinitionCatalogRejectsUnknownCreateProfile(t *testing.T) {
	service, _ := NewPersistentAgentDefinitionCatalogV1(&agentDefinitionCatalogStoreStub{}, "UAI", time.Now)
	if _, err := service.Create(context.Background(), "U100", application.AgentDefinitionCatalogCreateRequestV1{TenantID: "dipole", Profile: "write_everywhere"}); err != application.ErrAgentDefinitionCatalogInvalid {
		t.Fatalf("unknown profile error = %v, want invalid", err)
	}
}

func definitionCatalogFixture(id string, version uint64, now time.Time) application.AgentDefinitionVersionV1 {
	return application.AgentDefinitionVersionV1{
		DefinitionUUID: id, Version: version, TenantID: "dipole", OwnerUUID: "U100", AgentUUID: "UAI",
		Status: application.AgentDefinitionStatusActive, Permissions: []string{application.AgentPermissionConversationRead},
		Scopes:    []application.AgentResourceScopeV1{{ResourceType: "conversation", ResourceID: "*", Actions: []string{"read"}}},
		ValidFrom: now.Add(-time.Minute), CreatedAt: now.Add(-time.Minute), UpdatedAt: now,
	}
}
