package agentmysql_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/application"
	mysqlData "github.com/JekYUlll/Dipole/internal/platform/mysql"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/migration"
	agentapplication "github.com/JekYUlll/Dipole/internal/services/agent/application"
	sqlcRepository "github.com/JekYUlll/Dipole/internal/services/agent/infrastructure/mysql"
	agentgrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/agent"
)

type temporalReceiptFixtureState struct {
	Target, Secret, CAFile, CertFile, KeyFile, ServerName          string
	TenantID, PrincipalUserID, AgentID, TaskID, RunID              string
	CandidateID, CandidateSHA256, ReviewID, PolicyVersion          string
	RejectedTaskID, RejectedRunID                                  string
	RejectedCandidateID, RejectedCandidateSHA256, RejectedReviewID string
	RevokePath, RevokedPath, RollbackPath, RolledBackPath          string
}

func TestAgentMemoryPromotionTemporalMySQLMTLSFixtureProcess(t *testing.T) {
	if os.Getenv("DIPOLE_AGENT_TEMPORAL_MYSQL_MTLS_FIXTURE") != "true" {
		t.Skip("external Temporal/MySQL mTLS fixture is disabled")
	}
	readyPath := requiredTemporalFixtureEnv(t, "DIPOLE_AGENT_TEMPORAL_MYSQL_MTLS_READY")
	stopPath := requiredTemporalFixtureEnv(t, "DIPOLE_AGENT_TEMPORAL_MYSQL_MTLS_STOP")
	fixtureDir := filepath.Dir(readyPath)
	revokePath, revokedPath := filepath.Join(fixtureDir, "revoke-grant"), filepath.Join(fixtureDir, "grant-revoked")
	rollbackPath, rolledBackPath := filepath.Join(fixtureDir, "rollback-memory"), filepath.Join(fixtureDir, "memory-rolled-back")
	ctx := context.Background()
	db, _ := openContractDatabase(t)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatalf("create migration runner: %v", err)
	}
	if err := runner.Up(ctx); err != nil {
		t.Fatalf("migrate fixture database: %v", err)
	}
	store, err := mysqlData.NewStore(db)
	if err != nil {
		t.Fatalf("create transaction store: %v", err)
	}
	policy, err := sqlcRepository.NewAgentPolicyRepositoryWithTransactions(store)
	if err != nil {
		t.Fatalf("create policy repository: %v", err)
	}
	memories, err := sqlcRepository.NewAgentMemoryRepositoryWithTransactions(store)
	if err != nil {
		t.Fatalf("create Memory repository: %v", err)
	}
	owners, err := agentapplication.NewPersistentAgentMemoryOwnerControlV1(memories, time.Now)
	if err != nil {
		t.Fatalf("create Memory owner control: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	definition, grant := createReceiptContractPolicy(t, ctx, policy, now)
	authorizer, err := agentapplication.NewPersistentAgentActiveRunPromotionAuthorizerV1(policy)
	if err != nil {
		t.Fatalf("create promotion authorizer: %v", err)
	}
	admission, err := agentapplication.NewPersistentAgentRunAdmissionV1WithClock(policy, time.Now, authorizer)
	if err != nil {
		t.Fatalf("create Run admission: %v", err)
	}
	admitted, err := admission.Admit(ctx, application.AgentRunAdmissionRequestV1{
		AgentExecutionPolicyStartV1: application.AgentExecutionPolicyStartV1{
			TenantID: definition.TenantID, PrincipalUUID: definition.OwnerUUID, AgentUUID: definition.AgentUUID,
			DelegatedByUUID: definition.OwnerUUID, TriggerType: "manual", TriggerRef: "temporal-mysql-mtls-fixture",
		},
		RuntimeID: grant.RuntimeID, Mode: "active", CandidateVersion: grant.CandidateVersion,
	})
	if err != nil {
		t.Fatalf("admit fixture Run: %v", err)
	}
	rejected, err := admission.Admit(ctx, application.AgentRunAdmissionRequestV1{
		AgentExecutionPolicyStartV1: application.AgentExecutionPolicyStartV1{
			TenantID: definition.TenantID, PrincipalUUID: definition.OwnerUUID, AgentUUID: definition.AgentUUID,
			DelegatedByUUID: definition.OwnerUUID, TriggerType: "manual", TriggerRef: "temporal-mysql-mtls-revoked-fixture",
		},
		RuntimeID: grant.RuntimeID, Mode: "active", CandidateVersion: grant.CandidateVersion,
	})
	if err != nil {
		t.Fatalf("admit revocation fixture Run: %v", err)
	}
	candidateID, reviewID := "CAND-TEMPORAL-MYSQL", "REVIEW-TEMPORAL-MYSQL"
	candidateSHA256 := strings.Repeat("e", 64)
	insertAcceptedMemoryCandidate(t, ctx, db, candidateID, reviewID, candidateSHA256, now)
	rejectedCandidateID, rejectedReviewID := "CAND-TEMPORAL-REVOKED", "REVIEW-TEMPORAL-REVOKED"
	rejectedCandidateSHA256 := strings.Repeat("f", 64)
	insertAcceptedMemoryCandidate(t, ctx, db, rejectedCandidateID, rejectedReviewID, rejectedCandidateSHA256, now)
	resolver, err := agentapplication.NewPersistentAgentInvocationResolverV1(policy, authorizer)
	if err != nil {
		t.Fatalf("create Invocation resolver: %v", err)
	}
	promotions, err := agentapplication.NewPersistentAgentMemoryCandidatePromotionServiceV1(memories, time.Now)
	if err != nil {
		t.Fatalf("create Memory promotion service: %v", err)
	}
	commits, err := agentapplication.NewPersistentAgentMemoryPromotionReceiptCommitServiceV1(resolver, promotions, time.Now)
	if err != nil {
		t.Fatalf("create receipt commit service: %v", err)
	}
	adapter, err := agentgrpc.NewMemoryPromotionReceiptServer(commits)
	if err != nil {
		t.Fatalf("create Core receipt adapter: %v", err)
	}
	certs := generateReceiptContractCertificates(t)
	server := startReceiptContractRPCServer(t, certs, adapter)
	state := temporalReceiptFixtureState{
		Target: server.Address(), Secret: "receipt-mysql-contract-secret", CAFile: certs.ca, CertFile: certs.agentCert, KeyFile: certs.agentKey, ServerName: "core",
		TenantID: definition.TenantID, PrincipalUserID: definition.OwnerUUID, AgentID: definition.AgentUUID, TaskID: admitted.TaskUUID, RunID: admitted.RunUUID,
		CandidateID: candidateID, CandidateSHA256: candidateSHA256, ReviewID: reviewID, PolicyVersion: "memory-v1",
		RejectedTaskID: rejected.TaskUUID, RejectedRunID: rejected.RunUUID,
		RejectedCandidateID: rejectedCandidateID, RejectedCandidateSHA256: rejectedCandidateSHA256, RejectedReviewID: rejectedReviewID,
		RevokePath: revokePath, RevokedPath: revokedPath, RollbackPath: rollbackPath, RolledBackPath: rolledBackPath,
	}
	writeTemporalFixtureState(t, readyPath, state)
	deadline := time.Now().Add(3 * time.Minute)
	grantRevoked := false
	memoryRolledBack := false
	for {
		if _, err := os.Stat(stopPath); err == nil {
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		if !grantRevoked {
			if _, err := os.Stat(revokePath); err == nil {
				revoked, revokeErr := policy.RevokeRuntimePromotionGrant(ctx, grant.GrantUUID, time.Now().UTC())
				if revokeErr != nil || !revoked {
					t.Fatalf("revoke fixture promotion grant: revoked=%v err=%v", revoked, revokeErr)
				}
				if err := os.WriteFile(revokedPath, []byte("revoked\n"), 0o600); err != nil {
					t.Fatalf("write fixture grant revocation acknowledgement: %v", err)
				}
				grantRevoked = true
			} else if !errors.Is(err, os.ErrNotExist) {
				t.Fatal(err)
			}
		}
		if !memoryRolledBack {
			if rollbackID, err := os.ReadFile(rollbackPath); err == nil {
				var promoted sql.NullString
				if err := db.QueryRowContext(ctx, `SELECT promoted_memory_uuid FROM agent_memory_candidates WHERE candidate_uuid = ?`, candidateID).Scan(&promoted); err != nil || !promoted.Valid || strings.TrimSpace(string(rollbackID)) != promoted.String {
					t.Fatalf("validate fixture Memory rollback ID=%q promoted=%q valid=%v err=%v", strings.TrimSpace(string(rollbackID)), promoted.String, promoted.Valid, err)
				}
				rolledBack, rollbackErr := owners.RevokeOwnedMemory(ctx, application.AgentMemoryOwnerRevokeRequestV1{
					TenantID: definition.TenantID, PrincipalUUID: definition.OwnerUUID, MemoryUUID: promoted.String, Reason: "isolated drill rollback",
				})
				if rollbackErr != nil || rolledBack == nil || rolledBack.Status != application.AgentMemoryStatusRevoked || rolledBack.RevokedAt == nil {
					t.Fatalf("rollback fixture promoted Memory=%+v err=%v", rolledBack, rollbackErr)
				}
				if err := os.WriteFile(rolledBackPath, []byte("rolled_back\n"), 0o600); err != nil {
					t.Fatalf("write fixture Memory rollback acknowledgement: %v", err)
				}
				memoryRolledBack = true
			} else if !errors.Is(err, os.ErrNotExist) {
				t.Fatal(err)
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("Temporal/MySQL mTLS fixture timed out")
		}
		time.Sleep(50 * time.Millisecond)
	}
	var memoryID sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT promoted_memory_uuid FROM agent_memory_candidates WHERE candidate_uuid = ?`, candidateID).Scan(&memoryID); err != nil || !memoryID.Valid {
		t.Fatalf("fixture promoted Memory ID=%q valid=%v err=%v", memoryID.String, memoryID.Valid, err)
	}
	var memoryStatus string
	var revokedAt sql.NullTime
	if err := db.QueryRowContext(ctx, `SELECT status, revoked_at FROM agent_memories WHERE memory_uuid = ?`, memoryID.String).Scan(&memoryStatus, &revokedAt); err != nil || memoryStatus != string(application.AgentMemoryStatusRevoked) || !revokedAt.Valid {
		t.Fatalf("fixture rolled back Memory ID=%q status=%q revoked=%v err=%v", memoryID.String, memoryStatus, revokedAt.Valid, err)
	}
	var rejectedMemory sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT promoted_memory_uuid FROM agent_memory_candidates WHERE candidate_uuid = ?`, rejectedCandidateID).Scan(&rejectedMemory); err != nil || rejectedMemory.Valid {
		t.Fatalf("fixture revoked grant candidate Memory=%q valid=%v err=%v, want none", rejectedMemory.String, rejectedMemory.Valid, err)
	}
}

func requiredTemporalFixtureEnv(t *testing.T, name string) string {
	t.Helper()
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		t.Fatalf("%s is required", name)
	}
	return value
}

func writeTemporalFixtureState(t *testing.T, path string, state temporalReceiptFixtureState) {
	t.Helper()
	encoded, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".tmp", append(encoded, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path+".tmp", path); err != nil {
		t.Fatal(err)
	}
}
