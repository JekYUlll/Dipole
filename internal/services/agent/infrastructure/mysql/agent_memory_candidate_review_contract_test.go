package agentmysql_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/application"
	mysqlData "github.com/JekYUlll/Dipole/internal/platform/mysql"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/migration"
	sqlcRepository "github.com/JekYUlll/Dipole/internal/services/agent/infrastructure/mysql"
)

func TestAgentMemoryCandidateReviewMySQLContractAcceptsOnce(t *testing.T) {
	ctx := context.Background()
	db, _ := openContractDatabase(t)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatalf("create migration runner: %v", err)
	}
	if err := runner.Up(ctx); err != nil {
		t.Fatalf("migrate contract database: %v", err)
	}
	txStore, err := mysqlData.NewStore(db)
	if err != nil {
		t.Fatalf("new Memory transaction store: %v", err)
	}
	repository, err := sqlcRepository.NewAgentMemoryRepositoryWithTransactions(txStore)
	if err != nil {
		t.Fatalf("create Memory repository: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	insertCandidateCatalogRow(t, ctx, db, "CAND-R", "U100", "pending", "", now)
	candidate, err := repository.GetCandidateForPromotion(ctx, "dipole", "U100", "CAND-R")
	if err != nil || candidate == nil {
		t.Fatalf("load pending candidate=%+v err=%v", candidate, err)
	}
	review, err := application.BuildAgentMemoryCandidateReviewV1(candidate.CandidateUUID, candidate.CandidateSHA256, "U100", "accepted", "owner confirmed", now)
	if err != nil {
		t.Fatal(err)
	}
	first, err := repository.ReviewCandidate(ctx, *candidate, review)
	if err != nil || first == nil || first.ReviewUUID != review.ReviewUUID || first.Candidate.Status != "accepted" {
		t.Fatalf("first review=%+v err=%v", first, err)
	}
	second, err := repository.ReviewCandidate(ctx, *candidate, review)
	if err != nil || second.ReviewUUID != first.ReviewUUID {
		t.Fatalf("replay review=%+v err=%v", second, err)
	}
	conflict, err := application.BuildAgentMemoryCandidateReviewV1(candidate.CandidateUUID, candidate.CandidateSHA256, "U100", "rejected", "owner declined", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repository.ReviewCandidate(ctx, *candidate, conflict); !errors.Is(err, application.ErrAgentMemoryCandidateConflict) {
		t.Fatalf("conflicting review err=%v", err)
	}
}
