package repository_test

import (
	"context"
	"errors"
	"strings"
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

func TestAgentPolicyRepositoryContract(t *testing.T) {
	db, _ := openContractDatabase(t)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatalf("create migration runner: %v", err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatalf("migrate contract database: %v", err)
	}
	store, err := sqlcRepository.NewAgentPolicyRepository(generated.New(db))
	if err != nil {
		t.Fatalf("create Agent Policy repository: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Millisecond)
	definition := application.AgentDefinitionVersionV1{
		DefinitionUUID: "DEF-1", Version: 1, TenantID: "dipole", OwnerUUID: "U100", AgentUUID: "UAI",
		Status:      application.AgentDefinitionStatusActive,
		Permissions: []string{application.AgentPermissionConversationRead, application.AgentPermissionMessageWrite},
		Scopes:      []application.AgentResourceScopeV1{{ResourceType: "conversation", ResourceID: "direct:U100:UAI", Actions: []string{"read", "write"}}},
		ValidFrom:   now,
	}
	if err := store.CreateDefinitionVersion(context.Background(), definition); err != nil {
		t.Fatalf("create definition v1: %v", err)
	}
	definition.Version = 2
	definition.Permissions = []string{application.AgentPermissionConversationRead}
	if err := store.CreateDefinitionVersion(context.Background(), definition); err != nil {
		t.Fatalf("create definition v2: %v", err)
	}
	pinned, err := store.GetDefinitionVersion(context.Background(), "DEF-1", 1)
	if err != nil || pinned == nil || pinned.Version != 1 || len(pinned.Permissions) != 2 {
		t.Fatalf("pinned definition v1: %+v err=%v", pinned, err)
	}
	latest, err := store.GetLatestDefinition(context.Background(), "dipole", "UAI")
	if err != nil || latest == nil || latest.Version != 2 || len(latest.Permissions) != 1 || len(latest.Scopes) != 1 {
		t.Fatalf("latest definition: %+v err=%v", latest, err)
	}
	if err := store.RevokeDefinitionVersion(context.Background(), "DEF-1", 2, now.Add(time.Minute)); err != nil {
		t.Fatalf("revoke definition: %v", err)
	}
	latest, err = store.GetLatestDefinition(context.Background(), "dipole", "UAI")
	if err != nil || latest.Status != application.AgentDefinitionStatusRevoked || latest.RevokedAt == nil {
		t.Fatalf("revoked definition: %+v err=%v", latest, err)
	}

	task := application.AgentTaskV1{
		TaskUUID: "TASK-1", DefinitionUUID: "DEF-1", DefinitionVersion: 2, TenantID: "dipole",
		PrincipalUUID: "U100", AgentUUID: "UAI", Status: application.AgentTaskStatusCreated,
		TriggerType: "message.direct.created", TriggerRef: "M100", Goal: "reply",
	}
	if created, err := store.CreateTask(context.Background(), task); err != nil || !created {
		t.Fatalf("create task: created=%v err=%v", created, err)
	}
	if created, err := store.CreateTask(context.Background(), task); err != nil || created {
		t.Fatalf("replay task: created=%v err=%v", created, err)
	}
	conflict := task
	conflict.PrincipalUUID = "U999"
	if _, err := store.CreateTask(context.Background(), conflict); !errors.Is(err, sqlcRepository.ErrAgentPolicyConflict) {
		t.Fatalf("expected task conflict, got %v", err)
	}
	if changed, err := store.TransitionTaskStatus(context.Background(), task.TaskUUID, application.AgentTaskStatusCreated, application.AgentTaskStatusRunning); err != nil || !changed {
		t.Fatalf("start task: changed=%v err=%v", changed, err)
	}
	runUUID, err := application.AgentRunUUIDV1(task.TaskUUID, "dipole-agent", "shadow")
	if err != nil {
		t.Fatalf("derive Agent Run UUID: %v", err)
	}
	run := application.AgentRunV1{RunUUID: runUUID, TaskUUID: task.TaskUUID, RuntimeID: "dipole-agent", Mode: "shadow", Status: application.AgentRunStatusRunning}
	if created, err := store.CreateRun(context.Background(), run); err != nil || !created {
		t.Fatalf("create Agent Run: created=%v err=%v", created, err)
	}
	projection := application.AgentTaskWorkflowProjectionV1{
		TaskUUID: task.TaskUUID, WorkflowID: "dipole-agent-task/TASK-1", RunID: "temporal-run-1",
		Status: application.AgentTaskWorkflowStatusRunning, Revision: 1,
	}
	var projectionApplied atomic.Int64
	projectionErrors := make(chan error, 16)
	var projectionWorkers sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		projectionWorkers.Add(1)
		go func() {
			defer projectionWorkers.Done()
			applied, projectErr := store.ProjectTaskWorkflowState(context.Background(), projection)
			if projectErr != nil {
				projectionErrors <- projectErr
				return
			}
			if applied {
				projectionApplied.Add(1)
			}
		}()
	}
	projectionWorkers.Wait()
	close(projectionErrors)
	for projectErr := range projectionErrors {
		t.Fatalf("concurrent Workflow projection: %v", projectErr)
	}
	if projectionApplied.Load() != 1 {
		t.Fatalf("concurrent Workflow projection applied %d times, want 1", projectionApplied.Load())
	}
	if applied, err := store.ProjectTaskWorkflowState(context.Background(), projection); err != nil || applied {
		t.Fatalf("replay Workflow projection: applied=%v err=%v", applied, err)
	}
	nextProjection := projection
	nextProjection.Status, nextProjection.Revision = application.AgentTaskWorkflowStatusWaitingInput, 2
	if applied, err := store.ProjectTaskWorkflowState(context.Background(), nextProjection); err != nil || !applied {
		t.Fatalf("advance Workflow projection: applied=%v err=%v", applied, err)
	}
	loadedTask, err := store.GetTask(context.Background(), task.TaskUUID)
	if err != nil || loadedTask == nil || loadedTask.Status != application.AgentTaskStatusRunning || loadedTask.Workflow == nil ||
		loadedTask.Workflow.Status != application.AgentTaskWorkflowStatusWaitingInput || loadedTask.Workflow.Revision != 2 {
		t.Fatalf("loaded Workflow projection: task=%+v err=%v", loadedTask, err)
	}
	missingProjectionTask := task
	missingProjectionTask.TaskUUID, missingProjectionTask.TriggerRef = "TASK-2", "M200"
	missingProjectionTask.Status = application.AgentTaskStatusCreated
	if created, err := store.CreateTask(context.Background(), missingProjectionTask); err != nil || !created {
		t.Fatalf("create missing-projection task: created=%v err=%v", created, err)
	}
	missingRunUUID, _ := application.AgentRunUUIDV1(missingProjectionTask.TaskUUID, "dipole-agent", "shadow")
	if created, err := store.CreateRun(context.Background(), application.AgentRunV1{
		RunUUID: missingRunUUID, TaskUUID: missingProjectionTask.TaskUUID, RuntimeID: "dipole-agent", Mode: "shadow", Status: application.AgentRunStatusRunning,
	}); err != nil || !created {
		t.Fatalf("create missing-projection run: created=%v err=%v", created, err)
	}
	projectionPage, err := store.ListTaskWorkflowProjectionSnapshots(context.Background(), "dipole-agent", "shadow", "", 10)
	if err != nil || len(projectionPage) != 2 || projectionPage[0].TaskUUID != task.TaskUUID || projectionPage[0].Workflow == nil || projectionPage[0].Workflow.Revision != 2 ||
		projectionPage[1].TaskUUID != missingProjectionTask.TaskUUID || projectionPage[1].Workflow != nil {
		t.Fatalf("list Workflow projection snapshots: page=%+v err=%v", projectionPage, err)
	}
	if page, err := store.ListTaskWorkflowProjectionSnapshots(context.Background(), "dipole-agent", "shadow", task.TaskUUID, 10); err != nil || len(page) != 1 || page[0].TaskUUID != missingProjectionTask.TaskUUID {
		t.Fatalf("list Workflow projection snapshots after cursor: page=%+v err=%v", page, err)
	}
	for _, conflictProjection := range []application.AgentTaskWorkflowProjectionV1{
		projection,
		{TaskUUID: task.TaskUUID, WorkflowID: projection.WorkflowID, RunID: projection.RunID, Status: application.AgentTaskWorkflowStatusWaitingApproval, Revision: 2},
		{TaskUUID: task.TaskUUID, WorkflowID: "other-workflow", RunID: projection.RunID, Status: application.AgentTaskWorkflowStatusRunning, Revision: 3},
	} {
		if _, err := store.ProjectTaskWorkflowState(context.Background(), conflictProjection); !errors.Is(err, sqlcRepository.ErrAgentPolicyConflict) {
			t.Fatalf("expected Workflow projection conflict for %+v, got %v", conflictProjection, err)
		}
	}
	if created, err := store.CreateRun(context.Background(), run); err != nil || created {
		t.Fatalf("replay Agent Run: created=%v err=%v", created, err)
	}
	conflictingRun := run
	conflictingRun.RuntimeID = "other-runtime"
	if _, err := store.CreateRun(context.Background(), conflictingRun); !errors.Is(err, sqlcRepository.ErrAgentPolicyConflict) {
		t.Fatalf("expected Agent Run conflict, got %v", err)
	}
	if changed, err := store.TransitionRunStatus(context.Background(), runUUID, application.AgentRunStatusRunning, application.AgentRunStatusCompleted, ""); err != nil || !changed {
		t.Fatalf("complete Agent Run: changed=%v err=%v", changed, err)
	}
	if changed, err := store.TransitionRunStatus(context.Background(), runUUID, application.AgentRunStatusRunning, application.AgentRunStatusFailed, "stale"); err != nil || changed {
		t.Fatalf("stale Agent Run transition: changed=%v err=%v", changed, err)
	}
	if changed, err := store.TransitionTaskStatus(context.Background(), task.TaskUUID, application.AgentTaskStatusCreated, application.AgentTaskStatusRunning); err != nil || changed {
		t.Fatalf("stale task transition: changed=%v err=%v", changed, err)
	}
	if _, err := store.TransitionTaskStatus(context.Background(), task.TaskUUID, application.AgentTaskStatusRunning, application.AgentTaskStatusCreated); !errors.Is(err, application.ErrAgentPolicyInvalid) {
		t.Fatalf("expected invalid reverse transition, got %v", err)
	}

	approval := application.AgentApprovalV1{
		ApprovalUUID: "APR-1", TaskUUID: task.TaskUUID, CapabilityID: "message.bulk.send",
		ResourceScope:   application.AgentResourceScopeV1{ResourceType: "conversation", ResourceID: "G1", Actions: []string{"write"}},
		ArgumentsSHA256: strings.Repeat("b", 64), NonceSHA256: strings.Repeat("c", 64),
		Status: application.AgentApprovalStatusApproved, ApprovedByUUID: "U100", ExpiresAt: now.Add(time.Hour),
	}
	approval.ScopeSHA256, err = application.AgentResourceScopeSHA256V1(approval.ResourceScope)
	if err != nil {
		t.Fatalf("hash approval scope: %v", err)
	}
	if err := store.CreateApproval(context.Background(), approval); err != nil {
		t.Fatalf("create approval: %v", err)
	}
	if err := store.CreateApproval(context.Background(), approval); err != nil {
		t.Fatalf("replay exact approval: %v", err)
	}
	loadedApproval, err := store.GetApproval(context.Background(), approval.ApprovalUUID)
	if err != nil || loadedApproval == nil || loadedApproval.NonceSHA256 != approval.NonceSHA256 || loadedApproval.ResourceScope.ResourceID != approval.ResourceScope.ResourceID {
		t.Fatalf("get exact approval: approval=%+v err=%v", loadedApproval, err)
	}
	conflictingApproval := approval
	conflictingApproval.ArgumentsSHA256 = strings.Repeat("9", 64)
	if err := store.CreateApproval(context.Background(), conflictingApproval); !errors.Is(err, sqlcRepository.ErrAgentPolicyConflict) {
		t.Fatalf("expected approval conflict, got %v", err)
	}
	pending := approval
	pending.ApprovalUUID, pending.NonceSHA256 = "APR-0", strings.Repeat("0", 64)
	pending.Status, pending.ApprovedByUUID = application.AgentApprovalStatusPending, ""
	if err := store.CreateApproval(context.Background(), pending); err != nil {
		t.Fatalf("create pending approval: %v", err)
	}
	if approved, err := store.ApproveApproval(context.Background(), pending.ApprovalUUID, "U100", now); err != nil || !approved {
		t.Fatalf("approve pending approval: approved=%v err=%v", approved, err)
	}
	if approved, err := store.ApproveApproval(context.Background(), pending.ApprovalUUID, "U100", now); err != nil || approved {
		t.Fatalf("replay approval transition: approved=%v err=%v", approved, err)
	}
	claim := application.AgentApprovalClaimV1{TaskUUID: task.TaskUUID, CapabilityID: approval.CapabilityID, ScopeSHA256: approval.ScopeSHA256, ArgumentsSHA256: strings.Repeat("d", 64), NonceSHA256: approval.NonceSHA256}
	if consumed, err := store.ConsumeApproval(context.Background(), approval.ApprovalUUID, claim, now); err != nil || consumed {
		t.Fatalf("mismatched consume: consumed=%v err=%v", consumed, err)
	}
	claim.ArgumentsSHA256 = approval.ArgumentsSHA256
	if consumed, err := store.ConsumeApproval(context.Background(), approval.ApprovalUUID, claim, now); err != nil || !consumed {
		t.Fatalf("exact consume: consumed=%v err=%v", consumed, err)
	}
	raceApproval := pending
	raceApproval.ApprovalUUID, raceApproval.NonceSHA256 = "APR-RACE", strings.Repeat("6", 64)
	if err := store.CreateApproval(context.Background(), raceApproval); err != nil {
		t.Fatalf("create decision-race approval: %v", err)
	}
	var decisionWins atomic.Int64
	var decisionWait sync.WaitGroup
	decisionWait.Add(2)
	go func() {
		defer decisionWait.Done()
		changed, _ := store.ApproveApproval(context.Background(), raceApproval.ApprovalUUID, "U100", now)
		if changed {
			decisionWins.Add(1)
		}
	}()
	go func() {
		defer decisionWait.Done()
		changed, _ := store.DenyApproval(context.Background(), raceApproval.ApprovalUUID, now)
		if changed {
			decisionWins.Add(1)
		}
	}()
	decisionWait.Wait()
	decisionResult, err := store.GetApproval(context.Background(), raceApproval.ApprovalUUID)
	if err != nil || decisionWins.Load() != 1 || (decisionResult.Status != application.AgentApprovalStatusApproved && decisionResult.Status != application.AgentApprovalStatusRevoked) {
		t.Fatalf("Approval decision race: wins=%d approval=%+v err=%v", decisionWins.Load(), decisionResult, err)
	}
	if consumed, err := store.ConsumeApproval(context.Background(), approval.ApprovalUUID, claim, now); err != nil || consumed {
		t.Fatalf("replayed consume: consumed=%v err=%v", consumed, err)
	}

	revoked := approval
	revoked.ApprovalUUID, revoked.NonceSHA256 = "APR-2", strings.Repeat("e", 64)
	if err := store.CreateApproval(context.Background(), revoked); err != nil {
		t.Fatalf("create revocable approval: %v", err)
	}
	if err := store.RevokeApproval(context.Background(), revoked.ApprovalUUID, now); err != nil {
		t.Fatalf("revoke approval: %v", err)
	}
	claim.NonceSHA256 = revoked.NonceSHA256
	if consumed, err := store.ConsumeApproval(context.Background(), revoked.ApprovalUUID, claim, now); err != nil || consumed {
		t.Fatalf("revoked consume: consumed=%v err=%v", consumed, err)
	}

	concurrent := approval
	concurrent.ApprovalUUID, concurrent.NonceSHA256 = "APR-3", strings.Repeat("f", 64)
	if err := store.CreateApproval(context.Background(), concurrent); err != nil {
		t.Fatalf("create concurrent approval: %v", err)
	}
	claim.NonceSHA256 = concurrent.NonceSHA256
	var successes atomic.Int64
	var failures atomic.Int64
	var wait sync.WaitGroup
	for range 16 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			consumed, err := store.ConsumeApproval(context.Background(), concurrent.ApprovalUUID, claim, now)
			if err != nil {
				failures.Add(1)
				return
			}
			if consumed {
				successes.Add(1)
			}
		}()
	}
	wait.Wait()
	if failures.Load() != 0 || successes.Load() != 1 {
		t.Fatalf("concurrent approval consumption: successes=%d failures=%d", successes.Load(), failures.Load())
	}

	permissions, scopes := application.EmbeddedAgentPolicyGrantV1()
	const persistentAgentUUID = "UAI000000000000000001"
	const persistentPrincipalUUID = "USR000000000000000001"
	if err := appComposition.EnsureEmbeddedAgentDefinitionV1(context.Background(), store, "dipole", persistentAgentUUID, permissions, scopes); err != nil {
		t.Fatalf("ensure persistent Embedded Definition: %v", err)
	}
	policy, err := appComposition.NewPersistentAgentExecutionPolicyV1(store)
	if err != nil {
		t.Fatalf("create persistent execution policy: %v", err)
	}
	execution, err := policy.Start(context.Background(), application.AgentExecutionPolicyStartV1{
		TenantID: "dipole", PrincipalUUID: persistentPrincipalUUID, AgentUUID: persistentAgentUUID, DelegatedByUUID: persistentPrincipalUUID,
		TriggerType: "message.direct.created", TriggerRef: "M-PERSISTENT", RequestID: "R-PERSISTENT", TraceID: "T-PERSISTENT", EventID: "E-PERSISTENT",
	})
	if err != nil {
		t.Fatalf("start persistent execution policy: %v", err)
	}
	if len(execution.TaskUUID) != 64 || len(execution.Invocation.Permissions) != len(permissions) || len(execution.Invocation.ResourceScopes) != len(scopes) {
		t.Fatalf("unexpected persistent execution snapshot: %+v", execution)
	}
	if len(execution.RunUUID) != 64 {
		t.Fatalf("persistent execution did not create a stable Run: %+v", execution)
	}
	if err := policy.Complete(context.Background(), *execution); err != nil {
		t.Fatalf("complete persistent execution policy: %v", err)
	}
	persistedTask, err := store.GetTask(context.Background(), execution.TaskUUID)
	if err != nil || persistedTask == nil || persistedTask.Status != application.AgentTaskStatusCompleted || persistedTask.DefinitionVersion != embeddedAgentDefinitionVersionForContract {
		t.Fatalf("persistent execution Task: %+v err=%v", persistedTask, err)
	}
	persistedRun, err := store.GetRun(context.Background(), execution.RunUUID)
	if err != nil || persistedRun == nil || persistedRun.Status != application.AgentRunStatusCompleted {
		t.Fatalf("persistent execution Run: %+v err=%v", persistedRun, err)
	}

	admission, err := appComposition.NewPersistentAgentRunAdmissionV1(store)
	if err != nil {
		t.Fatalf("create persistent Run admission: %v", err)
	}
	for _, terminal := range []struct {
		triggerRef string
		status     application.AgentRunStatusV1
		lastError  string
	}{
		{triggerRef: "M-TEMPORAL-FAILED", status: application.AgentRunStatusFailed, lastError: "Activity retries exhausted"},
		{triggerRef: "M-TEMPORAL-CANCELLED", status: application.AgentRunStatusCancelled, lastError: "approval_denied"},
	} {
		admitted, admitErr := admission.Admit(context.Background(), application.AgentRunAdmissionRequestV1{
			AgentExecutionPolicyStartV1: application.AgentExecutionPolicyStartV1{
				TenantID: "dipole", PrincipalUUID: persistentPrincipalUUID, AgentUUID: persistentAgentUUID,
				DelegatedByUUID: persistentPrincipalUUID, TriggerType: "message.direct.created", TriggerRef: terminal.triggerRef,
				RequestID: "R-" + terminal.triggerRef, TraceID: "T-" + terminal.triggerRef, EventID: "E-" + terminal.triggerRef,
			},
			RuntimeID: "dipole-agent", Mode: "shadow",
		})
		if admitErr != nil {
			t.Fatalf("admit %s Temporal Run: %v", terminal.status, admitErr)
		}
		if finishErr := admission.Finish(context.Background(), admitted.TaskUUID, admitted.RunUUID, "dipole-agent", "shadow", terminal.status, terminal.lastError); finishErr != nil {
			t.Fatalf("finish %s Temporal Run: %v", terminal.status, finishErr)
		}
		if finishErr := admission.Finish(context.Background(), admitted.TaskUUID, admitted.RunUUID, "dipole-agent", "shadow", terminal.status, terminal.lastError); finishErr != nil {
			t.Fatalf("replay %s Temporal Run: %v", terminal.status, finishErr)
		}
		persistedTerminal, lookupErr := store.GetRun(context.Background(), admitted.RunUUID)
		if lookupErr != nil || persistedTerminal == nil || persistedTerminal.Status != terminal.status || persistedTerminal.LastError != terminal.lastError {
			t.Fatalf("persisted %s Temporal Run: %+v err=%v", terminal.status, persistedTerminal, lookupErr)
		}
		if finishErr := admission.Finish(context.Background(), admitted.TaskUUID, admitted.RunUUID, "dipole-agent", "shadow", application.AgentRunStatusCompleted, ""); !errors.Is(finishErr, application.ErrAgentExecutionPolicyDenied) {
			t.Fatalf("conflicting %s Temporal terminal replay should be denied, got %v", terminal.status, finishErr)
		}
	}

	durableRun, err := admission.Admit(context.Background(), application.AgentRunAdmissionRequestV1{
		AgentExecutionPolicyStartV1: application.AgentExecutionPolicyStartV1{
			TenantID: "dipole", PrincipalUUID: persistentPrincipalUUID, AgentUUID: persistentAgentUUID,
			DelegatedByUUID: persistentPrincipalUUID, TriggerType: "message.direct.created", TriggerRef: "M-TEMPORAL-APPROVAL",
			RequestID: "R-TEMPORAL-APPROVAL", TraceID: "T-TEMPORAL-APPROVAL", EventID: "E-TEMPORAL-APPROVAL",
		}, RuntimeID: "dipole-agent", Mode: "shadow",
	})
	if err != nil {
		t.Fatalf("admit durable Approval Run: %v", err)
	}
	approvalService, err := appComposition.NewPersistentAgentApprovalServiceV1(store)
	if err != nil {
		t.Fatalf("create persistent Approval service: %v", err)
	}
	durableScope := application.AgentResourceScopeV1{ResourceType: "conversation", ResourceID: "G-DURABLE", Actions: []string{"write"}}
	durableScopeHash, _ := application.AgentResourceScopeSHA256V1(durableScope)
	durableApproval := application.AgentApprovalV1{
		ApprovalUUID: "APR-DURABLE", TaskUUID: durableRun.TaskUUID, CapabilityID: "message.bulk.send", ResourceScope: durableScope,
		ScopeSHA256: durableScopeHash, ArgumentsSHA256: strings.Repeat("7", 64), NonceSHA256: strings.Repeat("8", 64),
		Status: application.AgentApprovalStatusPending, ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	approvalRequest := application.AgentApprovalRequestV1{TaskUUID: durableRun.TaskUUID, RunUUID: durableRun.RunUUID, RuntimeID: "dipole-agent", Mode: "shadow", Approval: durableApproval}
	if _, err := approvalService.Request(context.Background(), approvalRequest); err != nil {
		t.Fatalf("persist durable Approval: %v", err)
	}
	resolution := application.AgentApprovalResolutionV1{TaskUUID: durableRun.TaskUUID, RunUUID: durableRun.RunUUID, RuntimeID: "dipole-agent", Mode: "shadow", ApprovalUUID: durableApproval.ApprovalUUID, ActorUUID: persistentPrincipalUUID, Decision: application.AgentApprovalDecisionApproved}
	var resolutionFailures atomic.Int64
	var resolutionWait sync.WaitGroup
	for range 16 {
		resolutionWait.Add(1)
		go func() {
			defer resolutionWait.Done()
			resolved, resolveErr := approvalService.Resolve(context.Background(), resolution)
			if resolveErr != nil || resolved.Status != application.AgentApprovalStatusApproved {
				resolutionFailures.Add(1)
			}
		}()
	}
	resolutionWait.Wait()
	if resolutionFailures.Load() != 0 {
		t.Fatalf("concurrent durable Approval resolution failures=%d", resolutionFailures.Load())
	}
	if resolved, err := approvalService.Resolve(context.Background(), resolution); err != nil || resolved.ApprovedByUUID != persistentPrincipalUUID {
		t.Fatalf("replay durable Approval resolution: approval=%+v err=%v", resolved, err)
	}
}

const embeddedAgentDefinitionVersionForContract uint64 = 1
