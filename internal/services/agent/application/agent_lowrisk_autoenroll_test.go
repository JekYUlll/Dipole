package agentapplication_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	agentapplication "github.com/JekYUlll/Dipole/internal/services/agent/application"
)

// interactiveAdmissionRequest builds an active-mode interactive admission for a
// principal with no owner-scoped Definition.
func interactiveAdmissionRequest() application.AgentRunAdmissionRequestV1 {
	return application.AgentRunAdmissionRequestV1{
		AgentExecutionPolicyStartV1: application.AgentExecutionPolicyStartV1{
			TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", DelegatedByUUID: "U100",
			TriggerType: "agent.interactive.requested", TriggerRef: "interactive:abc", RequestID: "R1", TraceID: "T1", EventID: "E1",
		},
		RuntimeID: "dipole-agent", Mode: "active", CandidateVersion: "candidate-v1",
	}
}

func TestAdmitAutoEnrollsFirstContactInteractiveOntoLowRiskDefinition(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 6, 8, 0, 0, 0, time.UTC)
	// No owner Definition for U100: the store starts empty.
	store := &agentPolicyStoreStub{}
	authorizer := &activeRunPromotionAuthorizerStub{}
	admission, err := agentapplication.NewPersistentAgentRunAdmissionV1WithClock(store, func() time.Time { return now }, authorizer)
	if err != nil {
		t.Fatalf("new Run admission: %v", err)
	}

	result, err := admission.Admit(context.Background(), interactiveAdmissionRequest())
	if err != nil {
		t.Fatalf("Admit first-contact interactive: %v", err)
	}
	if result == nil || result.TaskUUID == "" {
		t.Fatalf("Admit result = %+v", result)
	}
	// The shared low-risk Definition must have been created and pinned to the task.
	created := store.definitions[definitionKeyV1(agentapplication.LowRiskAssistantDefinitionUUIDV1, 1)]
	if created == nil {
		t.Fatalf("low-risk assistant Definition was not created: %+v", store.definitions)
	}
	if created.AgentUUID != "UAI" {
		t.Fatalf("low-risk Definition agent = %s, want the Agent", created.AgentUUID)
	}
	if created.OwnerUUID == "" || created.OwnerUUID == created.AgentUUID {
		t.Fatalf("low-risk Definition owner must be a distinct platform identity, got %q", created.OwnerUUID)
	}
	task := store.tasks[result.TaskUUID]
	if task == nil || task.DefinitionUUID != agentapplication.LowRiskAssistantDefinitionUUIDV1 {
		t.Fatalf("task pinned Definition = %+v", task)
	}
	// Active authorization must have been consulted for the shared Definition.
	if authorizer.request.Definition.DefinitionUUID != agentapplication.LowRiskAssistantDefinitionUUIDV1 {
		t.Fatalf("active authorizer saw Definition %q", authorizer.request.Definition.DefinitionUUID)
	}
}

// ungrantedOwnerPromotionAuthorizer allows only the shared low-risk Definition.
// This matches the experience stack: the platform grant promotes lowrisk-assistant:v1,
// not a user-created catalog Definition.
type ungrantedOwnerPromotionAuthorizer struct {
	seen []string
}

func (s *ungrantedOwnerPromotionAuthorizer) AuthorizeActiveRun(_ context.Context, request application.AgentActiveRunPromotionRequestV1) error {
	s.seen = append(s.seen, request.Definition.DefinitionUUID)
	if request.Definition.DefinitionUUID == agentapplication.LowRiskAssistantDefinitionUUIDV1 {
		return nil
	}
	return fmt.Errorf("%w: active Runtime promotion grant is unavailable", application.ErrAgentExecutionPolicyDenied)
}

func TestAdmitFallsBackToLowRiskWhenOwnerDefinitionHasNoPromotionGrant(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 6, 8, 0, 0, 0, time.UTC)
	ownerDefinition := application.AgentDefinitionVersionV1{
		DefinitionUUID: "user:owner-def", Version: 1,
		TenantID: "dipole", OwnerUUID: "U100", AgentUUID: "UAI", Status: application.AgentDefinitionStatusActive,
		Permissions: []string{application.AgentPermissionConversationRead, application.AgentPermissionMessageWrite},
		Scopes: []application.AgentResourceScopeV1{{
			ResourceType: application.AgentResourceTypeConversation, ResourceID: application.AgentResourceWildcard,
			Actions: []string{application.AgentResourceActionRead, application.AgentResourceActionWrite},
		}},
		ValidFrom: now.Add(-time.Hour),
	}
	store := &agentPolicyStoreStub{
		latestByOwner: map[string]*application.AgentDefinitionVersionV1{"U100": &ownerDefinition},
		definitions: map[string]*application.AgentDefinitionVersionV1{
			definitionKeyV1(ownerDefinition.DefinitionUUID, ownerDefinition.Version): &ownerDefinition,
		},
	}
	authorizer := &ungrantedOwnerPromotionAuthorizer{}
	admission, err := agentapplication.NewPersistentAgentRunAdmissionV1WithClock(store, func() time.Time { return now }, authorizer)
	if err != nil {
		t.Fatalf("new Run admission: %v", err)
	}

	result, err := admission.Admit(context.Background(), interactiveAdmissionRequest())
	if err != nil {
		t.Fatalf("Admit with ungranted owner Definition: %v", err)
	}
	task := store.tasks[result.TaskUUID]
	if task == nil || task.DefinitionUUID != agentapplication.LowRiskAssistantDefinitionUUIDV1 {
		t.Fatalf("task pinned Definition = %+v, want low-risk fallback", task)
	}
	if len(authorizer.seen) < 2 || authorizer.seen[0] != "user:owner-def" || authorizer.seen[len(authorizer.seen)-1] != agentapplication.LowRiskAssistantDefinitionUUIDV1 {
		t.Fatalf("promotion lookups = %v, want owner then low-risk", authorizer.seen)
	}
}

func TestAdmitDoesNotFallBackForSubscriptionWhenOwnerDefinitionHasNoGrant(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 6, 8, 0, 0, 0, time.UTC)
	ownerDefinition := application.AgentDefinitionVersionV1{
		DefinitionUUID: "user:owner-def", Version: 1,
		TenantID: "dipole", OwnerUUID: "U100", AgentUUID: "UAI", Status: application.AgentDefinitionStatusActive,
		Permissions: []string{application.AgentPermissionConversationRead, application.AgentPermissionMessageWrite},
		Scopes: []application.AgentResourceScopeV1{{
			ResourceType: application.AgentResourceTypeConversation, ResourceID: application.AgentResourceWildcard,
			Actions: []string{application.AgentResourceActionRead, application.AgentResourceActionWrite},
		}},
		ValidFrom: now.Add(-time.Hour),
	}
	store := &agentPolicyStoreStub{
		latestByOwner: map[string]*application.AgentDefinitionVersionV1{"U100": &ownerDefinition},
		definitions: map[string]*application.AgentDefinitionVersionV1{
			definitionKeyV1(ownerDefinition.DefinitionUUID, ownerDefinition.Version): &ownerDefinition,
		},
	}
	admission, err := agentapplication.NewPersistentAgentRunAdmissionV1WithClock(store, func() time.Time { return now }, &ungrantedOwnerPromotionAuthorizer{})
	if err != nil {
		t.Fatalf("new Run admission: %v", err)
	}
	request := interactiveAdmissionRequest()
	request.TriggerType = "message.group.created"
	request.SubscriptionUUID = "SUB-1"

	if _, err := admission.Admit(context.Background(), request); !errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
		t.Fatalf("subscription with ungranted owner Definition error = %v, want denial", err)
	}
	if taskCount := len(store.tasks); taskCount != 0 {
		t.Fatalf("subscription must not create a task, got %d", taskCount)
	}
}

func TestAdmitDoesNotAutoEnrollSubscriptionTriggers(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 6, 8, 0, 0, 0, time.UTC)
	store := &agentPolicyStoreStub{}
	admission, err := agentapplication.NewPersistentAgentRunAdmissionV1WithClock(store, func() time.Time { return now }, &activeRunPromotionAuthorizerStub{})
	if err != nil {
		t.Fatalf("new Run admission: %v", err)
	}
	request := interactiveAdmissionRequest()
	request.TriggerType = "message.group.created"
	request.SubscriptionUUID = "SUB-1"

	if _, err := admission.Admit(context.Background(), request); !errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
		t.Fatalf("subscription trigger without owner Definition error = %v, want denial", err)
	}
	if _, created := store.definitions[definitionKeyV1(agentapplication.LowRiskAssistantDefinitionUUIDV1, 1)]; created {
		t.Fatalf("subscription trigger must not auto-enroll the low-risk Definition")
	}
}

func TestAutoApproveInteractiveReplyAllowsLowRiskAgentOwnedDefinition(t *testing.T) {
	now := time.Date(2026, 9, 6, 8, 0, 0, 0, time.UTC)
	// The shared low-risk Definition is owned by a platform identity, not the principal.
	definition := application.AgentDefinitionVersionV1{
		DefinitionUUID: agentapplication.LowRiskAssistantDefinitionUUIDV1, Version: 1,
		TenantID: "dipole", OwnerUUID: "UAI00000000000000LOW01", AgentUUID: "UAI", Status: application.AgentDefinitionStatusActive,
		Permissions: []string{application.AgentPermissionConversationRead, application.AgentPermissionMessageWrite},
		Scopes: []application.AgentResourceScopeV1{{
			ResourceType: application.AgentResourceTypeConversation, ResourceID: application.AgentResourceWildcard,
			Actions: []string{application.AgentResourceActionRead, application.AgentResourceActionWrite},
		}},
		ValidFrom: now.Add(-time.Hour),
	}
	store := &agentPolicyStoreStub{
		definitions: map[string]*application.AgentDefinitionVersionV1{
			definitionKeyV1(definition.DefinitionUUID, definition.Version): &definition,
		},
		tasks: map[string]*application.AgentTaskV1{"TASK-1": {
			TaskUUID: "TASK-1", DefinitionUUID: definition.DefinitionUUID, DefinitionVersion: definition.Version,
			TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", Status: application.AgentTaskStatusRunning,
			TriggerType: "agent.interactive.requested", TriggerRef: "interactive:abc",
		}},
		runs: map[string]*application.AgentRunV1{"RUN-1": {
			RunUUID: "RUN-1", TaskUUID: "TASK-1", RuntimeID: "dipole-agent", Mode: "active", Status: application.AgentRunStatusRunning,
		}},
	}
	scope := application.AgentResourceScopeV1{
		ResourceType: application.AgentResourceTypeConversation, ResourceID: model.DirectConversationKey("U100", "UAI"),
		Actions: []string{application.AgentResourceActionWrite},
	}
	scopeHash, _ := application.AgentResourceScopeSHA256V1(scope)
	request := application.AgentApprovalRequestV1{
		TaskUUID: "TASK-1", RunUUID: "RUN-1", RuntimeID: "dipole-agent", Mode: "active",
		Approval: application.AgentApprovalV1{
			ApprovalUUID: "approval:lowrisk-1", TaskUUID: "TASK-1", CapabilityID: application.AgentCapabilityAssistantReplySend,
			ResourceScope: scope, ScopeSHA256: scopeHash, ArgumentsSHA256: strings.Repeat("a", 64), NonceSHA256: strings.Repeat("b", 64),
			ExpiresAt: now.Add(30 * time.Minute),
		},
	}
	service, err := agentapplication.NewPersistentAgentApprovalServiceV1WithClock(store, func() time.Time { return now })
	if err != nil {
		t.Fatalf("new Approval service: %v", err)
	}
	grant, err := service.AutoApproveInteractiveReply(context.Background(), request)
	if err != nil {
		t.Fatalf("low-risk interactive reply approval: %v", err)
	}
	if grant.Status != application.AgentApprovalStatusApproved || grant.ApprovedByUUID != "U100" {
		t.Fatalf("low-risk grant = %+v", grant)
	}
}

// lowRiskPromotionStoreStub records the grant lookup and captures created grants.
type lowRiskPromotionStoreStub struct {
	lookup  application.AgentRuntimePromotionGrantLookupV1
	grant   *application.AgentRuntimePromotionGrantV1
	created *application.AgentRuntimePromotionGrantV1
}

func (s *lowRiskPromotionStoreStub) CreateRuntimePromotionGrant(_ context.Context, grant application.AgentRuntimePromotionGrantV1) (bool, error) {
	copy := grant
	s.created = &copy
	return true, nil
}

func (s *lowRiskPromotionStoreStub) GetActiveRuntimePromotionGrant(_ context.Context, lookup application.AgentRuntimePromotionGrantLookupV1) (*application.AgentRuntimePromotionGrantV1, error) {
	s.lookup = lookup
	return s.grant, nil
}

func (s *lowRiskPromotionStoreStub) RevokeRuntimePromotionGrant(context.Context, string, time.Time) (bool, error) {
	return false, nil
}

func lowRiskActivePromotionRequest(now time.Time) application.AgentActiveRunPromotionRequestV1 {
	definition := application.AgentDefinitionVersionV1{
		DefinitionUUID: agentapplication.LowRiskAssistantDefinitionUUIDV1, Version: 1,
		TenantID: "dipole", OwnerUUID: "UAI00000000000000LOW01", AgentUUID: "UAI", Status: application.AgentDefinitionStatusActive,
		Permissions: []string{application.AgentPermissionConversationRead, application.AgentPermissionMessageWrite},
		Scopes: []application.AgentResourceScopeV1{{
			ResourceType: application.AgentResourceTypeConversation, ResourceID: application.AgentResourceWildcard,
			Actions: []string{application.AgentResourceActionRead, application.AgentResourceActionWrite},
		}},
		ValidFrom: now.Add(-time.Hour),
	}
	return application.AgentActiveRunPromotionRequestV1{
		RuntimeID: "dipole-agent", CandidateVersion: "candidate-v1",
		Task: application.AgentTaskV1{
			TaskUUID: "TASK-1", DefinitionUUID: definition.DefinitionUUID, DefinitionVersion: definition.Version,
			TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", Status: application.AgentTaskStatusRunning,
			TriggerType: "agent.interactive.requested", TriggerRef: "interactive:abc",
		},
		Definition: definition,
	}
}

func TestActiveRunPromotionLooksUpPlatformGrantForLowRiskDefinition(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 6, 8, 0, 0, 0, time.UTC)
	store := &lowRiskPromotionStoreStub{grant: &application.AgentRuntimePromotionGrantV1{
		GrantUUID: agentapplication.LowRiskAssistantPromotionGrantUUIDV1, TenantID: "dipole", RuntimeID: "dipole-agent",
		CandidateVersion: "candidate-v1", DefinitionUUID: agentapplication.LowRiskAssistantDefinitionUUIDV1, DefinitionVersion: 1,
		PolicyVersion:  application.AgentRuntimePromotionPolicyVersionV2,
		EvidenceSHA256: strings.Repeat("0", 64), EvalSuiteSHA256: strings.Repeat("0", 64),
		GrantedByUUID: "UAI", ReviewedByUUID: "UREVIEWER",
		ValidFrom: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour),
	}}
	authorizer, err := agentapplication.NewPersistentAgentActiveRunPromotionAuthorizerV1WithClock(store, func() time.Time { return now })
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}
	if err := authorizer.AuthorizeActiveRun(context.Background(), lowRiskActivePromotionRequest(now)); err != nil {
		t.Fatalf("low-risk active promotion: %v", err)
	}
	// The lookup must target the shared Definition's own UUID (the grant's FK target).
	if store.lookup.DefinitionUUID != agentapplication.LowRiskAssistantDefinitionUUIDV1 {
		t.Fatalf("promotion lookup DefinitionUUID = %q, want shared Definition UUID", store.lookup.DefinitionUUID)
	}
}

func TestEnsureLowRiskAssistantPromotionGrantIsIdempotent(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 6, 8, 0, 0, 0, time.UTC)
	store := &lowRiskPromotionStoreStub{}
	policy := &agentPolicyStoreStub{}

	// Empty candidate version is a no-op (governed runtime not in use).
	if err := agentapplication.EnsureLowRiskAssistantPromotionGrantV1(context.Background(), policy, store, "dipole", "UAI", "", now); err != nil {
		t.Fatalf("empty candidate should be a no-op: %v", err)
	}
	if store.created != nil {
		t.Fatalf("empty candidate must not create a grant")
	}

	// First call provisions the shared Definition and the platform grant.
	if err := agentapplication.EnsureLowRiskAssistantPromotionGrantV1(context.Background(), policy, store, "dipole", "UAI", "candidate-v1", now); err != nil {
		t.Fatalf("provision grant: %v", err)
	}
	if _, ok := policy.definitions[definitionKeyV1(agentapplication.LowRiskAssistantDefinitionUUIDV1, 1)]; !ok {
		t.Fatalf("shared low-risk Definition was not ensured: %+v", policy.definitions)
	}
	if store.created == nil || store.created.GrantUUID != agentapplication.LowRiskAssistantPromotionGrantUUIDV1 ||
		store.created.DefinitionUUID != agentapplication.LowRiskAssistantDefinitionUUIDV1 || store.created.CandidateVersion != "candidate-v1" {
		t.Fatalf("created grant = %+v", store.created)
	}
	if store.created.GrantedByUUID == store.created.ReviewedByUUID {
		t.Fatalf("grant must separate grantor and reviewer")
	}

	// A second call with an already-active grant leaves it untouched.
	store.grant = store.created
	store.created = nil
	if err := agentapplication.EnsureLowRiskAssistantPromotionGrantV1(context.Background(), policy, store, "dipole", "UAI", "candidate-v1", now); err != nil {
		t.Fatalf("re-provision grant: %v", err)
	}
	if store.created != nil {
		t.Fatalf("existing active grant must not be recreated")
	}
}
