package repository_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/data/migration"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	sqlcRepository "github.com/JekYUlll/Dipole/internal/data/mysql/repository"
)

func TestAgentMCPReadinessEvidenceMySQLAppendOnlyContract(t *testing.T) {
	db, _ := openContractDatabase(t)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatal(err)
	}
	repository, err := sqlcRepository.NewAgentMCPReadinessEvidenceRepository(generated.New(db))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	record := newReadinessEvidenceRecord(t, "dipole", strings.Repeat("b", 64), strings.Repeat("a", 64), now)
	if created, err := repository.AppendAgentMCPReadinessEvidence(context.Background(), record); err != nil || !created {
		t.Fatalf("first append: created=%t err=%v", created, err)
	}
	if created, err := repository.AppendAgentMCPReadinessEvidence(context.Background(), record); err != nil || created {
		t.Fatalf("exact replay: created=%t err=%v", created, err)
	}
	loaded, err := repository.GetAgentMCPReadinessEvidence(context.Background(), "dipole", record.EvidenceUUID)
	if err != nil || loaded == nil || loaded.ContentSHA256 != record.ContentSHA256 || string(loaded.ContentJSON) != string(record.ContentJSON) {
		t.Fatalf("load exact evidence: loaded=%+v err=%v", loaded, err)
	}
	if crossTenant, err := repository.GetAgentMCPReadinessEvidence(context.Background(), "other", record.EvidenceUUID); err != nil || crossTenant != nil {
		t.Fatalf("cross-tenant read: loaded=%+v err=%v", crossTenant, err)
	}
	lookup := application.AgentMCPReadinessEvidenceLookupV1{
		TenantID: "dipole", ProfileBindingSHA256: record.ProfileBindingSHA256,
		RuntimeBindingSHA256: record.RuntimeBindingSHA256, At: now.Add(10 * time.Minute),
	}
	lookup.At = record.CollectedAt.Add(-time.Millisecond)
	if future, err := repository.GetFreshAgentMCPReadinessEvidence(context.Background(), lookup); err != nil || future != nil {
		t.Fatalf("future evidence: evidence=%+v err=%v", future, err)
	}
	lookup.At = now.Add(10 * time.Minute)
	if fresh, err := repository.GetFreshAgentMCPReadinessEvidence(context.Background(), lookup); err != nil || fresh == nil || fresh.EvidenceUUID != record.EvidenceUUID {
		t.Fatalf("fresh exact binding: evidence=%+v err=%v", fresh, err)
	}
	lookup.At = record.ExpiresAt
	if stale, err := repository.GetFreshAgentMCPReadinessEvidence(context.Background(), lookup); err != nil || stale != nil {
		t.Fatalf("expired evidence: evidence=%+v err=%v", stale, err)
	}
	lookup.At = now.Add(10 * time.Minute)
	lookup.ProfileBindingSHA256 = strings.Repeat("c", 64)
	if drifted, err := repository.GetFreshAgentMCPReadinessEvidence(context.Background(), lookup); err != nil || drifted != nil {
		t.Fatalf("Profile drift: evidence=%+v err=%v", drifted, err)
	}

	drift := newReadinessEvidenceRecord(t, "dipole", strings.Repeat("b", 64), strings.Repeat("d", 64), now)
	if created, err := repository.AppendAgentMCPReadinessEvidence(context.Background(), drift); err != nil || !created || drift.EvidenceUUID == record.EvidenceUUID {
		t.Fatalf("Runtime binding drift: created=%t evidence=%s err=%v", created, drift.EvidenceUUID, err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM agent_mcp_readiness_evidence`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("immutable history count=%d err=%v", count, err)
	}
	currentVersion, err := runner.CurrentVersion(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Down(context.Background(), int(currentVersion-36)); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'agent_mcp_readiness_evidence'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("table after rollback: count=%d err=%v", count, err)
	}
}

func newReadinessEvidenceRecord(t *testing.T, tenantID, profileBinding, runtimeBinding string, startedAt time.Time) application.AgentMCPReadinessEvidenceRecordV1 {
	t.Helper()
	evidence := application.AgentMCPReadinessEvidenceV1{
		SchemaVersion: application.AgentMCPReadinessEvidenceSchemaVersionV2,
		BindingSHA256: runtimeBinding, ProfileBindingSHA256: profileBinding,
		StartedAt: startedAt, PreflightCheckedAt: startedAt.Add(time.Second),
		ConnectivityCheckedAt: startedAt.Add(2 * time.Second), CompletedAt: startedAt.Add(3 * time.Second),
		ProfileCount: 1, CredentialCount: 1, CABundleCount: 1, ToolCount: 1,
	}
	record, err := application.NewAgentMCPReadinessEvidenceRecordV1("OPERATOR", application.AgentMCPReadinessEvidenceRequestV1{
		TenantID: tenantID, ProfileBindingSHA256: profileBinding, RequestID: "REQ-1", TraceID: "TRACE-1",
		ExpiresAt: startedAt.Add(30 * time.Minute), Evidence: evidence,
	})
	if err != nil {
		t.Fatal(err)
	}
	return record
}
