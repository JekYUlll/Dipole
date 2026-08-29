package agentmysql_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/data/migration"
	mysqlData "github.com/JekYUlll/Dipole/internal/data/mysql"
	sqlcRepository "github.com/JekYUlll/Dipole/internal/services/agent/infrastructure/mysql"
)

func TestAgentMemoryRepositoryContract(t *testing.T) {
	db, _ := openContractDatabase(t)
	runner, _ := migration.NewRunner(db, migrations.Files)
	if err := runner.Up(context.Background()); err != nil {
		t.Fatalf("migrate contract database: %v", err)
	}
	txStore, err := mysqlData.NewStore(db)
	if err != nil {
		t.Fatalf("new Memory transaction store: %v", err)
	}
	store, err := sqlcRepository.NewAgentMemoryRepositoryWithTransactions(txStore)
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
	source, err := store.GetOwnedMemory(context.Background(), "dipole", "U100", "MEM-B")
	if err != nil || source == nil || source.MemoryRootUUID != "MEM-B" || source.MemoryVersion != 1 {
		t.Fatalf("canonical source Memory=%+v err=%v", source, err)
	}
	corrected := application.AgentMemoryV1{
		MemoryUUID: "MEM-CORR-B", TenantID: source.TenantID, PrincipalUUID: source.PrincipalUUID, AgentUUID: source.AgentUUID,
		MemoryType: source.MemoryType, Status: application.AgentMemoryStatusActive, ResourceType: source.ResourceType, ResourceID: source.ResourceID,
		Content: "second corrected", CompactContent: "corrected", Priority: source.Priority,
		Provenance: application.AgentMemoryProvenanceV1{SourceType: application.AgentMemorySourceOwnerCorrectionV1, SourceID: source.MemoryUUID, Sequence: "2"},
		ValidFrom:  now, MemoryRootUUID: source.MemoryRootUUID, MemoryVersion: 2, SupersedesMemoryUUID: source.MemoryUUID,
		CorrectedByUUID: "U100", CorrectionReason: "owner corrected",
	}
	write := application.AgentMemoryOwnerCorrectionWriteV1{
		TenantID: "dipole", PrincipalUUID: "U100", SourceMemoryUUID: source.MemoryUUID, ExpectedVersion: 1, Corrected: corrected, CorrectedAt: now,
	}
	results := make(chan *application.AgentMemoryOwnerCorrectionResultV1, 2)
	errorsByWorker := make(chan error, 2)
	for range 2 {
		go func() {
			result, correctionErr := store.CorrectOwnedMemory(context.Background(), write)
			results <- result
			errorsByWorker <- correctionErr
		}()
	}
	for range 2 {
		if correctionErr := <-errorsByWorker; correctionErr != nil {
			t.Fatalf("concurrent exact correction: %v", correctionErr)
		}
		result := <-results
		if result == nil || result.Corrected.MemoryUUID != corrected.MemoryUUID || result.Previous.Status != application.AgentMemoryStatusRevoked {
			t.Fatalf("concurrent correction result=%+v", result)
		}
	}
	var rootRows, activeRows int
	if err = db.QueryRow(`SELECT COUNT(*), SUM(status = 'active') FROM agent_memories WHERE tenant_id = 'dipole' AND memory_root_uuid = 'MEM-B'`).Scan(&rootRows, &activeRows); err != nil || rootRows != 2 || activeRows != 1 {
		t.Fatalf("correction lineage rows=%d active=%d err=%v", rootRows, activeRows, err)
	}
	drifted := write
	drifted.Corrected.Content = "different correction"
	if _, err = store.CorrectOwnedMemory(context.Background(), drifted); !errors.Is(err, application.ErrAgentMemoryConflict) {
		t.Fatalf("drifted correction error=%v", err)
	}
	var createdBefore time.Time
	if err := db.QueryRow(`SELECT MAX(created_at) FROM agent_memories`).Scan(&createdBefore); err != nil {
		t.Fatalf("read Memory creation cutoff: %v", err)
	}
	query := application.AgentMemoryQueryV1{TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", ResourceType: "conversation", ResourceID: "group:G1", CreatedBefore: createdBefore, At: now, Limit: 20}
	items, err := store.ListContextMemories(context.Background(), query)
	if err != nil || len(items) != 2 || items[0].MemoryUUID != "MEM-A" || items[1].MemoryUUID != "MEM-CORR-B" || items[0].CompactContent != "first compact" {
		t.Fatalf("scoped Memories=%+v err=%v", items, err)
	}
	owned, err := store.ListOwnedMemories(context.Background(), application.AgentMemoryOwnerListRequestV1{
		TenantID: "dipole", PrincipalUUID: "U100", AfterCreatedAt: createdBefore, Limit: 10,
	})
	if err != nil || len(owned) != 3 || owned[0].PrincipalUUID != "U100" || owned[1].PrincipalUUID != "U100" || owned[2].PrincipalUUID != "U100" {
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
	if len(items) != 1 || items[0].MemoryUUID != "MEM-CORR-B" {
		t.Fatalf("revoked Memory remained visible: %+v", items)
	}
	if err := store.RevokeOwnedMemory(context.Background(), "dipole", "U100", "MEM-A", "U100", "outdated", now); !errors.Is(err, application.ErrAgentMemoryConflict) {
		t.Fatalf("second revoke error=%v", err)
	}
	erasureReceipts := make(chan *application.AgentMemoryOwnerErasureReceiptV1, 2)
	erasureErrors := make(chan error, 2)
	for index, memoryID := range []string{"MEM-B", "MEM-CORR-B"} {
		go func() {
			receipt, eraseErr := store.EraseOwnedMemoryRoot(context.Background(), "dipole", "U100", memoryID, "U100", application.AgentMemoryErasureReasonOwnerRequest, now.Add(time.Duration(index+1)*time.Minute))
			erasureReceipts <- receipt
			erasureErrors <- eraseErr
		}()
	}
	var receipt *application.AgentMemoryOwnerErasureReceiptV1
	for range 2 {
		if err = <-erasureErrors; err != nil {
			t.Fatalf("concurrent erase corrected root: %v", err)
		}
		current := <-erasureReceipts
		if current == nil || current.MemoryRootUUID != "MEM-B" || current.Versions != 2 || current.ErasedByUUID != "U100" {
			t.Fatalf("erasure receipt=%+v", current)
		}
		if receipt == nil {
			receipt = current
		} else if !receipt.ErasedAt.Equal(current.ErasedAt) {
			t.Fatalf("erasure receipts diverged: first=%+v current=%+v", receipt, current)
		}
	}
	var erasedRows, leakedRows int
	if err = db.QueryRow(`SELECT COUNT(*), SUM(content <> '[erased]' OR compact_content IS NOT NULL OR source_uri IS NOT NULL OR resource_type <> 'erased' OR resource_id <> '[erased]' OR revoke_reason <> 'privacy erasure' OR (memory_version = 1 AND (source_type <> 'erased' OR source_id <> '[erased]' OR source_sequence IS NOT NULL)) OR (memory_version > 1 AND correction_reason <> 'privacy erasure')) FROM agent_memories WHERE memory_root_uuid = 'MEM-B' AND content_erased_at IS NOT NULL AND content_erasure_reason_code = 'owner_request'`).Scan(&erasedRows, &leakedRows); err != nil || erasedRows != 2 || leakedRows != 0 {
		t.Fatalf("erased rows=%d leaked=%d err=%v", erasedRows, leakedRows, err)
	}
	items, err = store.ListContextMemories(context.Background(), query)
	if err != nil || len(items) != 0 {
		t.Fatalf("erased root remained in Context: items=%+v err=%v", items, err)
	}
	replayed, err := store.EraseOwnedMemoryRoot(context.Background(), "dipole", "U100", "MEM-B", "U100", application.AgentMemoryErasureReasonOwnerRequest, now.Add(2*time.Minute))
	if err != nil || replayed == nil || !replayed.ErasedAt.Equal(receipt.ErasedAt) {
		t.Fatalf("exact erasure replay: receipt=%+v err=%v", replayed, err)
	}
	if _, err = store.EraseOwnedMemoryRoot(context.Background(), "dipole", "U999", "MEM-B", "U999", application.AgentMemoryErasureReasonOwnerRequest, now); !errors.Is(err, application.ErrAgentMemoryDenied) {
		t.Fatalf("foreign erasure error=%v", err)
	}
}
