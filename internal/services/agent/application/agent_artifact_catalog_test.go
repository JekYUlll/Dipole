package agentapplication

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

type artifactCatalogStoreStub struct {
	tenantID, principalID, afterID string
	afterCreatedAt                 time.Time
	limit                          int
	items                          []application.AgentArtifactV1
}

func (s *artifactCatalogStoreStub) ListOwnedAgentArtifacts(_ context.Context, tenantID, principalID string, afterCreatedAt time.Time, afterID string, limit int) ([]application.AgentArtifactV1, error) {
	s.tenantID, s.principalID, s.afterCreatedAt, s.afterID, s.limit = tenantID, principalID, afterCreatedAt, afterID, limit
	return append([]application.AgentArtifactV1(nil), s.items...), nil
}

func TestAgentArtifactCatalogPagesOwnerArtifactsByCompositeCursor(t *testing.T) {
	now := time.Date(2026, 9, 3, 1, 0, 0, 0, time.UTC)
	store := &artifactCatalogStoreStub{items: []application.AgentArtifactV1{
		artifactCatalogItem(strings.Repeat("a", 64), now), artifactCatalogItem(strings.Repeat("b", 64), now.Add(-time.Second)), artifactCatalogItem(strings.Repeat("c", 64), now.Add(-2*time.Second)),
	}}
	service, err := NewPersistentAgentArtifactCatalogV1(store)
	if err != nil {
		t.Fatal(err)
	}
	page, err := service.ListForPrincipal(context.Background(), application.AgentArtifactCatalogRequestV1{TenantID: "dipole", PrincipalUUID: "U100", Limit: 2})
	if err != nil || len(page.Artifacts) != 2 || page.NextArtifactID != strings.Repeat("b", 64) || !page.NextCreatedAt.Equal(now.Add(-time.Second)) || store.limit != 3 || store.principalID != "U100" || store.afterID != "" || store.afterCreatedAt.Year() != 9999 {
		t.Fatalf("page=%+v request=%+v err=%v", page, store, err)
	}
}

func TestAgentArtifactCatalogRejectsPartialCursor(t *testing.T) {
	service, _ := NewPersistentAgentArtifactCatalogV1(&artifactCatalogStoreStub{})
	_, err := service.ListForPrincipal(context.Background(), application.AgentArtifactCatalogRequestV1{TenantID: "dipole", PrincipalUUID: "U100", AfterCreatedAt: time.Now(), Limit: 1})
	if !errors.Is(err, application.ErrAgentArtifactInvalid) {
		t.Fatalf("error=%v", err)
	}
}

func artifactCatalogItem(id string, createdAt time.Time) application.AgentArtifactV1 {
	return application.AgentArtifactV1{ArtifactUUID: id, TaskUUID: "TASK-1", RunUUID: "RUN-1", ArtifactType: "conversation_digest", Version: 1, Title: "Daily digest", MediaType: "text/markdown", ContentSHA256: strings.Repeat("d", 64), SizeBytes: 12, CreatedAt: createdAt}
}
