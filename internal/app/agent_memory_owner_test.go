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

func agentMemoryOwnerFixture(memoryUUID, principalUUID string, createdAt time.Time) application.AgentMemoryV1 {
	return application.AgentMemoryV1{
		MemoryUUID: memoryUUID, TenantID: "dipole", PrincipalUUID: principalUUID, AgentUUID: "UAI",
		MemoryType: application.AgentMemoryTypeSemantic, Status: application.AgentMemoryStatusActive,
		ResourceType: "conversation", ResourceID: "group:G1", Content: "Owner is Alice", Priority: 80,
		Provenance: application.AgentMemoryProvenanceV1{SourceType: "message", SourceID: "MSG-1"},
		ValidFrom:  createdAt.Add(-time.Hour), CreatedAt: createdAt,
	}
}
