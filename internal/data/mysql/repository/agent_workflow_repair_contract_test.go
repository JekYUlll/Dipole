package repository_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	appComposition "github.com/JekYUlll/Dipole/internal/app"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/data/migration"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	sqlcRepository "github.com/JekYUlll/Dipole/internal/data/mysql/repository"
)

func TestAgentWorkflowRepairAuditMySQLConcurrencyContract(t *testing.T) {
	db, _ := openContractDatabase(t)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	store, err := sqlcRepository.NewAgentPolicyRepository(generated.New(db))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	definition := application.AgentDefinitionVersionV1{DefinitionUUID: "DEF-REPAIR", Version: 1, TenantID: "dipole", OwnerUUID: "U100", AgentUUID: "UAI", Status: application.AgentDefinitionStatusActive, Permissions: []string{application.AgentPermissionConversationRead}, Scopes: []application.AgentResourceScopeV1{{ResourceType: "conversation", ResourceID: "G1", Actions: []string{"read"}}}, ValidFrom: now.Add(-time.Hour)}
	if err := store.CreateDefinitionVersion(context.Background(), definition); err != nil {
		t.Fatal(err)
	}
	task := application.AgentTaskV1{TaskUUID: "TASK-REPAIR", DefinitionUUID: definition.DefinitionUUID, DefinitionVersion: 1, TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", Status: application.AgentTaskStatusRunning, TriggerType: "manual", TriggerRef: "INC-1", Goal: "audit repair"}
	if _, err := store.CreateTask(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	projection := application.AgentTaskWorkflowProjectionV1{TaskUUID: task.TaskUUID, WorkflowID: "dipole-agent-task/" + task.TaskUUID, RunID: "WR-1", Status: application.AgentTaskWorkflowStatusRunning, Revision: 2}
	if _, err := store.ProjectTaskWorkflowState(context.Background(), projection); err != nil {
		t.Fatal(err)
	}
	for _, grant := range []struct {
		id               string
		propose, approve bool
	}{{"PROPOSER", true, false}, {"APPROVER-1", false, true}, {"APPROVER-2", false, true}} {
		if _, err := db.Exec(`INSERT INTO agent_workflow_repair_operator_grants (user_uuid, can_propose, can_approve, granted_by_uuid, valid_from) VALUES (?, ?, ?, 'DBA', ?)`, grant.id, grant.propose, grant.approve, now.Add(-time.Hour)); err != nil {
			t.Fatalf("grant %s: %v", grant.id, err)
		}
	}
	service, err := appComposition.NewPersistentAgentWorkflowRepairAuditServiceV1(store, store)
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := service.Propose(context.Background(), "PROPOSER", application.AgentWorkflowRepairProposalRequestV1{TaskUUID: task.TaskUUID, Outcome: application.AgentWorkflowRepairOutcomeStale, TicketRef: "INC-1", Reason: "Temporal confirms completion", Projected: &application.AgentWorkflowEvidenceV1{WorkflowID: projection.WorkflowID, WorkflowRunID: projection.RunID, Status: string(projection.Status), Revision: projection.Revision}, Temporal: application.AgentWorkflowEvidenceV1{WorkflowID: projection.WorkflowID, WorkflowRunID: projection.RunID, Status: "completed", Revision: 3}, ProposedAt: now, ExpiresAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	var successes atomic.Int64
	errorsCh := make(chan error, 16)
	var workers sync.WaitGroup
	for i := 0; i < 16; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			result, decideErr := service.Decide(context.Background(), "APPROVER-1", proposal.ProposalUUID, application.AgentWorkflowRepairDecisionApproved)
			if decideErr != nil {
				errorsCh <- decideErr
				return
			}
			if result.Status == application.AgentWorkflowRepairStatusProposed {
				successes.Add(1)
			}
		}()
	}
	workers.Wait()
	close(errorsCh)
	for workerErr := range errorsCh {
		t.Errorf("concurrent decision: %v", workerErr)
	}
	counts, err := store.CountWorkflowRepairDecisions(context.Background(), proposal.ProposalUUID)
	if err != nil || counts.Approved != 1 || counts.Rejected != 0 {
		t.Fatalf("first approver counts=%+v err=%v", counts, err)
	}
	if successes.Load() != 16 {
		t.Fatalf("idempotent results=%d", successes.Load())
	}
	approved, err := service.Decide(context.Background(), "APPROVER-2", proposal.ProposalUUID, application.AgentWorkflowRepairDecisionApproved)
	if err != nil || approved.Status != application.AgentWorkflowRepairStatusApproved {
		t.Fatalf("second approval=%+v err=%v", approved, err)
	}
	if _, err := service.Decide(context.Background(), "APPROVER-2", proposal.ProposalUUID, application.AgentWorkflowRepairDecisionRejected); err == nil {
		t.Fatal("expected immutable decision conflict after approval")
	}
}
