package agentapplication

import (
	"context"
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
	service, err := NewPersistentAgentDefinitionCatalogV1(store, func() time.Time { return now })
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
			service, _ := NewPersistentAgentDefinitionCatalogV1(&agentDefinitionCatalogStoreStub{items: []application.AgentDefinitionVersionV1{item}}, func() time.Time { return now })
			if _, err := service.List(context.Background(), "U100", application.AgentDefinitionCatalogListRequestV1{TenantID: "dipole", Limit: 10}); err != application.ErrAgentDefinitionCatalogConflict {
				t.Fatalf("error = %v, want conflict", err)
			}
		})
	}
}

func TestAgentDefinitionCatalogRejectsPartialCursor(t *testing.T) {
	service, _ := NewPersistentAgentDefinitionCatalogV1(&agentDefinitionCatalogStoreStub{}, time.Now)
	if _, err := service.List(context.Background(), "U100", application.AgentDefinitionCatalogListRequestV1{TenantID: "dipole", AfterDefinitionUUID: "DEF-1"}); err != application.ErrAgentDefinitionCatalogInvalid {
		t.Fatalf("error = %v, want invalid", err)
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
