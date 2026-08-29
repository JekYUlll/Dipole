package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

type agentMemoryOwnerStoreStub struct {
	items              []application.AgentMemoryV1
	listRequest        application.AgentMemoryOwnerListRequestV1
	revokeRequest      application.AgentMemoryOwnerRevokeRequestV1
	revokedAt          time.Time
	conflictAfterWrite bool
	correctionWrite    application.AgentMemoryOwnerCorrectionWriteV1
	correctionResult   *application.AgentMemoryOwnerCorrectionResultV1
}

func (s *agentMemoryOwnerStoreStub) EraseOwnedMemoryRoot(_ context.Context, tenantID, principalUUID, memoryUUID, erasedByUUID string, reason application.AgentMemoryErasureReasonV1, erasedAt time.Time) (*application.AgentMemoryOwnerErasureReceiptV1, error) {
	item, _ := s.GetOwnedMemory(context.Background(), tenantID, principalUUID, memoryUUID)
	if item == nil {
		return nil, application.ErrAgentMemoryDenied
	}
	versions := uint32(0)
	for index := range s.items {
		memory := &s.items[index]
		if memory.MemoryRootUUID != item.MemoryRootUUID {
			continue
		}
		versions++
		memory.Status, memory.Content, memory.CompactContent, memory.Provenance.URI = application.AgentMemoryStatusRevoked, application.AgentMemoryErasedContentV1, "", ""
		memory.ResourceType, memory.ResourceID = application.AgentMemorySourceErasedV1, application.AgentMemoryErasedReferenceV1
		if memory.MemoryVersion == 1 {
			memory.Provenance = application.AgentMemoryProvenanceV1{SourceType: application.AgentMemorySourceErasedV1, SourceID: application.AgentMemoryErasedReferenceV1}
		}
		memory.RevokedAt, memory.RevokedByUUID, memory.RevokeReason = &erasedAt, erasedByUUID, application.AgentMemoryPrivacyErasureAuditV1
		memory.ContentErasedAt, memory.ContentErasedByUUID, memory.ContentErasureReason = &erasedAt, erasedByUUID, reason
		if memory.MemoryVersion > 1 {
			memory.CorrectionReason = application.AgentMemoryPrivacyErasureAuditV1
		}
	}
	return &application.AgentMemoryOwnerErasureReceiptV1{MemoryRootUUID: item.MemoryRootUUID, Versions: versions, ErasedAt: erasedAt, ErasedByUUID: erasedByUUID, Reason: reason}, nil
}

func TestPersistentAgentMemoryOwnerControlErasesWholeRootWithoutPublicReason(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 28, 13, 0, 0, 0, time.UTC)
	root := agentMemoryOwnerFixture("MEM-1", "U100", now.Add(-time.Hour))
	root.MemoryRootUUID, root.MemoryVersion = "MEM-1", 1
	successor := root
	successor.MemoryUUID, successor.MemoryVersion, successor.SupersedesMemoryUUID = "MEM-2", 2, "MEM-1"
	successor.Status = application.AgentMemoryStatusActive
	successor.Provenance = application.AgentMemoryProvenanceV1{SourceType: application.AgentMemorySourceOwnerCorrectionV1, SourceID: "MEM-1", Sequence: "2"}
	successor.CorrectedByUUID, successor.CorrectionReason = "U100", "private correction reason"
	revokedAt := now.Add(-time.Minute)
	root.Status, root.RevokedAt, root.RevokedByUUID, root.RevokeReason = application.AgentMemoryStatusRevoked, &revokedAt, "U100", "superseded by MEM-2"
	store := &agentMemoryOwnerStoreStub{items: []application.AgentMemoryV1{root, successor}}
	service, _ := NewPersistentAgentMemoryOwnerControlV1(store, func() time.Time { return now })
	receipt, err := service.EraseOwnedMemory(context.Background(), application.AgentMemoryOwnerErasureRequestV1{TenantID: "dipole", PrincipalUUID: "U100", MemoryUUID: "MEM-2"})
	if err != nil || receipt.Versions != 2 || receipt.MemoryRootUUID != "MEM-1" || receipt.Reason != application.AgentMemoryErasureReasonOwnerRequest {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	for _, item := range store.items {
		if item.Content != application.AgentMemoryErasedContentV1 || item.CompactContent != "" || item.CorrectionReason == "private correction reason" || item.Validate() != nil {
			t.Fatalf("non-erased item: %+v", item)
		}
	}
}

func (s *agentMemoryOwnerStoreStub) ListOwnedMemories(_ context.Context, request application.AgentMemoryOwnerListRequestV1) ([]application.AgentMemoryV1, error) {
	s.listRequest = request
	return append([]application.AgentMemoryV1(nil), s.items...), nil
}

func (s *agentMemoryOwnerStoreStub) GetOwnedMemory(_ context.Context, tenantID, principalUUID, memoryUUID string) (*application.AgentMemoryV1, error) {
	for index := range s.items {
		item := &s.items[index]
		if item.TenantID == tenantID && item.PrincipalUUID == principalUUID && item.MemoryUUID == memoryUUID {
			copy := *item
			return &copy, nil
		}
	}
	return nil, nil
}

func (s *agentMemoryOwnerStoreStub) RevokeOwnedMemory(_ context.Context, tenantID, principalUUID, memoryUUID, revokedByUUID, reason string, revokedAt time.Time) error {
	s.revokeRequest = application.AgentMemoryOwnerRevokeRequestV1{TenantID: tenantID, PrincipalUUID: principalUUID, MemoryUUID: memoryUUID, Reason: reason}
	s.revokedAt = revokedAt
	for index := range s.items {
		item := &s.items[index]
		if item.TenantID == tenantID && item.PrincipalUUID == principalUUID && item.MemoryUUID == memoryUUID && item.Status == application.AgentMemoryStatusActive {
			item.Status, item.RevokedAt = application.AgentMemoryStatusRevoked, &revokedAt
			item.RevokedByUUID, item.RevokeReason = revokedByUUID, reason
			if s.conflictAfterWrite {
				s.conflictAfterWrite = false
				return application.ErrAgentMemoryConflict
			}
			return nil
		}
	}
	return application.ErrAgentMemoryConflict
}

func (s *agentMemoryOwnerStoreStub) CorrectOwnedMemory(_ context.Context, write application.AgentMemoryOwnerCorrectionWriteV1) (*application.AgentMemoryOwnerCorrectionResultV1, error) {
	s.correctionWrite = write
	if s.correctionResult != nil {
		result := *s.correctionResult
		return &result, nil
	}
	for index := range s.items {
		previous := &s.items[index]
		if previous.MemoryUUID != write.SourceMemoryUUID || previous.Status != application.AgentMemoryStatusActive || previous.MemoryVersion != write.ExpectedVersion {
			continue
		}
		revokedAt := write.CorrectedAt
		previous.Status, previous.RevokedAt = application.AgentMemoryStatusRevoked, &revokedAt
		previous.RevokedByUUID, previous.RevokeReason = write.PrincipalUUID, "superseded by "+write.Corrected.MemoryUUID
		s.items = append(s.items, write.Corrected)
		return &application.AgentMemoryOwnerCorrectionResultV1{Previous: *previous, Corrected: write.Corrected}, nil
	}
	return nil, application.ErrAgentMemoryConflict
}

func TestPersistentAgentMemoryOwnerControlListsStableOwnerPage(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	store := &agentMemoryOwnerStoreStub{items: []application.AgentMemoryV1{
		agentMemoryOwnerFixture("MEM-3", "U100", now.Add(-time.Minute)),
		agentMemoryOwnerFixture("MEM-2", "U100", now.Add(-2*time.Minute)),
		agentMemoryOwnerFixture("MEM-1", "U100", now.Add(-3*time.Minute)),
	}}
	service, err := NewPersistentAgentMemoryOwnerControlV1(store, func() time.Time { return now })
	if err != nil {
		t.Fatalf("new owner control: %v", err)
	}
	page, err := service.ListOwnedMemories(context.Background(), application.AgentMemoryOwnerListRequestV1{TenantID: "dipole", PrincipalUUID: "U100", Limit: 2})
	if err != nil || len(page.Memories) != 2 || page.NextMemoryUUID != "MEM-2" || !page.NextCreatedAt.Equal(now.Add(-2*time.Minute)) {
		t.Fatalf("owner page: page=%+v err=%v", page, err)
	}
	if store.listRequest.Limit != 3 || !store.listRequest.AfterCreatedAt.Equal(now) {
		t.Fatalf("store request = %+v", store.listRequest)
	}
}

func TestPersistentAgentMemoryOwnerControlRevokesWithExactAuditReplay(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	store := &agentMemoryOwnerStoreStub{items: []application.AgentMemoryV1{agentMemoryOwnerFixture("MEM-1", "U100", now.Add(-time.Hour))}}
	service, _ := NewPersistentAgentMemoryOwnerControlV1(store, func() time.Time { return now })
	request := application.AgentMemoryOwnerRevokeRequestV1{TenantID: "dipole", PrincipalUUID: "U100", MemoryUUID: "MEM-1", Reason: "information is outdated"}

	revoked, err := service.RevokeOwnedMemory(context.Background(), request)
	if err != nil || revoked.Status != application.AgentMemoryStatusRevoked || revoked.RevokedByUUID != "U100" || revoked.RevokeReason != request.Reason || revoked.RevokedAt == nil {
		t.Fatalf("revoke: memory=%+v err=%v", revoked, err)
	}
	if _, err = service.RevokeOwnedMemory(context.Background(), request); err != nil {
		t.Fatalf("exact replay: %v", err)
	}
	request.Reason = "different reason"
	if _, err = service.RevokeOwnedMemory(context.Background(), request); !errors.Is(err, application.ErrAgentMemoryConflict) {
		t.Fatalf("different replay error = %v", err)
	}
	foreign := request
	foreign.PrincipalUUID, foreign.Reason = "U999", "information is outdated"
	if _, err = service.RevokeOwnedMemory(context.Background(), foreign); !errors.Is(err, application.ErrAgentMemoryDenied) {
		t.Fatalf("foreign revoke error = %v", err)
	}
}

func TestPersistentAgentMemoryOwnerControlAcceptsConcurrentExactRevocation(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	store := &agentMemoryOwnerStoreStub{
		items:              []application.AgentMemoryV1{agentMemoryOwnerFixture("MEM-1", "U100", now.Add(-time.Hour))},
		conflictAfterWrite: true,
	}
	service, _ := NewPersistentAgentMemoryOwnerControlV1(store, func() time.Time { return now })
	request := application.AgentMemoryOwnerRevokeRequestV1{
		TenantID: "dipole", PrincipalUUID: "U100", MemoryUUID: "MEM-1", Reason: "information is outdated",
	}

	revoked, err := service.RevokeOwnedMemory(context.Background(), request)
	if err != nil || revoked.Status != application.AgentMemoryStatusRevoked || revoked.RevokeReason != request.Reason {
		t.Fatalf("concurrent exact revoke: memory=%+v err=%v", revoked, err)
	}
}

func TestPersistentAgentMemoryOwnerControlCorrectsAppendOnlyWithStableReplay(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	source := agentMemoryOwnerFixture("MEM-1", "U100", now.Add(-time.Hour))
	source.MemoryRootUUID, source.MemoryVersion = source.MemoryUUID, 1
	store := &agentMemoryOwnerStoreStub{items: []application.AgentMemoryV1{source}}
	service, _ := NewPersistentAgentMemoryOwnerControlV1(store, func() time.Time { return now })
	request := application.AgentMemoryOwnerCorrectionRequestV1{
		TenantID: "dipole", PrincipalUUID: "U100", MemoryUUID: "MEM-1", ExpectedVersion: 1,
		Content: "Owner is Bob", CompactContent: "Owner: Bob", Reason: "owner changed in MSG-2",
	}

	result, err := service.CorrectOwnedMemory(context.Background(), request)
	if err != nil {
		t.Fatalf("correct Memory: %v", err)
	}
	if result.Previous.Status != application.AgentMemoryStatusRevoked || result.Corrected.MemoryVersion != 2 ||
		result.Corrected.MemoryRootUUID != "MEM-1" || result.Corrected.SupersedesMemoryUUID != "MEM-1" ||
		result.Corrected.CorrectedByUUID != "U100" || result.Corrected.CorrectionReason != request.Reason ||
		result.Corrected.Provenance.SourceType != application.AgentMemorySourceOwnerCorrectionV1 || result.Corrected.Provenance.SourceID != "MEM-1" ||
		result.Corrected.Provenance.Sequence != "2" {
		t.Fatalf("correction result = %+v", result)
	}
	firstID := result.Corrected.MemoryUUID

	replayStore := &agentMemoryOwnerStoreStub{items: []application.AgentMemoryV1{result.Previous}, correctionResult: result}
	replayService, _ := NewPersistentAgentMemoryOwnerControlV1(replayStore, func() time.Time { return now.Add(time.Minute) })
	replayed, err := replayService.CorrectOwnedMemory(context.Background(), request)
	if err != nil || replayed.Corrected.MemoryUUID != firstID {
		t.Fatalf("exact replay: result=%+v err=%v", replayed, err)
	}
	request.Content = "Owner is Carol"
	if _, err = replayService.CorrectOwnedMemory(context.Background(), request); !errors.Is(err, application.ErrAgentMemoryConflict) {
		t.Fatalf("drifted replay error = %v", err)
	}
}

func TestPersistentAgentMemoryOwnerControlRejectsStaleExpiredAndForeignCorrection(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	source := agentMemoryOwnerFixture("MEM-1", "U100", now.Add(-time.Hour))
	source.MemoryRootUUID, source.MemoryVersion = source.MemoryUUID, 1
	expiredAt := now.Add(-time.Minute)
	source.ExpiresAt = &expiredAt
	service, _ := NewPersistentAgentMemoryOwnerControlV1(&agentMemoryOwnerStoreStub{items: []application.AgentMemoryV1{source}}, func() time.Time { return now })
	request := application.AgentMemoryOwnerCorrectionRequestV1{
		TenantID: "dipole", PrincipalUUID: "U100", MemoryUUID: "MEM-1", ExpectedVersion: 1,
		Content: "Owner is Bob", Reason: "owner changed",
	}
	if _, err := service.CorrectOwnedMemory(context.Background(), request); !errors.Is(err, application.ErrAgentMemoryConflict) {
		t.Fatalf("expired correction error = %v", err)
	}
	request.PrincipalUUID = "U999"
	if _, err := service.CorrectOwnedMemory(context.Background(), request); !errors.Is(err, application.ErrAgentMemoryDenied) {
		t.Fatalf("foreign correction error = %v", err)
	}
	request.PrincipalUUID, request.ExpectedVersion = "U100", 2
	if _, err := service.CorrectOwnedMemory(context.Background(), request); !errors.Is(err, application.ErrAgentMemoryConflict) {
		t.Fatalf("stale correction error = %v", err)
	}
}

func agentMemoryOwnerFixture(memoryUUID, principalUUID string, createdAt time.Time) application.AgentMemoryV1 {
	return application.AgentMemoryV1{
		MemoryUUID: memoryUUID, TenantID: "dipole", PrincipalUUID: principalUUID, AgentUUID: "UAI",
		MemoryType: application.AgentMemoryTypeSemantic, Status: application.AgentMemoryStatusActive,
		ResourceType: "conversation", ResourceID: "group:G1", Content: "Owner is Alice", Priority: 80,
		Provenance: application.AgentMemoryProvenanceV1{SourceType: "message", SourceID: "MSG-1"},
		ValidFrom:  createdAt.Add(-time.Hour), CreatedAt: createdAt,
	}
}
