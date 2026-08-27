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
}
