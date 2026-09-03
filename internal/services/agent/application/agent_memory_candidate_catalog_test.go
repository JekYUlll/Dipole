package agentapplication

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

type candidateCatalogStoreStub struct {
	request application.AgentMemoryCandidateCatalogRequestV1
	items   []application.AgentMemoryCandidateCatalogItemV1
}

func (s *candidateCatalogStoreStub) ListOwnedCandidates(_ context.Context, tenantID, principalUUID, afterCandidateUUID string, limit int) ([]application.AgentMemoryCandidateCatalogItemV1, error) {
	s.request = application.AgentMemoryCandidateCatalogRequestV1{TenantID: tenantID, PrincipalUUID: principalUUID, AfterCandidateUUID: afterCandidateUUID, Limit: limit}
	return append([]application.AgentMemoryCandidateCatalogItemV1(nil), s.items...), nil
}

func TestAgentMemoryCandidateCatalogPagesOwnedCandidates(t *testing.T) {
	now := time.Date(2026, 9, 3, 1, 0, 0, 0, time.UTC)
	store := &candidateCatalogStoreStub{items: []application.AgentMemoryCandidateCatalogItemV1{
		candidateCatalogItem("CAND-1", now), candidateCatalogItem("CAND-2", now), candidateCatalogItem("CAND-3", now),
	}}
	service, err := NewPersistentAgentMemoryCandidateCatalogV1(store)
	if err != nil {
		t.Fatal(err)
	}
	page, err := service.ListOwnedCandidates(context.Background(), application.AgentMemoryCandidateCatalogRequestV1{TenantID: "dipole", PrincipalUUID: "U100", AfterCandidateUUID: "CAND-0", Limit: 2})
	if err != nil || len(page.Items) != 2 || page.NextCursor != "CAND-2" || store.request.Limit != 3 || store.request.PrincipalUUID != "U100" {
		t.Fatalf("page=%+v request=%+v err=%v", page, store.request, err)
	}
}

func TestAgentMemoryCandidateCatalogRejectsInvalidOwnerRequest(t *testing.T) {
	service, _ := NewPersistentAgentMemoryCandidateCatalogV1(&candidateCatalogStoreStub{})
	_, err := service.ListOwnedCandidates(context.Background(), application.AgentMemoryCandidateCatalogRequestV1{TenantID: "dipole", PrincipalUUID: "", Limit: 1})
	if !errors.Is(err, application.ErrAgentMemoryCandidateInvalid) {
		t.Fatalf("error=%v", err)
	}
}

func candidateCatalogItem(id string, observedAt time.Time) application.AgentMemoryCandidateCatalogItemV1 {
	return application.AgentMemoryCandidateCatalogItemV1{Candidate: application.AgentMemoryCandidateV1{
		CandidateUUID: id, TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", ResourceType: "conversation", ResourceID: "group:G1", CandidateType: application.AgentMemoryCandidateTypeMessage, SourceID: "MSG-1", EvidenceIDs: []string{"MSG-1"}, Summary: "Candidate " + id, PolicyVersion: "memory-v1", CandidateSHA256: strings.Repeat("a", 64), Status: application.AgentMemoryCandidateStatusPending, ObservedAt: observedAt,
	}}
}
