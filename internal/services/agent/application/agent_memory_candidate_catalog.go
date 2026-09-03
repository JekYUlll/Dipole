package agentapplication

import (
	"context"
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
)

type PersistentAgentMemoryCandidateCatalogV1 struct {
	store application.AgentMemoryCandidateCatalogStoreV1
}

func NewPersistentAgentMemoryCandidateCatalogV1(store application.AgentMemoryCandidateCatalogStoreV1) (*PersistentAgentMemoryCandidateCatalogV1, error) {
	if store == nil {
		return nil, application.ErrAgentMemoryCandidateInvalid
	}
	return &PersistentAgentMemoryCandidateCatalogV1{store: store}, nil
}

func (s *PersistentAgentMemoryCandidateCatalogV1) ListOwnedCandidates(ctx context.Context, request application.AgentMemoryCandidateCatalogRequestV1) (*application.AgentMemoryCandidateCatalogPageV1, error) {
	request.TenantID = strings.TrimSpace(request.TenantID)
	request.PrincipalUUID = strings.TrimSpace(request.PrincipalUUID)
	request.AfterCandidateUUID = strings.TrimSpace(request.AfterCandidateUUID)
	if request.TenantID == "" || request.PrincipalUUID == "" || len(request.AfterCandidateUUID) > 72 || request.Limit < 0 || request.Limit > 100 {
		return nil, application.ErrAgentMemoryCandidateInvalid
	}
	if request.Limit == 0 {
		request.Limit = 50
	}
	items, err := s.store.ListOwnedCandidates(ctx, request.TenantID, request.PrincipalUUID, request.AfterCandidateUUID, request.Limit+1)
	if err != nil {
		return nil, err
	}
	page := &application.AgentMemoryCandidateCatalogPageV1{Items: items}
	if len(items) > request.Limit {
		page.Items = items[:request.Limit]
		page.NextCursor = page.Items[len(page.Items)-1].Candidate.CandidateUUID
	}
	return page, nil
}
