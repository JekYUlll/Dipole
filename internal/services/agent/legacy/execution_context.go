package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
)

var ErrExecutionContextMissing = errors.New("agent execution context is missing")

const defaultAgentTenantID = "dipole"

// ExecutionContext contains trusted run identity and correlation metadata.
// Tool arguments must not populate these fields.
type ExecutionContext struct {
	TenantID             string
	PrincipalUserUUID    string
	AgentUUID            string
	DelegatedByUUID      string
	TriggerMessageUUID   string
	ConversationKey      string
	RequestID            string
	TraceID              string
	EventID              string
	permissions          []string
	resourceScopes       []application.AgentResourceScopeV1
	approvedCapabilities []string
}

type executionContextKey struct{}

func withExecutionContext(ctx context.Context, execution ExecutionContext) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, executionContextKey{}, execution)
}

func newExecutionContext(execution ExecutionContext, permissions, approvedCapabilities []string, resourceScopes ...[]application.AgentResourceScopeV1) ExecutionContext {
	execution.permissions = append([]string(nil), permissions...)
	execution.approvedCapabilities = append([]string(nil), approvedCapabilities...)
	if len(resourceScopes) > 0 {
		execution.resourceScopes = cloneAgentResourceScopesV1(resourceScopes[0])
	}
	return execution
}

func requireExecutionContext(ctx context.Context) (ExecutionContext, error) {
	if ctx == nil {
		return ExecutionContext{}, ErrExecutionContextMissing
	}
	execution, ok := ctx.Value(executionContextKey{}).(ExecutionContext)
	if !ok || strings.TrimSpace(execution.TenantID) == "" || strings.TrimSpace(execution.PrincipalUserUUID) == "" || strings.TrimSpace(execution.AgentUUID) == "" {
		return ExecutionContext{}, ErrExecutionContextMissing
	}
	execution.PrincipalUserUUID = strings.TrimSpace(execution.PrincipalUserUUID)
	execution.AgentUUID = strings.TrimSpace(execution.AgentUUID)
	return execution, nil
}

func (e ExecutionContext) invocationV1() application.AgentInvocationV1 {
	return application.AgentInvocationV1{
		TenantID:             strings.TrimSpace(e.TenantID),
		PrincipalUUID:        strings.TrimSpace(e.PrincipalUserUUID),
		AgentUUID:            strings.TrimSpace(e.AgentUUID),
		DelegatedByUUID:      strings.TrimSpace(e.DelegatedByUUID),
		Permissions:          append([]string(nil), e.permissions...),
		ResourceScopes:       cloneAgentResourceScopesV1(e.resourceScopes),
		ApprovedCapabilities: append([]string(nil), e.approvedCapabilities...),
		RequestID:            strings.TrimSpace(e.RequestID),
		TraceID:              strings.TrimSpace(e.TraceID),
		EventID:              strings.TrimSpace(e.EventID),
	}
}

func authorizeExecutionCapabilityForResource(execution ExecutionContext, capabilityID, resourceType, resourceID, action string) error {
	descriptor, ok := application.AgentCapabilityDescriptorByIDV1(capabilityID)
	if !ok {
		return fmt.Errorf("%w: unknown capability %s", application.ErrAgentCapabilityDenied, capabilityID)
	}
	return application.AuthorizeAgentCapabilityForResourceV1(execution.invocationV1(), descriptor, resourceType, resourceID, action)
}

func authorizeExecutionCapability(execution ExecutionContext, capabilityID string) error {
	descriptor, ok := application.AgentCapabilityDescriptorByIDV1(capabilityID)
	if !ok {
		return fmt.Errorf("%w: unknown capability %s", application.ErrAgentCapabilityDenied, capabilityID)
	}
	return application.AuthorizeAgentCapabilityV1(execution.invocationV1(), descriptor)
}

func requireAuthorizedExecution(ctx context.Context, capabilityID string) (ExecutionContext, error) {
	execution, err := requireExecutionContext(ctx)
	if err != nil {
		return ExecutionContext{}, err
	}
	if err := authorizeExecutionCapability(execution, capabilityID); err != nil {
		return ExecutionContext{}, err
	}
	return execution, nil
}

func requireAuthorizedExecutionForResource(ctx context.Context, capabilityID, resourceType, resourceID, action string) (ExecutionContext, error) {
	execution, err := requireExecutionContext(ctx)
	if err != nil {
		return ExecutionContext{}, err
	}
	if err := authorizeExecutionCapabilityForResource(execution, capabilityID, resourceType, resourceID, action); err != nil {
		return ExecutionContext{}, err
	}
	return execution, nil
}

func embeddedAgentPermissionsV1() []string {
	permissions, _ := application.EmbeddedAgentPolicyGrantV1()
	return permissions
}

func embeddedAgentResourceScopesV1() []application.AgentResourceScopeV1 {
	_, scopes := application.EmbeddedAgentPolicyGrantV1()
	return scopes
}

func cloneAgentResourceScopesV1(scopes []application.AgentResourceScopeV1) []application.AgentResourceScopeV1 {
	cloned := make([]application.AgentResourceScopeV1, len(scopes))
	for index, scope := range scopes {
		cloned[index] = scope
		cloned[index].Actions = append([]string(nil), scope.Actions...)
	}
	return cloned
}
