package repository

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

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
		ArgumentsSHA256: invocation.ArgumentsSHA256, ProfileID: invocation.ProfileID, ServerID: invocation.ServerID, ArgumentsJSON: invocation.ArgumentsJSON,
		RequestID: invocation.RequestID, TraceID: invocation.TraceID, ApprovalUUID: invocation.ApprovalUUID,
	}
	if err := begin.Validate(); err != nil || invocation.Status != application.AgentToolInvocationStatusRunning || invocation.StartedAt.IsZero() || invocation.TenantID == "" || invocation.PrincipalUUID == "" || invocation.AgentUUID == "" {
		return false, application.ErrAgentToolInvocationInvalid
	}
	rows, err := r.queries.InsertAgentToolInvocation(ctx, generated.InsertAgentToolInvocationParams{
		InvocationUuid: invocation.InvocationUUID, TenantID: invocation.TenantID, PrincipalUuid: invocation.PrincipalUUID, AgentUuid: invocation.AgentUUID,
		TaskUuid: invocation.TaskUUID, RunUuid: invocation.RunUUID, Transport: string(invocation.Transport), ToolName: invocation.ToolName,
		CapabilityID: invocation.CapabilityID, ArgumentsSha256: invocation.ArgumentsSHA256,
		ProfileID: nullableString(invocation.ProfileID), ServerID: nullableString(invocation.ServerID), ArgumentsJson: nullableAgentToolJSON(invocation.ArgumentsJSON), Status: string(invocation.Status),
		RequestID: nullableString(invocation.RequestID), TraceID: nullableString(invocation.TraceID), ApprovalUuid: nullableString(invocation.ApprovalUUID), StartedAt: invocation.StartedAt.UTC(),
	})
	if err != nil {
		return false, fmt.Errorf("insert Agent Tool invocation: %w", err)
	}
	return rows == 1, nil
}

func (r *AgentToolInvocationRepository) GetToolInvocation(ctx context.Context, invocationUUID string) (*application.AgentToolInvocationV1, error) {
	row, err := r.queries.GetAgentToolInvocation(ctx, invocationUUID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get Agent Tool invocation: %w", err)
	}
	record := &application.AgentToolInvocationV1{
		InvocationUUID: row.InvocationUuid, TenantID: row.TenantID, PrincipalUUID: row.PrincipalUuid, AgentUUID: row.AgentUuid,
		TaskUUID: row.TaskUuid, RunUUID: row.RunUuid, Transport: application.AgentToolTransportV1(row.Transport), ToolName: row.ToolName,
		CapabilityID: row.CapabilityID, ArgumentsSHA256: row.ArgumentsSha256, ProfileID: row.ProfileID.String, ServerID: row.ServerID.String,
		ArgumentsJSON: string(row.ArgumentsJson), Status: application.AgentToolInvocationStatusV1(row.Status),
		RequestID: row.RequestID.String, TraceID: row.TraceID.String, ApprovalUUID: row.ApprovalUuid.String, StartedAt: row.StartedAt,
		ResultSHA256: row.ResultSha256.String, ResultBytes: nullInt64Uint64(row.ResultBytes), LatencyMS: nullInt64Uint64(row.LatencyMs),
		ErrorCode: row.ErrorCode.String, ActionReference: agentToolActionReference(row), FinishedAt: nullTimePointer(row.FinishedAt),
	}
	if record.ArgumentsJSON == "null" {
		record.ArgumentsJSON = ""
	} else if record.ArgumentsJSON != "" {
		var compact bytes.Buffer
		if err := json.Compact(&compact, row.ArgumentsJson); err != nil {
			return nil, fmt.Errorf("compact Agent Tool invocation arguments: %w", err)
		}
		record.ArgumentsJSON = compact.String()
	}
	return record, nil
}

func nullInt64Uint64(value sql.NullInt64) uint64 {
	if !value.Valid || value.Int64 < 0 {
		return 0
	}
	return uint64(value.Int64)
}

func nullTimePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}

func agentToolActionReference(row generated.AgentToolInvocation) *application.AgentToolActionReferenceV1 {
	if !row.ActionResourceType.Valid && !row.ActionResourceUuid.Valid && !row.ActionCommandKind.Valid && !row.ActionCommandID.Valid {
		return nil
	}
	return &application.AgentToolActionReferenceV1{
		ResourceType: application.AgentToolActionResourceTypeV1(row.ActionResourceType.String),
		ResourceUUID: row.ActionResourceUuid.String,
		CommandKind:  application.AgentMessageCommandKindV1(row.ActionCommandKind.String),
		CommandID:    row.ActionCommandID.String,
	}
}

func (r *AgentToolInvocationRepository) FinishToolInvocation(ctx context.Context, finish application.AgentToolInvocationFinishV1) (bool, error) {
	if err := finish.Validate(); err != nil {
		return false, err
	}
	var resourceType, resourceUUID, commandKind, commandID sql.NullString
	if finish.ActionReference != nil {
		resourceType = nullableString(string(finish.ActionReference.ResourceType))
		resourceUUID = nullableString(finish.ActionReference.ResourceUUID)
		commandKind = nullableString(string(finish.ActionReference.CommandKind))
		commandID = nullableString(finish.ActionReference.CommandID)
	}
	rows, err := r.queries.FinishAgentToolInvocation(ctx, generated.FinishAgentToolInvocationParams{
		Status: string(finish.Status), ResultSha256: nullableString(finish.ResultSHA256),
		ResultBytes: nullableUint64(finish.ResultBytes, finish.Status == application.AgentToolInvocationStatusCompleted),
		LatencyMs:   sql.NullInt64{Int64: int64(finish.LatencyMS), Valid: true}, ErrorCode: nullableString(finish.ErrorCode),
		ActionResourceType: resourceType, ActionResourceUuid: resourceUUID, ActionCommandKind: commandKind, ActionCommandID: commandID,
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

func nullableAgentToolJSON(value string) json.RawMessage {
	if value == "" {
		return nil
	}
	return json.RawMessage(value)
}
