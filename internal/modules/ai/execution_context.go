package ai

import (
	"context"
	"errors"
	"strings"
)

var ErrExecutionContextMissing = errors.New("agent execution context is missing")

// ExecutionContext contains trusted run identity and correlation metadata.
// Tool arguments must not populate these fields.
type ExecutionContext struct {
	PrincipalUserUUID  string
	AgentUUID          string
	TriggerMessageUUID string
	ConversationKey    string
	RequestID          string
	TraceID            string
	EventID            string
}

type executionContextKey struct{}

func withExecutionContext(ctx context.Context, execution ExecutionContext) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, executionContextKey{}, execution)
}

func requireExecutionContext(ctx context.Context) (ExecutionContext, error) {
	if ctx == nil {
		return ExecutionContext{}, ErrExecutionContextMissing
	}
	execution, ok := ctx.Value(executionContextKey{}).(ExecutionContext)
	if !ok || strings.TrimSpace(execution.PrincipalUserUUID) == "" || strings.TrimSpace(execution.AgentUUID) == "" {
		return ExecutionContext{}, ErrExecutionContextMissing
	}
	execution.PrincipalUserUUID = strings.TrimSpace(execution.PrincipalUserUUID)
	execution.AgentUUID = strings.TrimSpace(execution.AgentUUID)
	return execution, nil
}
