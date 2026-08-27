package app

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

type agentPolicyStoreStub struct {
	latest      *application.AgentDefinitionVersionV1
	definitions map[string]*application.AgentDefinitionVersionV1
	tasks       map[string]*application.AgentTaskV1
	runs        map[string]*application.AgentRunV1
	approvals   map[string]*application.AgentApprovalV1
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

func (s *agentPolicyStoreStub) CreateRun(_ context.Context, run application.AgentRunV1) (bool, error) {
	if s.runs == nil {
		s.runs = map[string]*application.AgentRunV1{}
	}
	if _, exists := s.runs[run.RunUUID]; exists {
		return false, nil
	}
	copy := run
	s.runs[run.RunUUID] = &copy
	return true, nil
}

func (s *agentPolicyStoreStub) GetRun(_ context.Context, uuid string) (*application.AgentRunV1, error) {
	run := s.runs[uuid]
	if run == nil {
		return nil, nil
	}
	copy := *run
	return &copy, nil
}

func (s *agentPolicyStoreStub) TransitionRunStatus(_ context.Context, uuid string, from, to application.AgentRunStatusV1, lastError string) (bool, error) {
	run := s.runs[uuid]
	if run == nil || run.Status != from {
		return false, nil
	}
	run.Status, run.LastError = to, lastError
	return true, nil
}

func (s *agentPolicyStoreStub) CreateApproval(_ context.Context, approval application.AgentApprovalV1) error {
	if s.approvals == nil {
		s.approvals = map[string]*application.AgentApprovalV1{}
	}
	copy := approval
	s.approvals[approval.ApprovalUUID] = &copy
	return nil
}
func (s *agentPolicyStoreStub) GetApproval(_ context.Context, uuid string) (*application.AgentApprovalV1, error) {
	if s.approvals[uuid] == nil {
		return nil, nil
	}
	copy := *s.approvals[uuid]
	return &copy, nil
}
func (s *agentPolicyStoreStub) ApproveApproval(_ context.Context, uuid, actor string, _ time.Time) (bool, error) {
	approval := s.approvals[uuid]
	if approval == nil || approval.Status != application.AgentApprovalStatusPending {
		return false, nil
	}
	approval.Status, approval.ApprovedByUUID = application.AgentApprovalStatusApproved, actor
	return true, nil
}
func (*agentPolicyStoreStub) ConsumeApproval(context.Context, string, application.AgentApprovalClaimV1, time.Time) (bool, error) {
	return false, nil
}
func (s *agentPolicyStoreStub) RevokeApproval(_ context.Context, uuid string, at time.Time) error {
	approval := s.approvals[uuid]
	if approval != nil && approval.Status == application.AgentApprovalStatusPending {
		approval.Status, approval.RevokedAt = application.AgentApprovalStatusRevoked, &at
	}
	return nil
}
func (s *agentPolicyStoreStub) DenyApproval(_ context.Context, uuid string, at time.Time) (bool, error) {
	approval := s.approvals[uuid]
	if approval == nil || approval.Status != application.AgentApprovalStatusPending {
		return false, nil
	}
	approval.Status, approval.RevokedAt = application.AgentApprovalStatusRevoked, &at
	return true, nil
}

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
	if err := policy.Complete(context.Background(), *execution); err != nil {
		t.Fatalf("replay completed Task: %v", err)
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

func TestAgentRunUUIDV1MatchesLanguageNeutralGoldenVector(t *testing.T) {
	t.Parallel()

	got, err := application.AgentRunUUIDV1("task:e47647aaf491da8a27072ed94d6b69b87a025a1e211000cbef6a9aeb458", "dipole-agent", "shadow")
	if err != nil {
		t.Fatalf("derive Agent Run UUID: %v", err)
	}
	const want = "run:fe813966ff90ac9c0a32e5d7b6a55dadbba657f436ad3a3765e9466aba0b"
	if got != want {
		t.Fatalf("Agent Run UUID = %q, want %q", got, want)
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

func TestPersistentAgentInvocationResolverUsesPinnedTaskIdentity(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	definition := activeAgentDefinitionV1(3, now.Add(-time.Hour), []string{application.AgentPermissionConversationList})
	store := policyStoreWithDefinitionV1(definition)
	store.tasks = map[string]*application.AgentTaskV1{
		"TASK-1": {
			TaskUUID: "TASK-1", DefinitionUUID: definition.DefinitionUUID, DefinitionVersion: definition.Version,
			TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", Status: application.AgentTaskStatusRunning,
			TriggerType: "message.direct.created", TriggerRef: "M100",
		},
	}
	store.runs = map[string]*application.AgentRunV1{
		"RUN-1": {RunUUID: "RUN-1", TaskUUID: "TASK-1", RuntimeID: "dipole-agent", Mode: "shadow", Status: application.AgentRunStatusRunning},
	}
	resolver := &PersistentAgentInvocationResolverV1{store: store, now: func() time.Time { return now }}

	invocation, err := resolver.Resolve(context.Background(), "TASK-1", "RUN-1")
	if err != nil {
		t.Fatalf("resolve Invocation: %v", err)
	}
	if invocation.PrincipalUUID != "U100" || invocation.AgentUUID != "UAI" || len(invocation.Permissions) != 1 || invocation.Permissions[0] != application.AgentPermissionConversationList {
		t.Fatalf("unexpected pinned Invocation: %+v", invocation)
	}
	store.tasks["TASK-1"].Status = application.AgentTaskStatusCompleted
	if _, err := resolver.Resolve(context.Background(), "TASK-1", "RUN-1"); err != nil {
		t.Fatalf("completed Task with running shadow Run should resolve: %v", err)
	}
	store.runs["RUN-1"].Status = application.AgentRunStatusCompleted
	if _, err := resolver.Resolve(context.Background(), "TASK-1", "RUN-1"); !errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
		t.Fatalf("completed Run should be denied, got %v", err)
	}
}

func TestPersistentAgentRunAdmissionCreatesAndReplaysShadowRun(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	definition := activeAgentDefinitionV1(1, now.Add(-time.Hour), []string{application.AgentPermissionConversationList})
	store := policyStoreWithDefinitionV1(definition)
	admission := &PersistentAgentRunAdmissionV1{store: store, now: func() time.Time { return now }}
	request := application.AgentRunAdmissionRequestV1{
		AgentExecutionPolicyStartV1: agentPolicyStartRequestV1(), RuntimeID: "dipole-agent", Mode: "shadow",
	}

	first, err := admission.Admit(context.Background(), request)
	if err != nil {
		t.Fatalf("admit first shadow Run: %v", err)
	}
	definitionV2 := definition
	definitionV2.Version = 2
	definitionV2.Permissions = []string{application.AgentPermissionMessageWrite}
	store.latest = &definitionV2
	store.definitions[definitionKeyV1(definitionV2.DefinitionUUID, definitionV2.Version)] = &definitionV2
	second, err := admission.Admit(context.Background(), request)
	if err != nil {
		t.Fatalf("replay shadow Run admission: %v", err)
	}
	if first.TaskUUID != second.TaskUUID || first.RunUUID != second.RunUUID || store.tasks[first.TaskUUID].Status != application.AgentTaskStatusRunning || store.runs[first.RunUUID].Status != application.AgentRunStatusRunning {
		t.Fatalf("admission did not converge: first=%+v second=%+v", first, second)
	}
	if second.Invocation.Permissions[0] != application.AgentPermissionConversationList {
		t.Fatalf("replay drifted from pinned Definition: %+v", second.Invocation)
	}
	store.tasks[first.TaskUUID].Status = application.AgentTaskStatusCompleted
	if _, err := admission.Admit(context.Background(), request); err != nil {
		t.Fatalf("completed authoritative Task should retain the running shadow Run: %v", err)
	}
	if err := admission.Complete(context.Background(), first.TaskUUID, first.RunUUID, "dipole-agent", "shadow"); err != nil {
		t.Fatalf("complete shadow Run: %v", err)
	}
	if err := admission.Complete(context.Background(), first.TaskUUID, first.RunUUID, "dipole-agent", "shadow"); err != nil {
		t.Fatalf("replay shadow Run completion: %v", err)
	}
	if store.runs[first.RunUUID].Status != application.AgentRunStatusCompleted {
		t.Fatalf("shadow Run status = %s, want completed", store.runs[first.RunUUID].Status)
	}
	replayed, err := admission.Admit(context.Background(), request)
	if err != nil || replayed.RunStatus != application.AgentRunStatusCompleted {
		t.Fatalf("completed shadow Run admission should converge: replayed=%+v err=%v", replayed, err)
	}
	if err := admission.Complete(context.Background(), first.TaskUUID, first.RunUUID, "forged-runtime", "shadow"); !errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
		t.Fatalf("forged Runtime completion should be denied, got %v", err)
	}
}

func TestPersistentAgentRunAdmissionFinishesExactTerminalStatusIdempotently(t *testing.T) {
	t.Parallel()

	for _, terminal := range []application.AgentRunStatusV1{
		application.AgentRunStatusCompleted,
		application.AgentRunStatusFailed,
		application.AgentRunStatusCancelled,
	} {
		terminal := terminal
		t.Run(string(terminal), func(t *testing.T) {
			definition := activeAgentDefinitionV1(1, time.Now().Add(-time.Hour), []string{application.AgentPermissionConversationList})
			store := policyStoreWithDefinitionV1(definition)
			admission := &PersistentAgentRunAdmissionV1{store: store, now: time.Now}
			run, err := admission.Admit(context.Background(), application.AgentRunAdmissionRequestV1{
				AgentExecutionPolicyStartV1: agentPolicyStartRequestV1(), RuntimeID: "dipole-agent", Mode: "shadow",
			})
			if err != nil {
				t.Fatalf("admit Run: %v", err)
			}
			lastError := ""
			if terminal == application.AgentRunStatusFailed {
				lastError = "Activity retries exhausted"
			}
			if err := admission.Finish(context.Background(), run.TaskUUID, run.RunUUID, "dipole-agent", "shadow", terminal, lastError); err != nil {
				t.Fatalf("finish Run: %v", err)
			}
			if err := admission.Finish(context.Background(), run.TaskUUID, run.RunUUID, "dipole-agent", "shadow", terminal, lastError); err != nil {
				t.Fatalf("replay terminal Run: %v", err)
			}
			if store.runs[run.RunUUID].Status != terminal || store.runs[run.RunUUID].LastError != lastError {
				t.Fatalf("terminal Run = %+v, want status=%s error=%q", store.runs[run.RunUUID], terminal, lastError)
			}
			if err := admission.Finish(context.Background(), run.TaskUUID, run.RunUUID, "dipole-agent", "shadow", application.AgentRunStatusCompleted, ""); terminal != application.AgentRunStatusCompleted && !errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
				t.Fatalf("conflicting terminal replay should be denied, got %v", err)
			}
		})
	}
}

func TestPersistentAgentRunAdmissionRejectsInvalidTerminalEvidence(t *testing.T) {
	t.Parallel()

	admission := &PersistentAgentRunAdmissionV1{store: &agentPolicyStoreStub{}, now: time.Now}
	for _, test := range []struct {
		status    application.AgentRunStatusV1
		lastError string
	}{
		{status: application.AgentRunStatusRunning},
		{status: application.AgentRunStatusFailed},
		{status: application.AgentRunStatusCompleted, lastError: "unexpected"},
		{status: application.AgentRunStatusCancelled, lastError: strings.Repeat("x", 1025)},
	} {
		if err := admission.Finish(context.Background(), "TASK-1", "RUN-1", "dipole-agent", "shadow", test.status, test.lastError); !errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
			t.Fatalf("Finish(%s, %q) error = %v, want policy denied", test.status, test.lastError, err)
		}
	}
}

func TestEmbeddedExecutionAdoptsTaskCreatedByShadowAdmission(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	definition := activeAgentDefinitionV1(1, now.Add(-time.Hour), []string{application.AgentPermissionConversationRead})
	store := policyStoreWithDefinitionV1(definition)
	admission := &PersistentAgentRunAdmissionV1{store: store, now: func() time.Time { return now }}
	request := agentPolicyStartRequestV1()
	shadow, err := admission.Admit(context.Background(), application.AgentRunAdmissionRequestV1{
		AgentExecutionPolicyStartV1: request, RuntimeID: "dipole-agent", Mode: "shadow",
	})
	if err != nil {
		t.Fatalf("admit shadow Run: %v", err)
	}
	policy, _ := newPersistentAgentExecutionPolicyV1(store, func() time.Time { return now })
	embedded, err := policy.Start(context.Background(), request)
	if err != nil {
		t.Fatalf("start Embedded Run from shared Task: %v", err)
	}
	if embedded.TaskUUID != shadow.TaskUUID || embedded.RunUUID == shadow.RunUUID || len(store.runs) != 2 {
		t.Fatalf("Task/Run separation did not converge: shadow=%+v embedded=%+v runs=%+v", shadow, embedded, store.runs)
	}
	if _, err := policy.Start(context.Background(), request); !errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
		t.Fatalf("duplicate Embedded Run should be denied, got %v", err)
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
