package agentmysql_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/data/migration"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	agentapplication "github.com/JekYUlll/Dipole/internal/services/agent/application"
	sqlcRepository "github.com/JekYUlll/Dipole/internal/services/agent/infrastructure/mysql"
)

func TestAgentRuntimePromotionGrantRepositoryContract(t *testing.T) {
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
	definition := application.AgentDefinitionVersionV1{
		DefinitionUUID: "DEF-PROMOTION", Version: 7, TenantID: "dipole", OwnerUUID: "U100", AgentUUID: "UAI",
		Status: application.AgentDefinitionStatusActive, Permissions: []string{application.AgentPermissionMessageWrite},
		Scopes:    []application.AgentResourceScopeV1{{ResourceType: "conversation", ResourceID: "*", Actions: []string{"write"}}},
		ValidFrom: now.Add(-time.Hour),
	}
	if err := store.CreateDefinitionVersion(context.Background(), definition); err != nil {
		t.Fatalf("create pinned Definition: %v", err)
	}
	grant := application.AgentRuntimePromotionGrantV1{
		GrantUUID: "PROMOTION-GRANT-1", TenantID: "dipole", RuntimeID: "dipole-agent", CandidateVersion: "runtime-v7",
		DefinitionUUID: definition.DefinitionUUID, DefinitionVersion: definition.Version,
		PolicyVersion: application.AgentRuntimePromotionPolicyVersionV2, EvidenceSHA256: strings.Repeat("a", 64),
		EvalSuiteSHA256: strings.Repeat("b", 64), GrantedByUUID: "OPERATOR-1", ReviewedByUUID: "OPERATOR-2",
		ValidFrom: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour),
	}
	created, err := store.CreateRuntimePromotionGrant(context.Background(), grant)
	if err != nil || !created {
		t.Fatalf("create promotion grant: created=%v err=%v", created, err)
	}
	if replayed, replayErr := store.CreateRuntimePromotionGrant(context.Background(), grant); replayErr != nil || replayed {
		t.Fatalf("replay promotion grant: created=%v err=%v", replayed, replayErr)
	}
	conflict := grant
	conflict.GrantUUID = "PROMOTION-GRANT-2"
	if created, conflictErr := store.CreateRuntimePromotionGrant(context.Background(), conflict); created || !errors.Is(conflictErr, sqlcRepository.ErrAgentPolicyConflict) {
		t.Fatalf("conflicting promotion binding: created=%v err=%v", created, conflictErr)
	}
	lookup := application.AgentRuntimePromotionGrantLookupV1{
		TenantID: grant.TenantID, RuntimeID: grant.RuntimeID, CandidateVersion: grant.CandidateVersion,
		DefinitionUUID: grant.DefinitionUUID, DefinitionVersion: grant.DefinitionVersion, At: now,
	}
	active, err := store.GetActiveRuntimePromotionGrant(context.Background(), lookup)
	if err != nil || active == nil || active.GrantUUID != grant.GrantUUID || active.EvidenceSHA256 != grant.EvidenceSHA256 || active.EvalSuiteSHA256 != grant.EvalSuiteSHA256 {
		t.Fatalf("get active promotion grant: grant=%+v err=%v", active, err)
	}
	drifted := lookup
	drifted.CandidateVersion = "runtime-v8"
	if mismatched, lookupErr := store.GetActiveRuntimePromotionGrant(context.Background(), drifted); lookupErr != nil || mismatched != nil {
		t.Fatalf("candidate drift matched grant: grant=%+v err=%v", mismatched, lookupErr)
	}
	authorizer, err := agentapplication.NewPersistentAgentActiveRunPromotionAuthorizerV1(store)
	if err != nil {
		t.Fatalf("create persistent promotion authorizer: %v", err)
	}
	request := application.AgentActiveRunPromotionRequestV1{
		RuntimeID: grant.RuntimeID, CandidateVersion: grant.CandidateVersion,
		Task: application.AgentTaskV1{
			TaskUUID: "TASK-PROMOTION", DefinitionUUID: definition.DefinitionUUID, DefinitionVersion: definition.Version,
			TenantID: definition.TenantID, PrincipalUUID: "U100", AgentUUID: definition.AgentUUID,
			Status: application.AgentTaskStatusCreated, TriggerType: "manual", TriggerRef: "promotion-contract", Goal: "promotion contract",
		},
		Definition: definition,
	}
	if err := authorizer.AuthorizeActiveRun(context.Background(), request); err != nil {
		t.Fatalf("authorize active Run from persisted grant: %v", err)
	}
	admission, err := agentapplication.NewPersistentAgentRunAdmissionV1(store, authorizer)
	if err != nil {
		t.Fatalf("create active Run admission: %v", err)
	}
	admitted, err := admission.Admit(context.Background(), application.AgentRunAdmissionRequestV1{
		AgentExecutionPolicyStartV1: application.AgentExecutionPolicyStartV1{
			TenantID: definition.TenantID, PrincipalUUID: "U100", AgentUUID: definition.AgentUUID,
			DelegatedByUUID: "U100", TriggerType: "manual", TriggerRef: "promotion-contract-admit",
		},
		RuntimeID: grant.RuntimeID, Mode: "active", CandidateVersion: grant.CandidateVersion,
	})
	if err != nil || admitted.Invocation.Mode != "active" || admitted.Invocation.RuntimeID != grant.RuntimeID ||
		len(admitted.Invocation.ApprovedCapabilities) != 1 || admitted.Invocation.ApprovedCapabilities[0] != application.AgentCapabilitySystemMessageSend {
		t.Fatalf("admit active Run from persisted grant: admission=%+v err=%v", admitted, err)
	}
	failClosedResolver, err := agentapplication.NewPersistentAgentInvocationResolverV1(store)
	if err != nil {
		t.Fatalf("create fail-closed Invocation resolver: %v", err)
	}
	if _, err := failClosedResolver.Resolve(context.Background(), admitted.TaskUUID, admitted.RunUUID); !errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
		t.Fatalf("active context without promotion authorizer error = %v, want policy denied", err)
	}
	resolver, err := agentapplication.NewPersistentAgentInvocationResolverV1(store, authorizer)
	if err != nil {
		t.Fatalf("create promoted Invocation resolver: %v", err)
	}
	if invocation, resolveErr := resolver.Resolve(context.Background(), admitted.TaskUUID, admitted.RunUUID); resolveErr != nil || invocation.Mode != "active" ||
		len(invocation.ApprovedCapabilities) != 1 || invocation.ApprovedCapabilities[0] != application.AgentCapabilitySystemMessageSend {
		t.Fatalf("resolve active context from persisted grant: invocation=%+v err=%v", invocation, resolveErr)
	}
	if revoked, revokeErr := store.RevokeRuntimePromotionGrant(context.Background(), grant.GrantUUID, now); revokeErr != nil || !revoked {
		t.Fatalf("revoke promotion grant: revoked=%v err=%v", revoked, revokeErr)
	}
	if revoked, revokeErr := store.RevokeRuntimePromotionGrant(context.Background(), grant.GrantUUID, now); revokeErr != nil || revoked {
		t.Fatalf("replay promotion revocation: revoked=%v err=%v", revoked, revokeErr)
	}
	if active, lookupErr := store.GetActiveRuntimePromotionGrant(context.Background(), lookup); lookupErr != nil || active != nil {
		t.Fatalf("revoked promotion grant remained active: grant=%+v err=%v", active, lookupErr)
	}
	if _, err := resolver.Resolve(context.Background(), admitted.TaskUUID, admitted.RunUUID); !errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
		t.Fatalf("revoked grant active context error = %v, want policy denied", err)
	}
	if _, err := admission.Admit(context.Background(), application.AgentRunAdmissionRequestV1{
		AgentExecutionPolicyStartV1: application.AgentExecutionPolicyStartV1{
			TenantID: definition.TenantID, PrincipalUUID: "U100", AgentUUID: definition.AgentUUID,
			DelegatedByUUID: "U100", TriggerType: "manual", TriggerRef: "promotion-contract-revoked",
		},
		RuntimeID: grant.RuntimeID, Mode: "active", CandidateVersion: grant.CandidateVersion,
	}); !errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
		t.Fatalf("revoked grant active admission error = %v, want policy denied", err)
	}
}
