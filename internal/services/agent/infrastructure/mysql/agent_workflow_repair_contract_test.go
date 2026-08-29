package agentmysql_test

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
	mysqlData "github.com/JekYUlll/Dipole/internal/data/mysql"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	sqlcRepository "github.com/JekYUlll/Dipole/internal/services/agent/infrastructure/mysql"
)

func TestAgentWorkflowRepairAuditMySQLConcurrencyContract(t *testing.T) {
	db, _ := openContractDatabase(t)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	mysqlStore, err := mysqlData.NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	transactionalStore, err := sqlcRepository.NewAgentPolicyRepositoryWithTransactions(mysqlStore)
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
		id                        string
		propose, approve, execute bool
	}{{"PROPOSER", true, false, false}, {"APPROVER-1", false, true, false}, {"APPROVER-2", false, true, false}, {"EXECUTOR-1", false, false, true}} {
		if _, err := db.Exec(`INSERT INTO agent_workflow_repair_operator_grants (user_uuid, can_propose, can_approve, can_execute, granted_by_uuid, valid_from) VALUES (?, ?, ?, ?, 'DBA', ?)`, grant.id, grant.propose, grant.approve, grant.execute, now.Add(-time.Hour)); err != nil {
			t.Fatalf("grant %s: %v", grant.id, err)
		}
	}
	executorGrant, err := store.GetWorkflowRepairOperatorGrant(context.Background(), "EXECUTOR-1")
	if err != nil || executorGrant == nil || executorGrant.Version != 1 || !executorGrant.CanExecute {
		t.Fatalf("executor grant=%+v err=%v", executorGrant, err)
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

	execution := application.AgentWorkflowRepairExecutionV1{
		ExecutionUUID: "repair-execution:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		PlanID:        "repair-plan:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ProposalUUID:  proposal.ProposalUUID, TaskUUID: task.TaskUUID, ExecutorUUID: "EXECUTOR-1", ExecutorGrantVersion: 3,
		ExpectedCurrentSHA256: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		TargetSHA256:          "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
		RollbackSHA256:        "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		Status:                application.AgentWorkflowRepairExecutionStatusPrepared,
	}
	created, err := store.CreateWorkflowRepairExecution(context.Background(), execution)
	if err != nil || !created {
		t.Fatalf("create prepared repair execution: created=%v err=%v", created, err)
	}
	replayed, err := store.CreateWorkflowRepairExecution(context.Background(), execution)
	if err != nil || replayed {
		t.Fatalf("replay prepared repair execution: created=%v err=%v", replayed, err)
	}
	loaded, err := store.GetWorkflowRepairExecution(context.Background(), execution.ExecutionUUID)
	if err != nil || loaded == nil || loaded.PlanID != execution.PlanID || loaded.Status != execution.Status {
		t.Fatalf("load prepared repair execution: execution=%+v err=%v", loaded, err)
	}
	conflict := execution
	conflict.TargetSHA256 = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	if _, err := store.CreateWorkflowRepairExecution(context.Background(), conflict); err == nil {
		t.Fatal("expected conflicting plan replay to fail")
	}
	startedAt := time.Now().UTC().Truncate(time.Millisecond)
	claimed, err := store.ClaimWorkflowRepairExecution(context.Background(), execution.ExecutionUUID, execution.ExecutorUUID, execution.ExecutorGrantVersion, startedAt)
	if err != nil || !claimed {
		t.Fatalf("claim prepared repair execution: claimed=%v err=%v", claimed, err)
	}
	claimedAgain, err := store.ClaimWorkflowRepairExecution(context.Background(), execution.ExecutionUUID, execution.ExecutorUUID, execution.ExecutorGrantVersion, startedAt.Add(time.Second))
	if err != nil || claimedAgain {
		t.Fatalf("reclaim executing repair execution: claimed=%v err=%v", claimedAgain, err)
	}
	failed, err := store.FailWorkflowRepairExecution(context.Background(), execution.ExecutionUUID, execution.ExecutorUUID, "projection_cas_mismatch", startedAt.Add(2*time.Second))
	if err != nil || !failed {
		t.Fatalf("fail executing repair execution: failed=%v err=%v", failed, err)
	}
	failedAgain, err := store.FailWorkflowRepairExecution(context.Background(), execution.ExecutionUUID, execution.ExecutorUUID, "projection_cas_mismatch", startedAt.Add(3*time.Second))
	if err != nil || failedAgain {
		t.Fatalf("refail terminal repair execution: failed=%v err=%v", failedAgain, err)
	}
	loaded, err = store.GetWorkflowRepairExecution(context.Background(), execution.ExecutionUUID)
	if err != nil || loaded == nil || loaded.Status != application.AgentWorkflowRepairExecutionStatusFailed {
		t.Fatalf("load failed repair execution: execution=%+v err=%v", loaded, err)
	}
	secondExecutionUUID := "repair-execution:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	secondPlanID := "repair-plan:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if _, err := db.Exec(`INSERT INTO agent_workflow_repair_executions (execution_uuid, plan_id, proposal_uuid, task_uuid, executor_uuid, executor_grant_version, target_sha256) VALUES (?, ?, ?, ?, ?, ?, ?)`, secondExecutionUUID, secondPlanID, execution.ProposalUUID, execution.TaskUUID, execution.ExecutorUUID, execution.ExecutorGrantVersion, execution.TargetSHA256); err != nil {
		t.Fatalf("seed transactional repair execution: %v", err)
	}
	if claimed, err := store.ClaimWorkflowRepairExecution(context.Background(), secondExecutionUUID, execution.ExecutorUUID, execution.ExecutorGrantVersion, startedAt.Add(4*time.Second)); err != nil || !claimed {
		t.Fatalf("claim transactional repair execution: claimed=%v err=%v", claimed, err)
	}
	transactionalTarget := application.AgentTaskWorkflowProjectionV1{TaskUUID: task.TaskUUID, WorkflowID: "dipole-agent-task/" + task.TaskUUID, RunID: "WR-REPAIR", Status: application.AgentTaskWorkflowStatusFailed, Revision: 3}
	if committed, err := transactionalStore.CommitWorkflowRepairProjection(context.Background(), secondExecutionUUID, execution.ExecutorUUID, execution.ExecutorGrantVersion, &projection, transactionalTarget, startedAt.Add(5*time.Second)); err != nil || !committed {
		t.Fatalf("commit transactional repair projection: committed=%v err=%v", committed, err)
	}
	if rolledBack, err := transactionalStore.RollbackWorkflowRepairProjection(context.Background(), secondExecutionUUID, execution.ExecutorUUID, execution.ExecutorGrantVersion, transactionalTarget, &projection, startedAt.Add(6*time.Second)); err != nil || !rolledBack {
		t.Fatalf("rollback transactional repair projection: rolled_back=%v err=%v", rolledBack, err)
	}
	loaded, err = store.GetWorkflowRepairExecution(context.Background(), secondExecutionUUID)
	if err != nil || loaded == nil || loaded.Status != application.AgentWorkflowRepairExecutionStatusRolledBack {
		t.Fatalf("load rolled-back repair execution: execution=%+v err=%v", loaded, err)
	}
}
