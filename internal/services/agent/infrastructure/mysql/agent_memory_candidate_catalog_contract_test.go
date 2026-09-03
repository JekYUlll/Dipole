package agentmysql_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/migration"
	sqlcRepository "github.com/JekYUlll/Dipole/internal/services/agent/infrastructure/mysql"
)

func TestAgentMemoryCandidateCatalogMySQLContractScopesOwnerAndReview(t *testing.T) {
	ctx := context.Background()
	db, _ := openContractDatabase(t)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatalf("create migration runner: %v", err)
	}
	if err := runner.Up(ctx); err != nil {
		t.Fatalf("migrate contract database: %v", err)
	}
	repository, err := sqlcRepository.NewAgentMemoryRepository(generated.New(db))
	if err != nil {
		t.Fatalf("create Memory repository: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	insertCandidateCatalogRow(t, ctx, db, "CAND-1", "U100", "pending", "", now)
	insertCandidateCatalogRow(t, ctx, db, "CAND-2", "U100", "accepted", "REV-2", now)
	insertCandidateCatalogRow(t, ctx, db, "CAND-3", "U200", "accepted", "REV-3", now)

	items, err := repository.ListOwnedCandidates(ctx, "dipole", "U100", "", 3)
	if err != nil || len(items) != 2 {
		t.Fatalf("owner candidates=%+v err=%v", items, err)
	}
	if items[0].Candidate.CandidateUUID != "CAND-1" || items[0].ReviewUUID != "" || items[1].Candidate.CandidateUUID != "CAND-2" || items[1].ReviewUUID != "REV-2" || items[1].Candidate.Summary != "candidate CAND-2" {
		t.Fatalf("unexpected owner candidates=%+v", items)
	}
	items, err = repository.ListOwnedCandidates(ctx, "dipole", "U100", "CAND-1", 3)
	if err != nil || len(items) != 1 || items[0].Candidate.CandidateUUID != "CAND-2" {
		t.Fatalf("cursor candidates=%+v err=%v", items, err)
	}
}

func insertCandidateCatalogRow(t *testing.T, ctx context.Context, db *sql.DB, candidateID, principalID, status, reviewID string, now time.Time) {
	t.Helper()
	hash := strings.Repeat(string(candidateID[5]), 64)
	if _, err := db.ExecContext(ctx, `INSERT INTO agent_memory_candidates
        (candidate_uuid, tenant_id, principal_uuid, agent_uuid, resource_type, resource_id, candidate_type, source_id, evidence_ids_json, summary, policy_version, candidate_sha256, status, observed_at)
        VALUES (?, 'dipole', ?, 'UAI', 'conversation', 'group:G1', 'message', ?, ?, ?, 'memory-v1', ?, ?, ?)`,
		candidateID, principalID, "MSG-"+candidateID, `["MSG-`+candidateID+`"]`, "candidate "+candidateID, hash, status, now); err != nil {
		t.Fatalf("insert candidate %s: %v", candidateID, err)
	}
	if reviewID == "" {
		return
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO agent_memory_candidate_reviews
        (review_uuid, candidate_uuid, candidate_sha256, reviewer_uuid, decision, reason, review_sha256, reviewed_at)
        VALUES (?, ?, ?, ?, 'accepted', 'owner reviewed', ?, ?)`,
		reviewID, candidateID, hash, principalID, strings.Repeat("f", 64), now); err != nil {
		t.Fatalf("insert candidate review %s: %v", candidateID, err)
	}
}
