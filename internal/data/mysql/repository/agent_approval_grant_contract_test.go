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

func TestAgentApprovalGrantRepositoryContract(t *testing.T) {
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
		t.Fatalf("create policy repository: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	task := application.AgentTaskV1{
		TaskUUID: "TASK-GRANT", DefinitionUUID: "DEF-GRANT", DefinitionVersion: 1, TenantID: "dipole",
		PrincipalUUID: "U100", AgentUUID: "UAI", Status: application.AgentTaskStatusRunning,
		TriggerType: "manual", TriggerRef: "grant-contract", Goal: "grant contract",
	}
	if created, createErr := store.CreateTask(context.Background(), task); createErr != nil || !created {
		t.Fatalf("create task: created=%v err=%v", created, createErr)
	}
	if created, createErr := store.CreateRun(context.Background(), application.AgentRunV1{
		RunUUID: "RUN-GRANT", TaskUUID: task.TaskUUID, RuntimeID: "dipole-agent", Mode: "active", Status: application.AgentRunStatusRunning,
	}); createErr != nil || !created {
		t.Fatalf("create run: created=%v err=%v", created, createErr)
	}
	scope := application.AgentResourceScopeV1{ResourceType: "conversation", ResourceID: "direct:U100:UAI", Actions: []string{"write"}}
	scopeHash, _ := application.AgentResourceScopeSHA256V1(scope)
	base := application.AgentApprovalV1{
		TaskUUID: task.TaskUUID, CapabilityID: "message.system.send", ResourceScope: scope, ScopeSHA256: scopeHash,
		ArgumentsSHA256: strings.Repeat("a", 64), Status: application.AgentApprovalStatusApproved,
		ApprovedByUUID: task.PrincipalUUID, ExpiresAt: now.Add(time.Hour),
	}
	first := base
	first.ApprovalUUID, first.NonceSHA256 = "APR-GRANT-1", strings.Repeat("b", 64)
	second := base
	second.ApprovalUUID, second.NonceSHA256 = "APR-GRANT-2", strings.Repeat("c", 64)
	for _, approval := range []application.AgentApprovalV1{first, second} {
		if err := store.CreateApproval(context.Background(), approval); err != nil {
			t.Fatalf("create approval %s: %v", approval.ApprovalUUID, err)
		}
	}
	grants, err := store.ListApprovedAgentApprovalGrants(context.Background(), task.TaskUUID, base.CapabilityID, scopeHash, base.ArgumentsSHA256, now, 2)
	if err != nil || len(grants) != 2 {
		t.Fatalf("ambiguous grants: grants=%+v err=%v", grants, err)
	}
	claim := application.AgentApprovalClaimV1{
		TaskUUID: task.TaskUUID, CapabilityID: first.CapabilityID, ScopeSHA256: scopeHash,
		ArgumentsSHA256: first.ArgumentsSHA256, NonceSHA256: first.NonceSHA256,
	}
	if consumed, consumeErr := store.ConsumeApproval(context.Background(), first.ApprovalUUID, claim, now); consumeErr != nil || !consumed {
		t.Fatalf("consume first grant: consumed=%v err=%v", consumed, consumeErr)
	}
	grants, err = store.ListApprovedAgentApprovalGrants(context.Background(), task.TaskUUID, base.CapabilityID, scopeHash, base.ArgumentsSHA256, now, 2)
	if err != nil || len(grants) != 1 || grants[0].ApprovalUUID != second.ApprovalUUID {
		t.Fatalf("remaining grant: grants=%+v err=%v", grants, err)
	}
	if err := store.RevokeApproval(context.Background(), second.ApprovalUUID, now); err != nil {
		t.Fatalf("revoke second grant: %v", err)
	}
	grants, err = store.ListApprovedAgentApprovalGrants(context.Background(), task.TaskUUID, base.CapabilityID, scopeHash, base.ArgumentsSHA256, now, 2)
	if err != nil || len(grants) != 0 {
		t.Fatalf("revoked grants should be absent: grants=%+v err=%v", grants, err)
	}
}
