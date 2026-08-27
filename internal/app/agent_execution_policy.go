package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

const embeddedAgentDefinitionVersionV1 uint64 = 1

type agentPolicyClockV1 func() time.Time

type StaticAgentExecutionPolicyV1 struct {
	permissions []string
	scopes      []application.AgentResourceScopeV1
}

type PersistentAgentExecutionPolicyV1 struct {
	store application.AgentPolicyStoreV1
	now   agentPolicyClockV1
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
	latest, err := p.store.GetLatestDefinition(ctx, strings.TrimSpace(request.TenantID), strings.TrimSpace(request.AgentUUID))
	if err != nil {
		return nil, fmt.Errorf("get latest Agent policy: %w", err)
	}
	if err := authorizeDefinitionAtV1(latest, request, p.now()); err != nil {
		return nil, err
	}

	task := application.AgentTaskV1{
		TaskUUID:          agentTaskUUIDV1(request),
		DefinitionUUID:    latest.DefinitionUUID,
		DefinitionVersion: latest.Version,
		TenantID:          strings.TrimSpace(request.TenantID),
		PrincipalUUID:     strings.TrimSpace(request.PrincipalUUID),
		AgentUUID:         strings.TrimSpace(request.AgentUUID),
		Status:            application.AgentTaskStatusCreated,
		TriggerType:       strings.TrimSpace(request.TriggerType),
		TriggerRef:        strings.TrimSpace(request.TriggerRef),
		Goal:              "handle_agent_trigger",
	}
	created, err := p.store.CreateTask(ctx, task)
	if err != nil {
		return nil, fmt.Errorf("create Agent policy Task: %w", err)
	}
	if !created {
		return nil, fmt.Errorf("%w: Agent Task already exists", application.ErrAgentExecutionPolicyDenied)
	}

	pinned, err := p.store.GetDefinitionVersion(ctx, task.DefinitionUUID, task.DefinitionVersion)
	if err != nil {
		return nil, fmt.Errorf("get pinned Agent policy: %w", err)
	}
	if err := authorizeDefinitionAtV1(pinned, request, p.now()); err != nil {
		_, _ = p.store.TransitionTaskStatus(ctx, task.TaskUUID, application.AgentTaskStatusCreated, application.AgentTaskStatusCancelled)
		return nil, err
	}
	changed, err := p.store.TransitionTaskStatus(ctx, task.TaskUUID, application.AgentTaskStatusCreated, application.AgentTaskStatusRunning)
	if err != nil {
		return nil, fmt.Errorf("start Agent policy Task: %w", err)
	}
	if !changed {
		return nil, fmt.Errorf("%w: Agent Task start lost compare-and-set", application.ErrAgentExecutionPolicyDenied)
	}
	return &application.AgentPolicyExecutionV1{
		TaskUUID:   task.TaskUUID,
		Invocation: invocationFromPolicyStartV1(request, pinned.Permissions, pinned.Scopes),
	}, nil
}

func (p *PersistentAgentExecutionPolicyV1) Complete(ctx context.Context, execution application.AgentPolicyExecutionV1) error {
	return p.transitionTerminalV1(ctx, execution.TaskUUID, application.AgentTaskStatusCompleted)
}

func (p *PersistentAgentExecutionPolicyV1) Fail(ctx context.Context, execution application.AgentPolicyExecutionV1) error {
	return p.transitionTerminalV1(ctx, execution.TaskUUID, application.AgentTaskStatusFailed)
}

func (p *PersistentAgentExecutionPolicyV1) transitionTerminalV1(ctx context.Context, taskUUID string, status application.AgentTaskStatusV1) error {
	taskUUID = strings.TrimSpace(taskUUID)
	if taskUUID == "" {
		return fmt.Errorf("%w: Agent Task UUID is required", application.ErrAgentExecutionPolicyDenied)
	}
	changed, err := p.store.TransitionTaskStatus(ctx, taskUUID, application.AgentTaskStatusRunning, status)
	if err != nil {
		return fmt.Errorf("finish Agent policy Task: %w", err)
	}
	if !changed {
		return fmt.Errorf("%w: Agent Task terminal transition lost compare-and-set", application.ErrAgentExecutionPolicyDenied)
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
