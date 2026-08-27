package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
)

type AgentToolInvocationRepository struct{ queries generated.Querier }

var _ application.AgentToolInvocationStoreV1 = (*AgentToolInvocationRepository)(nil)

func NewAgentToolInvocationRepository(queries generated.Querier) (*AgentToolInvocationRepository, error) {
	if queries == nil {
		return nil, errors.New("Agent Tool invocation queries are required")
	}
	return &AgentToolInvocationRepository{queries: queries}, nil
}

func (r *AgentToolInvocationRepository) BeginToolInvocation(ctx context.Context, invocation application.AgentToolInvocationV1) (bool, error) {
	begin := application.AgentToolInvocationBeginV1{
		InvocationUUID: invocation.InvocationUUID, TaskUUID: invocation.TaskUUID, RunUUID: invocation.RunUUID,
		Transport: invocation.Transport, ToolName: invocation.ToolName, CapabilityID: invocation.CapabilityID,
		ArgumentsSHA256: invocation.ArgumentsSHA256, RequestID: invocation.RequestID, TraceID: invocation.TraceID,
	}
	if err := begin.Validate(); err != nil || invocation.Status != application.AgentToolInvocationStatusRunning || invocation.StartedAt.IsZero() || invocation.TenantID == "" || invocation.PrincipalUUID == "" || invocation.AgentUUID == "" {
		return false, application.ErrAgentToolInvocationInvalid
	}
	rows, err := r.queries.InsertAgentToolInvocation(ctx, generated.InsertAgentToolInvocationParams{
		InvocationUuid: invocation.InvocationUUID, TenantID: invocation.TenantID, PrincipalUuid: invocation.PrincipalUUID, AgentUuid: invocation.AgentUUID,
		TaskUuid: invocation.TaskUUID, RunUuid: invocation.RunUUID, Transport: string(invocation.Transport), ToolName: invocation.ToolName,
		CapabilityID: invocation.CapabilityID, ArgumentsSha256: invocation.ArgumentsSHA256, Status: string(invocation.Status),
		RequestID: nullableString(invocation.RequestID), TraceID: nullableString(invocation.TraceID), StartedAt: invocation.StartedAt.UTC(),
	})
	if err != nil {
		return false, fmt.Errorf("insert Agent Tool invocation: %w", err)
	}
	return rows == 1, nil
}

func (r *AgentToolInvocationRepository) FinishToolInvocation(ctx context.Context, finish application.AgentToolInvocationFinishV1) (bool, error) {
	if err := finish.Validate(); err != nil {
		return false, err
	}
	rows, err := r.queries.FinishAgentToolInvocation(ctx, generated.FinishAgentToolInvocationParams{
		Status: string(finish.Status), ResultSha256: nullableString(finish.ResultSHA256),
		ResultBytes: nullableUint64(finish.ResultBytes, finish.Status == application.AgentToolInvocationStatusCompleted),
		LatencyMs:   sql.NullInt64{Int64: int64(finish.LatencyMS), Valid: true}, ErrorCode: nullableString(finish.ErrorCode),
		InvocationUuid: finish.InvocationUUID, TaskUuid: finish.TaskUUID, RunUuid: finish.RunUUID,
	})
	if err != nil {
		return false, fmt.Errorf("finish Agent Tool invocation: %w", err)
	}
	return rows == 1, nil
}

func nullableUint64(value uint64, valid bool) sql.NullInt64 {
	return sql.NullInt64{Int64: int64(value), Valid: valid}
}
