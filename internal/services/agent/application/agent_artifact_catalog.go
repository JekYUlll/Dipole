package agentapplication

import (
	"context"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

type PersistentAgentArtifactCatalogV1 struct {
	store application.AgentArtifactCatalogStoreV1
}

func NewPersistentAgentArtifactCatalogV1(store application.AgentArtifactCatalogStoreV1) (*PersistentAgentArtifactCatalogV1, error) {
	if store == nil {
		return nil, application.ErrAgentArtifactInvalid
	}
	return &PersistentAgentArtifactCatalogV1{store: store}, nil
}

func (s *PersistentAgentArtifactCatalogV1) ListForPrincipal(ctx context.Context, request application.AgentArtifactCatalogRequestV1) (*application.AgentArtifactCatalogPageV1, error) {
	request.TenantID, request.PrincipalUUID, request.AfterArtifactID = strings.TrimSpace(request.TenantID), strings.TrimSpace(request.PrincipalUUID), strings.TrimSpace(request.AfterArtifactID)
	if request.TenantID == "" || request.PrincipalUUID == "" || (request.AfterCreatedAt.IsZero() != (request.AfterArtifactID == "")) || (request.AfterArtifactID != "" && !agentArtifactCatalogIDV1(request.AfterArtifactID)) || request.Limit < 0 || request.Limit > 100 {
		return nil, application.ErrAgentArtifactInvalid
	}
	if request.Limit == 0 {
		request.Limit = 50
	}
	if request.AfterCreatedAt.IsZero() {
		request.AfterCreatedAt = time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC)
	}
	items, err := s.store.ListOwnedAgentArtifacts(ctx, request.TenantID, request.PrincipalUUID, request.AfterCreatedAt, request.AfterArtifactID, request.Limit+1)
	if err != nil {
		return nil, err
	}
	page := &application.AgentArtifactCatalogPageV1{Artifacts: items}
	if len(items) > request.Limit {
		page.Artifacts = items[:request.Limit]
		tail := page.Artifacts[len(page.Artifacts)-1]
		page.NextCreatedAt, page.NextArtifactID = tail.CreatedAt, tail.ArtifactUUID
	}
	return page, nil
}

func agentArtifactCatalogIDV1(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !(character >= '0' && character <= '9') && !(character >= 'a' && character <= 'f') && !(character >= 'A' && character <= 'F') {
			return false
		}
	}
	return true
}
