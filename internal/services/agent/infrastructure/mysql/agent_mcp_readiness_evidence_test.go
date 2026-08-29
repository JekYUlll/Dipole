package agentmysql

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
)

func TestAgentMCPReadinessEvidenceRepositoryFailsClosedOnStorageError(t *testing.T) {
	want := errors.New("storage unavailable")
	repository, err := NewAgentMCPReadinessEvidenceRepository(failingAgentMCPReadinessQueries{err: want})
	if err != nil {
		t.Fatal(err)
	}
	record := readinessRecordForRepositoryTest(t)
	if _, err := repository.AppendAgentMCPReadinessEvidence(context.Background(), record); !errors.Is(err, want) {
		t.Fatalf("append error=%v", err)
	}
	lookup := application.AgentMCPReadinessEvidenceLookupV1{TenantID: record.TenantID, ProfileBindingSHA256: record.ProfileBindingSHA256, RuntimeBindingSHA256: record.RuntimeBindingSHA256, At: record.CollectedAt}
	if _, err := repository.GetFreshAgentMCPReadinessEvidence(context.Background(), lookup); !errors.Is(err, want) {
		t.Fatalf("lookup error=%v", err)
	}
}

type failingAgentMCPReadinessQueries struct{ err error }

func (queries failingAgentMCPReadinessQueries) InsertAgentMCPReadinessEvidence(context.Context, generated.InsertAgentMCPReadinessEvidenceParams) (int64, error) {
	return 0, queries.err
}
func (queries failingAgentMCPReadinessQueries) GetAgentMCPReadinessEvidence(context.Context, generated.GetAgentMCPReadinessEvidenceParams) (generated.AgentMcpReadinessEvidence, error) {
	return generated.AgentMcpReadinessEvidence{}, queries.err
}
func (queries failingAgentMCPReadinessQueries) GetFreshAgentMCPReadinessEvidence(context.Context, generated.GetFreshAgentMCPReadinessEvidenceParams) (generated.AgentMcpReadinessEvidence, error) {
	return generated.AgentMcpReadinessEvidence{}, queries.err
}

func readinessRecordForRepositoryTest(t *testing.T) application.AgentMCPReadinessEvidenceRecordV1 {
	t.Helper()
	evidence := []byte(`{"schemaVersion":"dipole.agent.external-mcp-readiness-evidence.v2","bindingSha256":"` + strings.Repeat("a", 64) + `","profileBindingSha256":"` + strings.Repeat("b", 64) + `","startedAt":"2026-08-28T14:00:00.000Z","completedAt":"2026-08-28T14:00:03.000Z","preflightCheckedAt":"2026-08-28T14:00:01.000Z","connectivityCheckedAt":"2026-08-28T14:00:02.000Z","profileCount":1,"credentialCount":1,"caBundleCount":1,"toolCount":1}`)
	parsed, err := application.ParseAgentMCPReadinessEvidenceV1(evidence)
	if err != nil {
		t.Fatal(err)
	}
	record, err := application.NewAgentMCPReadinessEvidenceRecordV1("OPERATOR", application.AgentMCPReadinessEvidenceRequestV1{
		TenantID: "dipole", ProfileBindingSHA256: parsed.ProfileBindingSHA256,
		RequestID: "REQ-1", TraceID: "TRACE-1", ExpiresAt: parsed.CompletedAt.Add(30 * time.Minute), Evidence: parsed,
	})
	if err != nil {
		t.Fatal(err)
	}
	return record
}

var _ AgentMCPReadinessEvidenceQueries = failingAgentMCPReadinessQueries{}
