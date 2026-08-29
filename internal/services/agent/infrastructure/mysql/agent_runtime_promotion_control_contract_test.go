package agentmysql_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/application"
	mysqlData "github.com/JekYUlll/Dipole/internal/platform/mysql"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/migration"
	agentapplication "github.com/JekYUlll/Dipole/internal/services/agent/application"
	sqlcRepository "github.com/JekYUlll/Dipole/internal/services/agent/infrastructure/mysql"
)

func TestAgentRuntimePromotionControlMySQLContract(t *testing.T) {
	db, _ := openContractDatabase(t)
	runner, _ := migration.NewRunner(db, migrations.Files)
	if err := runner.Up(context.Background()); err != nil {
		t.Fatal(err)
	}
	queries := generated.New(db)
	policies, _ := sqlcRepository.NewAgentPolicyRepository(queries)
	artifacts, _ := sqlcRepository.NewAgentArtifactRepository(queries)
	txStore, _ := mysqlData.NewStore(db)
	control, _ := sqlcRepository.NewAgentRuntimePromotionControlRepository(txStore)
	now := time.Now().UTC().Truncate(time.Millisecond)
	definition := application.AgentDefinitionVersionV1{DefinitionUUID: "DEF-CONTROL", Version: 1, TenantID: "dipole", OwnerUUID: "U1", AgentUUID: "UAI", Status: application.AgentDefinitionStatusActive, Permissions: []string{"conversation.read"}, Scopes: []application.AgentResourceScopeV1{{ResourceType: "conversation", ResourceID: "*", Actions: []string{"read"}}}, ValidFrom: now.Add(-time.Hour)}
	if err := policies.CreateDefinitionVersion(context.Background(), definition); err != nil {
		t.Fatal(err)
	}
	task := application.AgentTaskV1{TaskUUID: "TASK-CONTROL", DefinitionUUID: definition.DefinitionUUID, DefinitionVersion: 1, TenantID: "dipole", PrincipalUUID: "U1", AgentUUID: "UAI", Status: application.AgentTaskStatusRunning, TriggerType: "manual", TriggerRef: "control", Goal: "evaluate"}
	if _, err := policies.CreateTask(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	run := application.AgentRunV1{RunUUID: "RUN-CONTROL", TaskUUID: task.TaskUUID, RuntimeID: "dipole-agent", Mode: "shadow", Status: application.AgentRunStatusRunning}
	if _, err := policies.CreateRun(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	if ok, err := policies.TransitionRunStatus(context.Background(), run.RunUUID, application.AgentRunStatusRunning, application.AgentRunStatusCompleted, ""); err != nil || !ok {
		t.Fatalf("complete Run: ok=%v err=%v", ok, err)
	}
	artifact := application.AgentArtifactV1{ArtifactUUID: strings.Repeat("1", 64), SchemaVersion: application.AgentArtifactSchemaVersionV1, TaskUUID: task.TaskUUID, RunUUID: run.RunUUID, ArtifactType: "promotion_evaluation", Version: 1, Title: "Evaluation", MediaType: "application/json", ObjectBucket: "agent", ObjectKey: "promotion/control", ContentSHA256: strings.Repeat("a", 64), SizeBytes: 2, Metadata: json.RawMessage(`{"runtimeId":"dipole-agent","candidateVersion":"candidate-v1","definitionId":"DEF-CONTROL","definitionVersion":1,"evalSuiteSHA256":"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"}`)}
	if created, err := artifacts.CreateAgentArtifact(context.Background(), artifact); err != nil || !created {
		t.Fatalf("create Artifact: created=%v err=%v", created, err)
	}
	for _, operator := range []struct {
		id                      string
		propose, review, revoke bool
	}{{"PROPOSER", true, false, false}, {"REVIEWER", false, true, false}, {"REVOKER", false, false, true}} {
		if _, err := db.Exec(`INSERT INTO agent_runtime_promotion_operator_grants (tenant_id,user_uuid,can_propose,can_review,can_revoke,granted_by_uuid,valid_from) VALUES (?,?,?,?,?,?,?)`, "dipole", operator.id, operator.propose, operator.review, operator.revoke, "ROOT", now.Add(-time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	service, err := agentapplication.NewPersistentAgentRuntimePromotionControlServiceV1(policies, artifacts, control)
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := service.Propose(context.Background(), "PROPOSER", application.AgentRuntimePromotionProposalRequestV1{TenantID: "dipole", RuntimeID: run.RuntimeID, CandidateVersion: "candidate-v1", DefinitionUUID: definition.DefinitionUUID, DefinitionVersion: 1, EvidenceArtifactUUID: artifact.ArtifactUUID, EvidenceSHA256: artifact.ContentSHA256, EvalSuiteSHA256: strings.Repeat("e", 64), TicketRef: "REL-1", Reason: "all promotion gates passed", ProposedAt: now, ExpiresAt: now.Add(time.Hour), GrantValidFrom: now, GrantExpiresAt: now.Add(24 * time.Hour)})
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	if _, err := service.Review(context.Background(), "PROPOSER", proposal.ProposalUUID, application.AgentRuntimePromotionReviewApproved); !errors.Is(err, application.ErrAgentRuntimePromotionControlDenied) {
		t.Fatalf("self review: %v", err)
	}
	approved, err := service.Review(context.Background(), "REVIEWER", proposal.ProposalUUID, application.AgentRuntimePromotionReviewApproved)
	if err != nil || approved.GrantUUID == "" {
		t.Fatalf("approve: proposal=%+v err=%v", approved, err)
	}
	if replay, err := service.Review(context.Background(), "REVIEWER", proposal.ProposalUUID, application.AgentRuntimePromotionReviewApproved); err != nil || replay.GrantUUID != approved.GrantUUID {
		t.Fatalf("review replay: proposal=%+v err=%v", replay, err)
	}
	if _, err := service.Revoke(context.Background(), "REVIEWER", approved.GrantUUID, "INC-1", "rollback"); !errors.Is(err, application.ErrAgentRuntimePromotionControlDenied) {
		t.Fatalf("unauthorized revoke: %v", err)
	}
	revoked, err := service.Revoke(context.Background(), "REVOKER", approved.GrantUUID, "INC-1", "rollback")
	if err != nil || revoked.RevokedAt == nil {
		t.Fatalf("revoke: grant=%+v err=%v", revoked, err)
	}
	if replay, err := service.Revoke(context.Background(), "REVOKER", approved.GrantUUID, "INC-1", "rollback"); err != nil || replay.RevokedAt == nil {
		t.Fatalf("revoke replay: grant=%+v err=%v", replay, err)
	}
}
