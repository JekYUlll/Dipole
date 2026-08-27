package app

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

type agentPolicyStoreStub struct {
	latest      *application.AgentDefinitionVersionV1
	definitions map[string]*application.AgentDefinitionVersionV1
	tasks       map[string]*application.AgentTaskV1
	afterCreate func()
}

func (s *agentPolicyStoreStub) CreateDefinitionVersion(_ context.Context, definition application.AgentDefinitionVersionV1) error {
	copy := definition
	s.latest = &copy
	if s.definitions == nil {
		s.definitions = map[string]*application.AgentDefinitionVersionV1{}
	}
	s.definitions[definitionKeyV1(definition.DefinitionUUID, definition.Version)] = &copy
	return nil
}

func (s *agentPolicyStoreStub) GetLatestDefinition(context.Context, string, string) (*application.AgentDefinitionVersionV1, error) {
	return cloneDefinitionV1(s.latest), nil
}

func (s *agentPolicyStoreStub) GetDefinitionVersion(_ context.Context, uuid string, version uint64) (*application.AgentDefinitionVersionV1, error) {
	return cloneDefinitionV1(s.definitions[definitionKeyV1(uuid, version)]), nil
}

func (*agentPolicyStoreStub) RevokeDefinitionVersion(context.Context, string, uint64, time.Time) error {
	return nil
}

func (s *agentPolicyStoreStub) CreateTask(_ context.Context, task application.AgentTaskV1) (bool, error) {
	if s.tasks == nil {
		s.tasks = map[string]*application.AgentTaskV1{}
	}
	if _, exists := s.tasks[task.TaskUUID]; exists {
		return false, nil
	}
	copy := task
	s.tasks[task.TaskUUID] = &copy
	if s.afterCreate != nil {
		s.afterCreate()
	}
	return true, nil
}

func (s *agentPolicyStoreStub) GetTask(_ context.Context, uuid string) (*application.AgentTaskV1, error) {
	task := s.tasks[uuid]
	if task == nil {
		return nil, nil
	}
	copy := *task
	return &copy, nil
}

func (s *agentPolicyStoreStub) TransitionTaskStatus(_ context.Context, uuid string, from, to application.AgentTaskStatusV1) (bool, error) {
	if err := application.ValidateAgentTaskTransitionV1(from, to); err != nil {
		return false, err
	}
	task := s.tasks[uuid]
	if task == nil || task.Status != from {
		return false, nil
	}
	task.Status = to
	return true, nil
}

func (*agentPolicyStoreStub) CreateApproval(context.Context, application.AgentApprovalV1) error {
	return nil
}
func (*agentPolicyStoreStub) ApproveApproval(context.Context, string, string, time.Time) (bool, error) {
	return false, nil
}
func (*agentPolicyStoreStub) ConsumeApproval(context.Context, string, application.AgentApprovalClaimV1, time.Time) (bool, error) {
	return false, nil
}
func (*agentPolicyStoreStub) RevokeApproval(context.Context, string, time.Time) error { return nil }

func TestPersistentAgentExecutionPolicyPinsDefinitionAndTransitionsTask(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	v1 := activeAgentDefinitionV1(1, now.Add(-time.Hour), []string{application.AgentPermissionConversationRead})
	v2 := activeAgentDefinitionV1(2, now.Add(-time.Minute), []string{application.AgentPermissionMessageWrite})
	store := &agentPolicyStoreStub{
		latest:      &v1,
		definitions: map[string]*application.AgentDefinitionVersionV1{definitionKeyV1(v1.DefinitionUUID, v1.Version): &v1},
	}
	store.afterCreate = func() {
		store.latest = &v2
		store.definitions[definitionKeyV1(v2.DefinitionUUID, v2.Version)] = &v2
	}
	policy, err := newPersistentAgentExecutionPolicyV1(store, func() time.Time { return now })
	if err != nil {
		t.Fatalf("new persistent policy: %v", err)
	}
	request := agentPolicyStartRequestV1()
	execution, err := policy.Start(context.Background(), request)
	if err != nil {
		t.Fatalf("start policy: %v", err)
	}
	if len(execution.TaskUUID) != 64 || execution.Invocation.Permissions[0] != application.AgentPermissionConversationRead {
		t.Fatalf("execution did not use pinned v1: %+v", execution)
	}
	task := store.tasks[execution.TaskUUID]
	if task.DefinitionVersion != 1 || task.Status != application.AgentTaskStatusRunning || task.PrincipalUUID != request.PrincipalUUID {
		t.Fatalf("unexpected pinned Task: %+v", task)
	}
	if err := policy.Complete(context.Background(), *execution); err != nil || task.Status != application.AgentTaskStatusCompleted {
		t.Fatalf("complete Task: status=%s err=%v", task.Status, err)
	}
}

func TestAgentTaskUUIDV1MatchesLanguageNeutralGoldenVector(t *testing.T) {
	t.Parallel()

	got := agentTaskUUIDV1(agentPolicyStartRequestV1())
	const want = "task:e47647aaf491da8a27072ed94d6b69b87a025a1e211000cbef6a9aeb458"
	if got != want {
		t.Fatalf("Agent Task UUID = %q, want %q", got, want)
	}
}

func TestPersistentAgentExecutionPolicyRejectsRevokedExpiredAndDuplicateRuns(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		definition application.AgentDefinitionVersionV1
	}{
		{name: "revoked", definition: func() application.AgentDefinitionVersionV1 {
			d := activeAgentDefinitionV1(1, now.Add(-time.Hour), []string{application.AgentPermissionConversationRead})
			d.Status, d.RevokedAt = application.AgentDefinitionStatusRevoked, &now
			return d
		}()},
		{name: "expired", definition: func() application.AgentDefinitionVersionV1 {
			d := activeAgentDefinitionV1(1, now.Add(-time.Hour), []string{application.AgentPermissionConversationRead})
			expired := now
			d.ExpiresAt = &expired
			return d
		}()},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			store := policyStoreWithDefinitionV1(test.definition)
			policy, _ := newPersistentAgentExecutionPolicyV1(store, func() time.Time { return now })
			if _, err := policy.Start(context.Background(), agentPolicyStartRequestV1()); !errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
				t.Fatalf("expected policy denial, got %v", err)
			}
		})
	}

	definition := activeAgentDefinitionV1(1, now.Add(-time.Hour), []string{application.AgentPermissionConversationRead})
	store := policyStoreWithDefinitionV1(definition)
	policy, _ := newPersistentAgentExecutionPolicyV1(store, func() time.Time { return now })
	if _, err := policy.Start(context.Background(), agentPolicyStartRequestV1()); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if _, err := policy.Start(context.Background(), agentPolicyStartRequestV1()); !errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
		t.Fatalf("duplicate run should be denied, got %v", err)
	}
}

func TestEnsureEmbeddedAgentDefinitionV1PreservesExistingDefinition(t *testing.T) {
	t.Parallel()

	scopes := []application.AgentResourceScopeV1{{ResourceType: application.AgentResourceTypeConversation, ResourceID: application.AgentResourceWildcard, Actions: []string{application.AgentResourceActionRead}}}
	store := &agentPolicyStoreStub{}
	if err := EnsureEmbeddedAgentDefinitionV1(context.Background(), store, "dipole", "UAI", []string{application.AgentPermissionConversationRead}, scopes); err != nil {
		t.Fatalf("ensure baseline: %v", err)
	}
	if store.latest == nil || store.latest.DefinitionUUID != "embedded:UAI" || store.latest.Version != 1 {
		t.Fatalf("unexpected baseline: %+v", store.latest)
	}
	custom := activeAgentDefinitionV1(7, time.Unix(0, 0).UTC(), []string{application.AgentPermissionMessageWrite})
	store.latest = &custom
	if err := EnsureEmbeddedAgentDefinitionV1(context.Background(), store, "dipole", "UAI", []string{application.AgentPermissionConversationRead}, scopes); err != nil {
		t.Fatalf("preserve custom Definition: %v", err)
	}
	if store.latest.Version != 7 {
		t.Fatalf("custom Definition was overwritten: %+v", store.latest)
	}
}

func activeAgentDefinitionV1(version uint64, validFrom time.Time, permissions []string) application.AgentDefinitionVersionV1 {
	return application.AgentDefinitionVersionV1{
		DefinitionUUID: "DEF-UAI", Version: version, TenantID: "dipole", OwnerUUID: "UAI", AgentUUID: "UAI",
		Status: application.AgentDefinitionStatusActive, Permissions: permissions,
		Scopes:    []application.AgentResourceScopeV1{{ResourceType: application.AgentResourceTypeConversation, ResourceID: application.AgentResourceWildcard, Actions: []string{application.AgentResourceActionRead}}},
		ValidFrom: validFrom,
	}
}

func policyStoreWithDefinitionV1(definition application.AgentDefinitionVersionV1) *agentPolicyStoreStub {
	return &agentPolicyStoreStub{
		latest:      &definition,
		definitions: map[string]*application.AgentDefinitionVersionV1{definitionKeyV1(definition.DefinitionUUID, definition.Version): &definition},
	}
}

func agentPolicyStartRequestV1() application.AgentExecutionPolicyStartV1 {
	return application.AgentExecutionPolicyStartV1{
		TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", DelegatedByUUID: "U100",
		TriggerType: "message.direct.created", TriggerRef: "M100", RequestID: "R1", TraceID: "T1", EventID: "E1",
	}
}

func definitionKeyV1(uuid string, version uint64) string {
	return uuid + ":" + strconv.FormatUint(version, 10)
}

func cloneDefinitionV1(definition *application.AgentDefinitionVersionV1) *application.AgentDefinitionVersionV1 {
	if definition == nil {
		return nil
	}
	copy := *definition
	copy.Permissions = append([]string(nil), definition.Permissions...)
	copy.Scopes = clonePolicyScopesV1(definition.Scopes)
	return &copy
}
