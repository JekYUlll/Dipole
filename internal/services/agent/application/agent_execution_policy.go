package agentapplication

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

const embeddedAgentDefinitionVersionV1 uint64 = 1
const embeddedAgentRuntimeIDV1 = "dipole-eino"

type agentPolicyClockV1 func() time.Time

type StaticAgentExecutionPolicyV1 struct {
	permissions []string
	scopes      []application.AgentResourceScopeV1
}

type PersistentAgentExecutionPolicyV1 struct {
	store application.AgentPolicyStoreV1
	now   agentPolicyClockV1
}

type PersistentAgentInvocationResolverV1 struct {
	store            application.AgentPolicyStoreV1
	now              agentPolicyClockV1
	activeAuthorizer application.AgentActiveRunPromotionAuthorizerV1
}

type PersistentAgentRunAdmissionV1 struct {
	store            application.AgentPolicyStoreV1
	now              agentPolicyClockV1
	activeAuthorizer application.AgentActiveRunPromotionAuthorizerV1
}

type agentEventSubscriptionReaderV1 interface {
	GetEventSubscription(ctx context.Context, subscriptionUUID string) (*application.AgentEventSubscriptionV1, error)
}

var _ application.AgentInvocationResolverV1 = (*PersistentAgentInvocationResolverV1)(nil)

func NewPersistentAgentInvocationResolverV1(store application.AgentPolicyStoreV1, activeAuthorizers ...application.AgentActiveRunPromotionAuthorizerV1) (*PersistentAgentInvocationResolverV1, error) {
	return NewPersistentAgentInvocationResolverV1WithClock(store, time.Now, activeAuthorizers...)
}

func NewPersistentAgentInvocationResolverV1WithClock(store application.AgentPolicyStoreV1, now func() time.Time, activeAuthorizers ...application.AgentActiveRunPromotionAuthorizerV1) (*PersistentAgentInvocationResolverV1, error) {
	if store == nil {
		return nil, fmt.Errorf("persistent Agent Invocation resolver requires store")
	}
	if len(activeAuthorizers) > 1 {
		return nil, fmt.Errorf("persistent Agent Invocation resolver accepts at most one active promotion authorizer")
	}
	if now == nil {
		return nil, fmt.Errorf("persistent Agent Invocation resolver requires clock")
	}
	resolver := &PersistentAgentInvocationResolverV1{store: store, now: now}
	if len(activeAuthorizers) == 1 {
		resolver.activeAuthorizer = activeAuthorizers[0]
	}
	return resolver, nil
}

func (r *PersistentAgentInvocationResolverV1) Resolve(ctx context.Context, taskUUID, runUUID string) (application.AgentInvocationV1, error) {
	taskUUID, runUUID = strings.TrimSpace(taskUUID), strings.TrimSpace(runUUID)
	if taskUUID == "" || runUUID == "" {
		return application.AgentInvocationV1{}, fmt.Errorf("%w: Agent Task UUID is required", application.ErrAgentExecutionPolicyDenied)
	}
	task, err := r.store.GetTask(ctx, taskUUID)
	if err != nil {
		return application.AgentInvocationV1{}, fmt.Errorf("get Agent Task Invocation: %w", err)
	}
	if task == nil || (task.Status != application.AgentTaskStatusRunning && task.Status != application.AgentTaskStatusCompleted) {
		return application.AgentInvocationV1{}, fmt.Errorf("%w: Agent Task is missing or outside executable state", application.ErrAgentExecutionPolicyDenied)
	}
	run, err := r.store.GetRun(ctx, runUUID)
	if err != nil {
		return application.AgentInvocationV1{}, fmt.Errorf("get Agent Run Invocation: %w", err)
	}
	if run == nil || run.TaskUUID != taskUUID || run.Status != application.AgentRunStatusRunning {
		return application.AgentInvocationV1{}, fmt.Errorf("%w: Agent Run is missing, mismatched, or not running", application.ErrAgentExecutionPolicyDenied)
	}
	definition, err := r.store.GetDefinitionVersion(ctx, task.DefinitionUUID, task.DefinitionVersion)
	if err != nil {
		return application.AgentInvocationV1{}, fmt.Errorf("get pinned Agent Invocation policy: %w", err)
	}
	request := application.AgentExecutionPolicyStartV1{
		TenantID: task.TenantID, PrincipalUUID: task.PrincipalUUID, AgentUUID: task.AgentUUID,
		DelegatedByUUID: task.PrincipalUUID, TriggerType: task.TriggerType, TriggerRef: task.TriggerRef,
	}
	if err := authorizeDefinitionAtV1(definition, request, r.now()); err != nil {
		return application.AgentInvocationV1{}, err
	}
	var approvedCapabilities []string
	if run.Mode == "active" {
		if r.activeAuthorizer == nil || strings.TrimSpace(run.CandidateVersion) == "" {
			return application.AgentInvocationV1{}, fmt.Errorf("%w: active Runtime promotion authorization is unavailable", application.ErrAgentExecutionPolicyDenied)
		}
		if err := authorizeActiveRunPromotionV1(ctx, r.activeAuthorizer, application.AgentActiveRunPromotionRequestV1{
			RuntimeID: run.RuntimeID, CandidateVersion: run.CandidateVersion, Task: *task, Definition: *definition,
		}); err != nil {
			return application.AgentInvocationV1{}, err
		}
		approvedCapabilities, err = application.ProjectAgentApprovedCapabilitiesV1(*definition)
		if err != nil {
			return application.AgentInvocationV1{}, err
		}
	}
	invocation := invocationFromPolicyStartV1(request, definition.Permissions, definition.Scopes)
	invocation.RuntimeID, invocation.Mode = run.RuntimeID, run.Mode
	invocation.ApprovedCapabilities = approvedCapabilities
	return invocation, nil
}

var _ application.AgentRunAdmissionServiceV1 = (*PersistentAgentRunAdmissionV1)(nil)

func NewPersistentAgentRunAdmissionV1(store application.AgentPolicyStoreV1, activeAuthorizers ...application.AgentActiveRunPromotionAuthorizerV1) (*PersistentAgentRunAdmissionV1, error) {
	return NewPersistentAgentRunAdmissionV1WithClock(store, time.Now, activeAuthorizers...)
}

func NewPersistentAgentRunAdmissionV1WithClock(store application.AgentPolicyStoreV1, now func() time.Time, activeAuthorizers ...application.AgentActiveRunPromotionAuthorizerV1) (*PersistentAgentRunAdmissionV1, error) {
	if store == nil {
		return nil, fmt.Errorf("persistent Agent Run admission requires store")
	}
	if len(activeAuthorizers) > 1 {
		return nil, fmt.Errorf("persistent Agent Run admission accepts at most one active promotion authorizer")
	}
	if now == nil {
		return nil, fmt.Errorf("persistent Agent Run admission requires clock")
	}
	admission := &PersistentAgentRunAdmissionV1{store: store, now: now}
	if len(activeAuthorizers) == 1 {
		admission.activeAuthorizer = activeAuthorizers[0]
	}
	return admission, nil
}

func (a *PersistentAgentRunAdmissionV1) Admit(ctx context.Context, admission application.AgentRunAdmissionRequestV1) (*application.AgentRunAdmissionV1, error) {
	request := admission.AgentExecutionPolicyStartV1
	if err := validateAgentExecutionPolicyStartV1(request); err != nil {
		return nil, err
	}
	if strings.TrimSpace(admission.RuntimeID) == "" || (admission.Mode != "shadow" && admission.Mode != "active") {
		return nil, fmt.Errorf("%w: Runtime identity and remote mode are required", application.ErrAgentExecutionPolicyDenied)
	}
	if admission.Mode == "active" && a.activeAuthorizer == nil {
		return nil, fmt.Errorf("%w: active Runtime promotion authorization is unavailable", application.ErrAgentExecutionPolicyDenied)
	}
	if admission.Mode == "active" && strings.TrimSpace(admission.CandidateVersion) == "" {
		return nil, fmt.Errorf("%w: active Runtime candidate version is required", application.ErrAgentExecutionPolicyDenied)
	}
	taskUUID := agentTaskUUIDV1(request)
	existingTask, err := a.store.GetTask(ctx, taskUUID)
	if err != nil {
		return nil, fmt.Errorf("lookup Agent Task admission: %w", err)
	}
	var task application.AgentTaskV1
	activeAuthorized := false
	if existingTask == nil {
		latest, lookupErr := a.store.GetLatestDefinition(ctx, request.TenantID, request.AgentUUID)
		if lookupErr != nil || authorizeDefinitionAtV1(latest, request, a.now()) != nil {
			return nil, fmt.Errorf("%w: Agent Definition unavailable", application.ErrAgentExecutionPolicyDenied)
		}
		if err := authorizeTriggerSubscriptionV1(ctx, a.store, request, latest, a.now()); err != nil {
			return nil, err
		}
		task = application.AgentTaskV1{
			TaskUUID: taskUUID, DefinitionUUID: latest.DefinitionUUID, DefinitionVersion: latest.Version,
			TenantID: request.TenantID, PrincipalUUID: request.PrincipalUUID, AgentUUID: request.AgentUUID,
			Status: application.AgentTaskStatusCreated, TriggerType: request.TriggerType, TriggerRef: request.TriggerRef, Goal: "handle_agent_trigger",
			TriggerSubscriptionUUID: strings.TrimSpace(request.SubscriptionUUID),
		}
		if err := a.authorizeActiveRunV1(ctx, admission, task, *latest); err != nil {
			return nil, err
		}
		activeAuthorized = admission.Mode == "active"
		created, createErr := a.store.CreateTask(ctx, task)
		if createErr != nil {
			return nil, fmt.Errorf("admit Agent Task: %w", createErr)
		}
		if !created {
			activeAuthorized = false
			existingTask, err = a.store.GetTask(ctx, taskUUID)
			if err != nil || existingTask == nil {
				return nil, fmt.Errorf("%w: concurrent Agent Task admission unavailable", application.ErrAgentExecutionPolicyDenied)
			}
			task = *existingTask
		}
	} else {
		task = *existingTask
	}
	if !agentTaskMatchesStartV1(task, request) {
		return nil, fmt.Errorf("%w: existing Agent Task cannot admit Run", application.ErrAgentExecutionPolicyDenied)
	}
	if task.Status != application.AgentTaskStatusCreated && task.Status != application.AgentTaskStatusRunning && task.Status != application.AgentTaskStatusCompleted {
		return nil, fmt.Errorf("%w: existing Agent Task cannot admit Run", application.ErrAgentExecutionPolicyDenied)
	}
	definition, err := a.store.GetDefinitionVersion(ctx, task.DefinitionUUID, task.DefinitionVersion)
	if err != nil || authorizeDefinitionAtV1(definition, request, a.now()) != nil {
		return nil, fmt.Errorf("%w: pinned Agent Definition unavailable", application.ErrAgentExecutionPolicyDenied)
	}
	if !activeAuthorized {
		if err := a.authorizeActiveRunV1(ctx, admission, task, *definition); err != nil {
			return nil, err
		}
	}
	var approvedCapabilities []string
	if admission.Mode == "active" {
		approvedCapabilities, err = application.ProjectAgentApprovedCapabilitiesV1(*definition)
		if err != nil {
			return nil, err
		}
	}
	if task.Status == application.AgentTaskStatusCreated {
		changed, transitionErr := a.store.TransitionTaskStatus(ctx, task.TaskUUID, application.AgentTaskStatusCreated, application.AgentTaskStatusRunning)
		if transitionErr != nil {
			return nil, fmt.Errorf("Agent Task admission transition: %w", transitionErr)
		}
		if changed {
			task.Status = application.AgentTaskStatusRunning
		} else {
			current, lookupErr := a.store.GetTask(ctx, task.TaskUUID)
			if lookupErr != nil || current == nil {
				return nil, fmt.Errorf("%w: concurrent Agent Task transition unavailable", application.ErrAgentExecutionPolicyDenied)
			}
			task = *current
		}
	}
	runUUID, err := application.AgentRunUUIDV1(task.TaskUUID, admission.RuntimeID, admission.Mode)
	if err != nil {
		return nil, err
	}
	createdRun, err := a.store.CreateRun(ctx, application.AgentRunV1{
		RunUUID: runUUID, TaskUUID: task.TaskUUID, RuntimeID: admission.RuntimeID, CandidateVersion: strings.TrimSpace(admission.CandidateVersion),
		TraceID: strings.TrimSpace(request.TraceID), Mode: admission.Mode, Status: application.AgentRunStatusRunning,
	})
	if err != nil {
		return nil, fmt.Errorf("admit Agent Run: %w", err)
	}
	runStatus := application.AgentRunStatusRunning
	if !createdRun {
		existingRun, lookupErr := a.store.GetRun(ctx, runUUID)
		if lookupErr != nil || existingRun == nil ||
			existingRun.CandidateVersion != strings.TrimSpace(admission.CandidateVersion) || existingRun.TraceID != strings.TrimSpace(request.TraceID) ||
			(existingRun.Status != application.AgentRunStatusRunning && existingRun.Status != application.AgentRunStatusCompleted) {
			return nil, fmt.Errorf("%w: existing Agent Run is terminal", application.ErrAgentExecutionPolicyDenied)
		}
		runStatus = existingRun.Status
	}
	invocation := invocationFromPolicyStartV1(request, definition.Permissions, definition.Scopes)
	invocation.RuntimeID, invocation.Mode = admission.RuntimeID, admission.Mode
	invocation.ApprovedCapabilities = approvedCapabilities
	return &application.AgentRunAdmissionV1{
		TaskUUID: task.TaskUUID, RunUUID: runUUID, RunStatus: runStatus,
		Invocation: invocation,
	}, nil
}

func (a *PersistentAgentRunAdmissionV1) authorizeActiveRunV1(ctx context.Context, admission application.AgentRunAdmissionRequestV1, task application.AgentTaskV1, definition application.AgentDefinitionVersionV1) error {
	if admission.Mode != "active" {
		return nil
	}
	return authorizeActiveRunPromotionV1(ctx, a.activeAuthorizer, application.AgentActiveRunPromotionRequestV1{
		RuntimeID: admission.RuntimeID, CandidateVersion: strings.TrimSpace(admission.CandidateVersion), Task: task, Definition: definition,
	})
}

func authorizeActiveRunPromotionV1(ctx context.Context, authorizer application.AgentActiveRunPromotionAuthorizerV1, request application.AgentActiveRunPromotionRequestV1) error {
	if err := authorizer.AuthorizeActiveRun(ctx, request); err != nil {
		if errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
			return fmt.Errorf("%w: active Runtime promotion denied", application.ErrAgentExecutionPolicyDenied)
		}
		return fmt.Errorf("authorize active Runtime promotion: %w", err)
	}
	return nil
}

func authorizeTriggerSubscriptionV1(ctx context.Context, store application.AgentPolicyStoreV1, request application.AgentExecutionPolicyStartV1, definition *application.AgentDefinitionVersionV1, at time.Time) error {
	subscriptionUUID := strings.TrimSpace(request.SubscriptionUUID)
	if subscriptionUUID == "" {
		return nil
	}
	reader, ok := store.(agentEventSubscriptionReaderV1)
	if !ok {
		return fmt.Errorf("%w: Agent Event Subscription reader unavailable", application.ErrAgentExecutionPolicyDenied)
	}
	subscription, err := reader.GetEventSubscription(ctx, subscriptionUUID)
	if err != nil || subscription == nil {
		return fmt.Errorf("%w: Agent Event Subscription unavailable", application.ErrAgentExecutionPolicyDenied)
	}
	match := application.AgentEventSubscriptionMatchRequestV1{
		TenantID: request.TenantID, AgentUUID: request.AgentUUID, EventType: request.TriggerType,
		ResourceType: subscription.ResourceType, ResourceID: subscription.ResourceID,
	}
	if subscription.SubscriptionUUID != subscriptionUUID || !ValidSubscriptionDefinitionV1(definition, *subscription, match, at.UTC()) {
		return fmt.Errorf("%w: Agent Event Subscription binding is invalid", application.ErrAgentExecutionPolicyDenied)
	}
	return nil
}

func (a *PersistentAgentRunAdmissionV1) Complete(ctx context.Context, taskUUID, runUUID, runtimeID, mode string) error {
	return a.Finish(ctx, taskUUID, runUUID, runtimeID, mode, application.AgentRunStatusCompleted, "")
}

func (a *PersistentAgentRunAdmissionV1) Finish(ctx context.Context, taskUUID, runUUID, runtimeID, mode string, runStatus application.AgentRunStatusV1, lastError string) error {
	taskUUID, runUUID, runtimeID, mode = strings.TrimSpace(taskUUID), strings.TrimSpace(runUUID), strings.TrimSpace(runtimeID), strings.TrimSpace(mode)
	lastError = strings.TrimSpace(lastError)
	if taskUUID == "" || runUUID == "" || runtimeID == "" || mode == "" {
		return fmt.Errorf("%w: Agent Run terminal identity is required", application.ErrAgentExecutionPolicyDenied)
	}
	if err := application.ValidateAgentRunTerminalV1(runStatus, lastError); err != nil {
		return fmt.Errorf("%w: Agent Run terminal evidence is invalid", application.ErrAgentExecutionPolicyDenied)
	}
	run, err := a.store.GetRun(ctx, runUUID)
	if err != nil {
		return fmt.Errorf("get Agent Run terminal state: %w", err)
	}
	if run == nil || run.TaskUUID != taskUUID || run.RuntimeID != runtimeID || run.Mode != mode {
		return fmt.Errorf("%w: Agent Run terminal binding mismatch", application.ErrAgentExecutionPolicyDenied)
	}
	if run.Status == runStatus {
		if strings.TrimSpace(run.LastError) == lastError {
			return nil
		}
		return fmt.Errorf("%w: Agent Run terminal evidence conflicts", application.ErrAgentExecutionPolicyDenied)
	}
	if run.Status != application.AgentRunStatusRunning {
		return fmt.Errorf("%w: Agent Run has a conflicting terminal state", application.ErrAgentExecutionPolicyDenied)
	}
	changed, err := a.store.TransitionRunStatus(ctx, runUUID, application.AgentRunStatusRunning, runStatus, lastError)
	if err != nil {
		return fmt.Errorf("finish Agent Run: %w", err)
	}
	if !changed {
		current, lookupErr := a.store.GetRun(ctx, runUUID)
		if lookupErr == nil && current != nil && current.Status == runStatus && strings.TrimSpace(current.LastError) == lastError {
			return nil
		}
		return fmt.Errorf("%w: Agent Run terminal transition lost compare-and-set", application.ErrAgentExecutionPolicyDenied)
	}
	return nil
}

func agentTaskMatchesStartV1(task application.AgentTaskV1, request application.AgentExecutionPolicyStartV1) bool {
	return strings.TrimSpace(task.TenantID) == strings.TrimSpace(request.TenantID) &&
		strings.TrimSpace(task.PrincipalUUID) == strings.TrimSpace(request.PrincipalUUID) &&
		strings.TrimSpace(task.AgentUUID) == strings.TrimSpace(request.AgentUUID) &&
		strings.TrimSpace(task.TriggerType) == strings.TrimSpace(request.TriggerType) &&
		strings.TrimSpace(task.TriggerRef) == strings.TrimSpace(request.TriggerRef) &&
		strings.TrimSpace(task.TriggerSubscriptionUUID) == strings.TrimSpace(request.SubscriptionUUID)
}

var _ application.AgentExecutionPolicyV1 = (*StaticAgentExecutionPolicyV1)(nil)
var _ application.AgentExecutionPolicyV1 = (*PersistentAgentExecutionPolicyV1)(nil)

func NewStaticAgentExecutionPolicyV1(permissions []string, scopes []application.AgentResourceScopeV1) (*StaticAgentExecutionPolicyV1, error) {
	if len(permissions) == 0 || len(scopes) == 0 {
		return nil, fmt.Errorf("static Agent execution policy requires permissions and scopes")
	}
	return &StaticAgentExecutionPolicyV1{permissions: append([]string(nil), permissions...), scopes: clonePolicyScopesV1(scopes)}, nil
}

func (p *StaticAgentExecutionPolicyV1) Start(_ context.Context, request application.AgentExecutionPolicyStartV1) (*application.AgentPolicyExecutionV1, error) {
	if err := validateAgentExecutionPolicyStartV1(request); err != nil {
		return nil, err
	}
	return &application.AgentPolicyExecutionV1{Invocation: invocationFromPolicyStartV1(request, p.permissions, p.scopes)}, nil
}

func (*StaticAgentExecutionPolicyV1) Complete(context.Context, application.AgentPolicyExecutionV1) error {
	return nil
}

func (*StaticAgentExecutionPolicyV1) Fail(context.Context, application.AgentPolicyExecutionV1) error {
	return nil
}

func NewPersistentAgentExecutionPolicyV1(store application.AgentPolicyStoreV1) (*PersistentAgentExecutionPolicyV1, error) {
	return newPersistentAgentExecutionPolicyV1(store, time.Now)
}

func NewPersistentAgentExecutionPolicyV1WithClock(store application.AgentPolicyStoreV1, now func() time.Time) (*PersistentAgentExecutionPolicyV1, error) {
	return newPersistentAgentExecutionPolicyV1(store, now)
}

func newPersistentAgentExecutionPolicyV1(store application.AgentPolicyStoreV1, now agentPolicyClockV1) (*PersistentAgentExecutionPolicyV1, error) {
	if store == nil || now == nil {
		return nil, fmt.Errorf("persistent Agent execution policy requires store and clock")
	}
	return &PersistentAgentExecutionPolicyV1{store: store, now: now}, nil
}

func EnsureEmbeddedAgentDefinitionV1(ctx context.Context, store application.AgentPolicyStoreV1, tenantID, agentUUID string, permissions []string, scopes []application.AgentResourceScopeV1) error {
	if store == nil {
		return fmt.Errorf("ensure Embedded Agent Definition: store is required")
	}
	tenantID = strings.TrimSpace(tenantID)
	agentUUID = strings.TrimSpace(agentUUID)
	if tenantID == "" || agentUUID == "" || len(permissions) == 0 || len(scopes) == 0 {
		return fmt.Errorf("ensure Embedded Agent Definition: identity, permissions, and scopes are required")
	}
	latest, err := store.GetLatestDefinition(ctx, tenantID, agentUUID)
	if err != nil {
		return fmt.Errorf("get Embedded Agent Definition: %w", err)
	}
	if latest != nil {
		return nil
	}
	definition := application.AgentDefinitionVersionV1{
		DefinitionUUID: "embedded:" + agentUUID,
		Version:        embeddedAgentDefinitionVersionV1,
		TenantID:       tenantID,
		OwnerUUID:      agentUUID,
		AgentUUID:      agentUUID,
		Status:         application.AgentDefinitionStatusActive,
		Permissions:    append([]string(nil), permissions...),
		Scopes:         clonePolicyScopesV1(scopes),
		ValidFrom:      time.Unix(0, 0).UTC(),
	}
	if err := store.CreateDefinitionVersion(ctx, definition); err != nil {
		// A concurrent process may have initialized the same Agent.
		latest, lookupErr := store.GetLatestDefinition(ctx, tenantID, agentUUID)
		if lookupErr != nil || latest == nil {
			return fmt.Errorf("create Embedded Agent Definition: %w", err)
		}
	}
	return nil
}

func (p *PersistentAgentExecutionPolicyV1) Start(ctx context.Context, request application.AgentExecutionPolicyStartV1) (*application.AgentPolicyExecutionV1, error) {
	if err := validateAgentExecutionPolicyStartV1(request); err != nil {
		return nil, err
	}
	taskUUID := agentTaskUUIDV1(request)
	existingTask, err := p.store.GetTask(ctx, taskUUID)
	if err != nil {
		return nil, fmt.Errorf("lookup Embedded Agent Task: %w", err)
	}
	var task application.AgentTaskV1
	if existingTask == nil {
		latest, lookupErr := p.store.GetLatestDefinition(ctx, strings.TrimSpace(request.TenantID), strings.TrimSpace(request.AgentUUID))
		if lookupErr != nil {
			return nil, fmt.Errorf("get latest Agent policy: %w", lookupErr)
		}
		if err := authorizeDefinitionAtV1(latest, request, p.now()); err != nil {
			return nil, err
		}
		if err := authorizeTriggerSubscriptionV1(ctx, p.store, request, latest, p.now()); err != nil {
			return nil, err
		}
		task = application.AgentTaskV1{
			TaskUUID: taskUUID, DefinitionUUID: latest.DefinitionUUID, DefinitionVersion: latest.Version,
			TenantID: strings.TrimSpace(request.TenantID), PrincipalUUID: strings.TrimSpace(request.PrincipalUUID),
			AgentUUID: strings.TrimSpace(request.AgentUUID), Status: application.AgentTaskStatusCreated,
			TriggerType: strings.TrimSpace(request.TriggerType), TriggerRef: strings.TrimSpace(request.TriggerRef), Goal: "handle_agent_trigger",
			TriggerSubscriptionUUID: strings.TrimSpace(request.SubscriptionUUID),
		}
		created, createErr := p.store.CreateTask(ctx, task)
		if createErr != nil {
			return nil, fmt.Errorf("create Agent policy Task: %w", createErr)
		}
		if !created {
			current, currentErr := p.store.GetTask(ctx, taskUUID)
			if currentErr != nil || current == nil {
				return nil, fmt.Errorf("%w: concurrent Embedded Agent Task unavailable", application.ErrAgentExecutionPolicyDenied)
			}
			task = *current
		}
	} else {
		task = *existingTask
	}
	if !agentTaskMatchesStartV1(task, request) || (task.Status != application.AgentTaskStatusCreated && task.Status != application.AgentTaskStatusRunning) {
		return nil, fmt.Errorf("%w: existing Agent Task cannot start Embedded Run", application.ErrAgentExecutionPolicyDenied)
	}
	pinned, err := p.store.GetDefinitionVersion(ctx, task.DefinitionUUID, task.DefinitionVersion)
	if err != nil {
		return nil, fmt.Errorf("get pinned Agent policy: %w", err)
	}
	if err := authorizeDefinitionAtV1(pinned, request, p.now()); err != nil {
		return nil, err
	}
	if task.Status == application.AgentTaskStatusCreated {
		changed, transitionErr := p.store.TransitionTaskStatus(ctx, task.TaskUUID, application.AgentTaskStatusCreated, application.AgentTaskStatusRunning)
		if transitionErr != nil {
			return nil, fmt.Errorf("start Agent policy Task: %w", transitionErr)
		}
		if !changed {
			current, lookupErr := p.store.GetTask(ctx, task.TaskUUID)
			if lookupErr != nil || current == nil || current.Status != application.AgentTaskStatusRunning || !agentTaskMatchesStartV1(*current, request) {
				return nil, fmt.Errorf("%w: Agent Task start lost compare-and-set", application.ErrAgentExecutionPolicyDenied)
			}
		}
	}
	runUUID, err := application.AgentRunUUIDV1(task.TaskUUID, embeddedAgentRuntimeIDV1, "embedded")
	if err != nil {
		return nil, fmt.Errorf("derive Embedded Agent Run: %w", err)
	}
	created, err := p.store.CreateRun(ctx, application.AgentRunV1{
		RunUUID: runUUID, TaskUUID: task.TaskUUID, RuntimeID: embeddedAgentRuntimeIDV1, TraceID: strings.TrimSpace(request.TraceID), Mode: "embedded", Status: application.AgentRunStatusRunning,
	})
	if err != nil || !created {
		if err == nil {
			err = application.ErrAgentExecutionPolicyDenied
		}
		return nil, fmt.Errorf("create Embedded Agent Run: %w", err)
	}
	return &application.AgentPolicyExecutionV1{
		TaskUUID:   task.TaskUUID,
		RunUUID:    runUUID,
		Invocation: invocationFromPolicyStartV1(request, pinned.Permissions, pinned.Scopes),
	}, nil
}

func (p *PersistentAgentExecutionPolicyV1) Complete(ctx context.Context, execution application.AgentPolicyExecutionV1) error {
	return p.transitionTerminalV1(ctx, execution, application.AgentRunStatusCompleted, application.AgentTaskStatusCompleted, "")
}

func (p *PersistentAgentExecutionPolicyV1) Fail(ctx context.Context, execution application.AgentPolicyExecutionV1) error {
	return p.transitionTerminalV1(ctx, execution, application.AgentRunStatusFailed, application.AgentTaskStatusFailed, "Agent execution failed")
}

func (p *PersistentAgentExecutionPolicyV1) transitionTerminalV1(ctx context.Context, execution application.AgentPolicyExecutionV1, runStatus application.AgentRunStatusV1, taskStatus application.AgentTaskStatusV1, lastError string) error {
	taskUUID, runUUID := strings.TrimSpace(execution.TaskUUID), strings.TrimSpace(execution.RunUUID)
	if taskUUID == "" || runUUID == "" {
		return fmt.Errorf("%w: Agent Task UUID is required", application.ErrAgentExecutionPolicyDenied)
	}
	run, err := p.store.GetRun(ctx, runUUID)
	if err != nil || run == nil || run.TaskUUID != taskUUID {
		return fmt.Errorf("%w: Agent Run terminal binding unavailable: %v", application.ErrAgentExecutionPolicyDenied, err)
	}
	if run.Status == application.AgentRunStatusRunning {
		changed, transitionErr := p.store.TransitionRunStatus(ctx, runUUID, application.AgentRunStatusRunning, runStatus, lastError)
		if transitionErr != nil {
			return fmt.Errorf("transition Agent Run terminal status: %w", transitionErr)
		}
		if !changed {
			current, lookupErr := p.store.GetRun(ctx, runUUID)
			if lookupErr != nil || current == nil || current.Status != runStatus {
				return fmt.Errorf("%w: Agent Run terminal transition lost compare-and-set", application.ErrAgentExecutionPolicyDenied)
			}
		}
	} else if run.Status != runStatus {
		return fmt.Errorf("%w: Agent Run has a conflicting terminal status", application.ErrAgentExecutionPolicyDenied)
	}
	changed, err := p.store.TransitionTaskStatus(ctx, taskUUID, application.AgentTaskStatusRunning, taskStatus)
	if err != nil {
		return fmt.Errorf("finish Agent policy Task: %w", err)
	}
	if !changed {
		current, lookupErr := p.store.GetTask(ctx, taskUUID)
		if lookupErr != nil || current == nil || current.Status != taskStatus {
			return fmt.Errorf("%w: Agent Task terminal transition lost compare-and-set", application.ErrAgentExecutionPolicyDenied)
		}
	}
	return nil
}

func validateAgentExecutionPolicyStartV1(request application.AgentExecutionPolicyStartV1) error {
	values := []string{request.TenantID, request.PrincipalUUID, request.AgentUUID, request.TriggerType, request.TriggerRef}
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%w: Agent policy identity and trigger are required", application.ErrAgentExecutionPolicyDenied)
		}
	}
	delegator := strings.TrimSpace(request.DelegatedByUUID)
	if delegator != "" && delegator != strings.TrimSpace(request.PrincipalUUID) {
		return fmt.Errorf("%w: Agent policy delegator mismatch", application.ErrAgentExecutionPolicyDenied)
	}
	return nil
}

func authorizeDefinitionAtV1(definition *application.AgentDefinitionVersionV1, request application.AgentExecutionPolicyStartV1, at time.Time) error {
	if definition == nil || definition.Validate() != nil || definition.Status != application.AgentDefinitionStatusActive || definition.RevokedAt != nil ||
		strings.TrimSpace(definition.TenantID) != strings.TrimSpace(request.TenantID) || strings.TrimSpace(definition.AgentUUID) != strings.TrimSpace(request.AgentUUID) ||
		at.Before(definition.ValidFrom) || (definition.ExpiresAt != nil && !at.Before(*definition.ExpiresAt)) {
		return fmt.Errorf("%w: Agent Definition is missing, revoked, expired, or outside scope", application.ErrAgentExecutionPolicyDenied)
	}
	return nil
}

func invocationFromPolicyStartV1(request application.AgentExecutionPolicyStartV1, permissions []string, scopes []application.AgentResourceScopeV1) application.AgentInvocationV1 {
	return application.AgentInvocationV1{
		TenantID: strings.TrimSpace(request.TenantID), PrincipalUUID: strings.TrimSpace(request.PrincipalUUID),
		AgentUUID: strings.TrimSpace(request.AgentUUID), DelegatedByUUID: strings.TrimSpace(request.DelegatedByUUID),
		Permissions: append([]string(nil), permissions...), ResourceScopes: clonePolicyScopesV1(scopes),
		RequestID: strings.TrimSpace(request.RequestID), TraceID: strings.TrimSpace(request.TraceID), EventID: strings.TrimSpace(request.EventID),
	}
}

func agentTaskUUIDV1(request application.AgentExecutionPolicyStartV1) string {
	canonical := application.AgentPolicyPersistenceVersionV1 + "\n" + strings.TrimSpace(request.TenantID) + "\n" + strings.TrimSpace(request.AgentUUID) + "\n" + strings.TrimSpace(request.TriggerType) + "\n" + strings.TrimSpace(request.TriggerRef)
	digest := sha256.Sum256([]byte(canonical))
	return "task:" + hex.EncodeToString(digest[:])[:59]
}

func clonePolicyScopesV1(scopes []application.AgentResourceScopeV1) []application.AgentResourceScopeV1 {
	cloned := make([]application.AgentResourceScopeV1, len(scopes))
	for index, scope := range scopes {
		cloned[index] = scope
		cloned[index].Actions = append([]string(nil), scope.Actions...)
	}
	return cloned
}

func AgentTaskUUIDV1(request application.AgentExecutionPolicyStartV1) string {
	return agentTaskUUIDV1(request)
}

func ClonePolicyScopesV1(scopes []application.AgentResourceScopeV1) []application.AgentResourceScopeV1 {
	return clonePolicyScopesV1(scopes)
}
