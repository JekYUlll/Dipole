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
	approvedCapabilities []string
}

type executionContextKey struct{}

func withExecutionContext(ctx context.Context, execution ExecutionContext) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, executionContextKey{}, execution)
}

func newExecutionContext(execution ExecutionContext, permissions, approvedCapabilities []string) ExecutionContext {
	execution.permissions = append([]string(nil), permissions...)
	execution.approvedCapabilities = append([]string(nil), approvedCapabilities...)
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
		ApprovedCapabilities: append([]string(nil), e.approvedCapabilities...),
		RequestID:            strings.TrimSpace(e.RequestID),
		TraceID:              strings.TrimSpace(e.TraceID),
		EventID:              strings.TrimSpace(e.EventID),
	}
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

func embeddedAgentPermissionsV1() []string {
	return []string{
		application.AgentPermissionUserProfileRead,
		application.AgentPermissionConversationList,
		application.AgentPermissionConversationRead,
		application.AgentPermissionMessageWrite,
	}
}
