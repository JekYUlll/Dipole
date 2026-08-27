package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
)

var ErrAgentPolicyConflict = errors.New("agent policy persistence conflict")

type AgentPolicyRepository struct {
	queries generated.Querier
}

var _ application.AgentPolicyStoreV1 = (*AgentPolicyRepository)(nil)

func NewAgentPolicyRepository(queries generated.Querier) (*AgentPolicyRepository, error) {
	if queries == nil {
		return nil, errors.New("Agent Policy queries are required")
	}
	return &AgentPolicyRepository{queries: queries}, nil
}

func (r *AgentPolicyRepository) CreateDefinitionVersion(ctx context.Context, definition application.AgentDefinitionVersionV1) error {
	if err := definition.Validate(); err != nil {
		return fmt.Errorf("validate Agent Definition version: %w", err)
	}
	permissions, err := json.Marshal(definition.Permissions)
	if err != nil {
		return fmt.Errorf("marshal Agent Definition permissions: %w", err)
	}
	scopes, err := json.Marshal(definition.Scopes)
	if err != nil {
		return fmt.Errorf("marshal Agent Definition scopes: %w", err)
	}
	err = r.queries.InsertAgentDefinitionVersion(ctx, generated.InsertAgentDefinitionVersionParams{
		DefinitionUuid: definition.DefinitionUUID, Version: definition.Version, TenantID: definition.TenantID,
		OwnerUuid: definition.OwnerUUID, AgentUuid: definition.AgentUUID, Status: string(definition.Status),
		PermissionsJson: permissions, ScopesJson: scopes, ValidFrom: definition.ValidFrom,
		ExpiresAt: nullableTime(definition.ExpiresAt), RevokedAt: nullableTime(definition.RevokedAt),
	})
	if err != nil {
		return fmt.Errorf("create Agent Definition version: %w", err)
	}
	return nil
}

func (r *AgentPolicyRepository) GetLatestDefinition(ctx context.Context, tenantID, agentUUID string) (*application.AgentDefinitionVersionV1, error) {
	row, err := r.queries.GetLatestAgentDefinition(ctx, generated.GetLatestAgentDefinitionParams{TenantID: tenantID, AgentUuid: agentUUID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest Agent Definition: %w", err)
	}
	return mapAgentDefinitionVersion(row)
}

func (r *AgentPolicyRepository) GetDefinitionVersion(ctx context.Context, definitionUUID string, version uint64) (*application.AgentDefinitionVersionV1, error) {
	row, err := r.queries.GetAgentDefinitionVersion(ctx, generated.GetAgentDefinitionVersionParams{
		DefinitionUuid: definitionUUID, Version: version,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get Agent Definition version: %w", err)
	}
	return mapAgentDefinitionVersion(row)
}

func mapAgentDefinitionVersion(row generated.AgentDefinitionVersion) (*application.AgentDefinitionVersionV1, error) {
	var permissions []string
	if err := json.Unmarshal(row.PermissionsJson, &permissions); err != nil {
		return nil, fmt.Errorf("decode Agent Definition permissions: %w", err)
	}
	var scopes []application.AgentResourceScopeV1
	if err := json.Unmarshal(row.ScopesJson, &scopes); err != nil {
		return nil, fmt.Errorf("decode Agent Definition scopes: %w", err)
	}
	return &application.AgentDefinitionVersionV1{
		DefinitionUUID: row.DefinitionUuid, Version: row.Version, TenantID: row.TenantID,
		OwnerUUID: row.OwnerUuid, AgentUUID: row.AgentUuid, Status: application.AgentDefinitionStatusV1(row.Status),
		Permissions: permissions, Scopes: scopes, ValidFrom: row.ValidFrom,
		ExpiresAt: timePointer(row.ExpiresAt), RevokedAt: timePointer(row.RevokedAt),
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

func (r *AgentPolicyRepository) RevokeDefinitionVersion(ctx context.Context, definitionUUID string, version uint64, revokedAt time.Time) error {
	_, err := r.queries.RevokeAgentDefinitionVersion(ctx, generated.RevokeAgentDefinitionVersionParams{
		RevokedAt: nullableTime(&revokedAt), DefinitionUuid: definitionUUID, Version: version,
	})
	if err != nil {
		return fmt.Errorf("revoke Agent Definition version: %w", err)
	}
	return nil
}

func (r *AgentPolicyRepository) CreateTask(ctx context.Context, task application.AgentTaskV1) (bool, error) {
	if err := task.Validate(); err != nil {
		return false, fmt.Errorf("validate Agent Task: %w", err)
	}
	rows, err := r.queries.InsertAgentTask(ctx, generated.InsertAgentTaskParams{
		TaskUuid: task.TaskUUID, DefinitionUuid: task.DefinitionUUID, DefinitionVersion: task.DefinitionVersion,
		TenantID: task.TenantID, PrincipalUuid: task.PrincipalUUID, AgentUuid: task.AgentUUID,
		Status: string(task.Status), TriggerType: task.TriggerType, TriggerRef: task.TriggerRef, Goal: task.Goal,
	})
	if err != nil {
		return false, fmt.Errorf("create Agent Task: %w", err)
	}
	if rows > 0 {
		return true, nil
	}
	existing, err := r.GetTask(ctx, task.TaskUUID)
	if err != nil {
		return false, err
	}
	if existing == nil || !sameAgentTask(*existing, task) {
		return false, fmt.Errorf("%w: task_uuid=%s", ErrAgentPolicyConflict, task.TaskUUID)
	}
	return false, nil
}

func (r *AgentPolicyRepository) GetTask(ctx context.Context, taskUUID string) (*application.AgentTaskV1, error) {
	row, err := r.queries.GetAgentTask(ctx, taskUUID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get Agent Task: %w", err)
	}
	return &application.AgentTaskV1{
		TaskUUID: row.TaskUuid, DefinitionUUID: row.DefinitionUuid, DefinitionVersion: row.DefinitionVersion,
		TenantID: row.TenantID, PrincipalUUID: row.PrincipalUuid, AgentUUID: row.AgentUuid,
		Status: application.AgentTaskStatusV1(row.Status), TriggerType: row.TriggerType, TriggerRef: row.TriggerRef,
		Goal: row.Goal, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

func (r *AgentPolicyRepository) TransitionTaskStatus(ctx context.Context, taskUUID string, from, to application.AgentTaskStatusV1) (bool, error) {
	if err := application.ValidateAgentTaskTransitionV1(from, to); err != nil {
		return false, fmt.Errorf("validate Agent Task transition: %w", err)
	}
	rows, err := r.queries.TransitionAgentTaskStatus(ctx, generated.TransitionAgentTaskStatusParams{
		Status: string(to), TaskUuid: taskUUID, Status_2: string(from),
	})
	if err != nil {
		return false, fmt.Errorf("transition Agent Task status: %w", err)
	}
	return rows > 0, nil
}

func (r *AgentPolicyRepository) CreateApproval(ctx context.Context, approval application.AgentApprovalV1) error {
	if err := approval.Validate(); err != nil {
		return fmt.Errorf("validate Agent Approval: %w", err)
	}
	resourceScope, err := json.Marshal(approval.ResourceScope)
	if err != nil {
		return fmt.Errorf("marshal Agent Approval scope: %w", err)
	}
	err = r.queries.InsertAgentApproval(ctx, generated.InsertAgentApprovalParams{
		ApprovalUuid: approval.ApprovalUUID, TaskUuid: approval.TaskUUID, CapabilityID: approval.CapabilityID,
		ResourceScopeJson: resourceScope, ScopeSha256: approval.ScopeSHA256, ArgumentsSha256: approval.ArgumentsSHA256,
		NonceSha256: approval.NonceSHA256, Status: string(approval.Status), ApprovedByUuid: approval.ApprovedByUUID,
		ExpiresAt: approval.ExpiresAt, ConsumedAt: nullableTime(approval.ConsumedAt), RevokedAt: nullableTime(approval.RevokedAt),
	})
	if err != nil {
		return fmt.Errorf("create Agent Approval: %w", err)
	}
	return nil
}

func (r *AgentPolicyRepository) ConsumeApproval(ctx context.Context, approvalUUID string, claim application.AgentApprovalClaimV1, consumedAt time.Time) (bool, error) {
	if err := claim.Validate(); err != nil || consumedAt.IsZero() {
		return false, fmt.Errorf("validate Agent Approval claim: %w", application.ErrAgentPolicyInvalid)
	}
	rows, err := r.queries.ConsumeAgentApproval(ctx, generated.ConsumeAgentApprovalParams{
		ConsumedAt: nullableTime(&consumedAt), ApprovalUuid: approvalUUID, TaskUuid: claim.TaskUUID,
		CapabilityID: claim.CapabilityID, ScopeSha256: claim.ScopeSHA256, ArgumentsSha256: claim.ArgumentsSHA256,
		NonceSha256: claim.NonceSHA256, ExpiresAt: consumedAt,
	})
	if err != nil {
		return false, fmt.Errorf("consume Agent Approval: %w", err)
	}
	return rows > 0, nil
}

func (r *AgentPolicyRepository) ApproveApproval(ctx context.Context, approvalUUID, approvedByUUID string, approvedAt time.Time) (bool, error) {
	if approvalUUID == "" || approvedByUUID == "" || approvedAt.IsZero() {
		return false, fmt.Errorf("validate Agent Approval transition: %w", application.ErrAgentPolicyInvalid)
	}
	rows, err := r.queries.ApproveAgentApproval(ctx, generated.ApproveAgentApprovalParams{
		ApprovedByUuid: approvedByUUID, ApprovalUuid: approvalUUID, ExpiresAt: approvedAt,
	})
	if err != nil {
		return false, fmt.Errorf("approve Agent Approval: %w", err)
	}
	return rows > 0, nil
}

func (r *AgentPolicyRepository) RevokeApproval(ctx context.Context, approvalUUID string, revokedAt time.Time) error {
	_, err := r.queries.RevokeAgentApproval(ctx, generated.RevokeAgentApprovalParams{
		RevokedAt: nullableTime(&revokedAt), ApprovalUuid: approvalUUID,
	})
	if err != nil {
		return fmt.Errorf("revoke Agent Approval: %w", err)
	}
	return nil
}

func sameAgentTask(left, right application.AgentTaskV1) bool {
	left.CreatedAt, left.UpdatedAt = time.Time{}, time.Time{}
	right.CreatedAt, right.UpdatedAt = time.Time{}, time.Time{}
	return reflect.DeepEqual(left, right)
}

func nullableTime(value *time.Time) sql.NullTime {
	if value == nil || value.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: value.UTC(), Valid: true}
}

func timePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time
	return &t
}
