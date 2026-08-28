package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/data/migration"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	sqlcRepository "github.com/JekYUlll/Dipole/internal/data/mysql/repository"
)

func TestAgentMemoryRepositoryContract(t *testing.T) {
	db, _ := openContractDatabase(t)
	runner, _ := migration.NewRunner(db, migrations.Files)
	if err := runner.Up(context.Background()); err != nil {
		t.Fatalf("migrate contract database: %v", err)
	}
	store, err := sqlcRepository.NewAgentMemoryRepository(generated.New(db))
	if err != nil {
		t.Fatalf("new Memory repository: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	memories := []application.AgentMemoryV1{
		{MemoryUUID: "MEM-B", TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", MemoryType: application.AgentMemoryTypeSemantic, Status: application.AgentMemoryStatusActive,
			ResourceType: "conversation", ResourceID: "group:G1", Content: "second", Priority: 10, Provenance: application.AgentMemoryProvenanceV1{SourceType: "message", SourceID: "M2"}, ValidFrom: now.Add(-time.Hour)},
		{MemoryUUID: "MEM-A", TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", MemoryType: application.AgentMemoryTypeEpisodic, Status: application.AgentMemoryStatusActive,
			ResourceType: "conversation", ResourceID: "group:G1", Content: "first", CompactContent: "first compact", Priority: 10, Provenance: application.AgentMemoryProvenanceV1{SourceType: "message", SourceID: "M1", Sequence: "42"}, ValidFrom: now.Add(-time.Hour)},
		{MemoryUUID: "MEM-OTHER", TenantID: "dipole", PrincipalUUID: "U999", AgentUUID: "UAI", MemoryType: application.AgentMemoryTypeSemantic, Status: application.AgentMemoryStatusActive,
			ResourceType: "conversation", ResourceID: "group:G1", Content: "private", Priority: 100, Provenance: application.AgentMemoryProvenanceV1{SourceType: "message", SourceID: "M3"}, ValidFrom: now.Add(-time.Hour)},
	}
	for _, memory := range memories {
		if err := store.CreateMemory(context.Background(), memory); err != nil {
			t.Fatalf("create %s: %v", memory.MemoryUUID, err)
		}
	}
	var createdBefore time.Time
	if err := db.QueryRow(`SELECT MAX(created_at) FROM agent_memories`).Scan(&createdBefore); err != nil {
		t.Fatalf("read Memory creation cutoff: %v", err)
	}
	query := application.AgentMemoryQueryV1{TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", ResourceType: "conversation", ResourceID: "group:G1", CreatedBefore: createdBefore, At: now, Limit: 20}
	items, err := store.ListContextMemories(context.Background(), query)
	if err != nil || len(items) != 2 || items[0].MemoryUUID != "MEM-A" || items[1].MemoryUUID != "MEM-B" || items[0].CompactContent != "first compact" {
		t.Fatalf("scoped Memories=%+v err=%v", items, err)
	}
	owned, err := store.ListOwnedMemories(context.Background(), application.AgentMemoryOwnerListRequestV1{
		TenantID: "dipole", PrincipalUUID: "U100", AfterCreatedAt: createdBefore, Limit: 10,
	})
	if err != nil || len(owned) != 2 || owned[0].PrincipalUUID != "U100" || owned[1].PrincipalUUID != "U100" {
		t.Fatalf("owned Memories=%+v err=%v", owned, err)
	}
	if err := store.RevokeOwnedMemory(context.Background(), "dipole", "U100", "MEM-A", "U100", "outdated", now); err != nil {
		t.Fatalf("revoke Memory: %v", err)
	}
	revoked, err := store.GetOwnedMemory(context.Background(), "dipole", "U100", "MEM-A")
	if err != nil || revoked == nil || revoked.RevokedByUUID != "U100" || revoked.RevokeReason != "outdated" || revoked.Validate() != nil {
		t.Fatalf("audited revoked Memory=%+v err=%v", revoked, err)
	}
	foreign, err := store.GetOwnedMemory(context.Background(), "dipole", "U999", "MEM-A")
	if err != nil || foreign != nil {
		t.Fatalf("foreign owner Memory=%+v err=%v", foreign, err)
	}
	items, _ = store.ListContextMemories(context.Background(), query)
	if len(items) != 1 || items[0].MemoryUUID != "MEM-B" {
		t.Fatalf("revoked Memory remained visible: %+v", items)
	}
	if err := store.RevokeOwnedMemory(context.Background(), "dipole", "U100", "MEM-A", "U100", "outdated", now); !errors.Is(err, application.ErrAgentMemoryConflict) {
		t.Fatalf("second revoke error=%v", err)
	}
}
