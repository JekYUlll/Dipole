package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	mysqlDriver "github.com/go-sql-driver/mysql"
)

var ErrAgentPolicyConflict = errors.New("agent policy persistence conflict")

type AgentPolicyRepository struct {
	queries generated.Querier
	store   transactionStore
}

var _ application.AgentPolicyStoreV1 = (*AgentPolicyRepository)(nil)
var _ application.AgentDefinitionCatalogStoreV1 = (*AgentPolicyRepository)(nil)
var _ application.AgentApprovalGrantStoreV1 = (*AgentPolicyRepository)(nil)
var _ application.AgentRuntimePromotionGrantStoreV1 = (*AgentPolicyRepository)(nil)
var _ application.AgentEventSubscriptionStoreV1 = (*AgentPolicyRepository)(nil)
var _ application.AgentWorkflowRepairAuditStoreV1 = (*AgentPolicyRepository)(nil)
var _ application.AgentWorkflowRepairExecutionStoreV1 = (*AgentPolicyRepository)(nil)

func NewAgentPolicyRepository(queries generated.Querier) (*AgentPolicyRepository, error) {
	if queries == nil {
		return nil, errors.New("Agent Policy queries are required")
	}
	return &AgentPolicyRepository{queries: queries}, nil
}

// NewAgentPolicyRepositoryWithTransactions keeps policy writes and timeline events atomic.
func NewAgentPolicyRepositoryWithTransactions(store transactionStore) (*AgentPolicyRepository, error) {
	if store == nil || store.Queries() == nil {
		return nil, errors.New("Agent Policy transaction store is required")
	}
	return &AgentPolicyRepository{queries: store.Queries(), store: store}, nil
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

func (r *AgentPolicyRepository) ListOwnedActiveDefinitions(ctx context.Context, tenantID, ownerUUID, afterDefinitionUUID string, afterVersion uint64, activeAt time.Time, limit int) ([]application.AgentDefinitionVersionV1, error) {
	rows, err := r.queries.ListOwnedActiveAgentDefinitions(ctx, generated.ListOwnedActiveAgentDefinitionsParams{
		TenantID: tenantID, OwnerUuid: ownerUUID, ValidAt: activeAt, ExpiresAfter: nullableTime(&activeAt),
		AfterDefinitionUuid: afterDefinitionUUID, AfterVersion: afterVersion, Limit: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list owned active Agent Definitions: %w", err)
	}
	definitions := make([]application.AgentDefinitionVersionV1, 0, len(rows))
	for _, row := range rows {
		definition, mapErr := mapAgentDefinitionVersion(row)
		if mapErr != nil {
			return nil, mapErr
		}
		definitions = append(definitions, *definition)
	}
	return definitions, nil
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

func (r *AgentPolicyRepository) CreateRuntimePromotionGrant(ctx context.Context, grant application.AgentRuntimePromotionGrantV1) (bool, error) {
	if grant.Validate() != nil || grant.RevokedAt != nil {
		return false, fmt.Errorf("validate Agent Runtime promotion grant: %w", application.ErrAgentPolicyInvalid)
	}
	rows, err := r.queries.InsertAgentRuntimePromotionGrant(ctx, generated.InsertAgentRuntimePromotionGrantParams{
		GrantUuid: grant.GrantUUID, TenantID: grant.TenantID, RuntimeID: grant.RuntimeID, CandidateVersion: grant.CandidateVersion,
		DefinitionUuid: grant.DefinitionUUID, DefinitionVersion: grant.DefinitionVersion, PolicyVersion: grant.PolicyVersion,
		EvidenceSha256: grant.EvidenceSHA256, EvalSuiteSha256: grant.EvalSuiteSHA256, GrantedByUuid: grant.GrantedByUUID,
		ReviewedByUuid: grant.ReviewedByUUID, ValidFrom: grant.ValidFrom, ExpiresAt: grant.ExpiresAt,
	})
	if err != nil {
		return false, fmt.Errorf("create Agent Runtime promotion grant: %w", err)
	}
	if rows > 0 {
		return true, nil
	}
	row, err := r.queries.GetAgentRuntimePromotionGrant(ctx, grant.GrantUUID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("%w: Runtime promotion binding is already occupied", ErrAgentPolicyConflict)
	}
	if err != nil {
		return false, fmt.Errorf("verify Agent Runtime promotion grant replay: %w", err)
	}
	existing, err := mapAgentRuntimePromotionGrant(row)
	if err != nil {
		return false, err
	}
	if !sameAgentRuntimePromotionGrant(*existing, grant) {
		return false, fmt.Errorf("%w: promotion grant UUID=%s", ErrAgentPolicyConflict, grant.GrantUUID)
	}
	return false, nil
}

func (r *AgentPolicyRepository) GetActiveRuntimePromotionGrant(ctx context.Context, lookup application.AgentRuntimePromotionGrantLookupV1) (*application.AgentRuntimePromotionGrantV1, error) {
	if strings.TrimSpace(lookup.TenantID) == "" || strings.TrimSpace(lookup.RuntimeID) == "" || strings.TrimSpace(lookup.CandidateVersion) == "" ||
		strings.TrimSpace(lookup.DefinitionUUID) == "" || lookup.DefinitionVersion == 0 || lookup.At.IsZero() {
		return nil, fmt.Errorf("validate Agent Runtime promotion lookup: %w", application.ErrAgentPolicyInvalid)
	}
	row, err := r.queries.GetActiveAgentRuntimePromotionGrant(ctx, generated.GetActiveAgentRuntimePromotionGrantParams{
		TenantID: lookup.TenantID, RuntimeID: lookup.RuntimeID, CandidateVersion: lookup.CandidateVersion,
		DefinitionUuid: lookup.DefinitionUUID, DefinitionVersion: lookup.DefinitionVersion, ValidFrom: lookup.At, ExpiresAt: lookup.At,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get active Agent Runtime promotion grant: %w", err)
	}
	return mapAgentRuntimePromotionGrant(row)
}

func mapAgentRuntimePromotionGrant(row generated.AgentRuntimePromotionGrant) (*application.AgentRuntimePromotionGrantV1, error) {
	grant := &application.AgentRuntimePromotionGrantV1{
		GrantUUID: row.GrantUuid, TenantID: row.TenantID, RuntimeID: row.RuntimeID, CandidateVersion: row.CandidateVersion,
		DefinitionUUID: row.DefinitionUuid, DefinitionVersion: row.DefinitionVersion, PolicyVersion: row.PolicyVersion,
		EvidenceSHA256: row.EvidenceSha256, EvalSuiteSHA256: row.EvalSuiteSha256, GrantedByUUID: row.GrantedByUuid,
		ReviewedByUUID: row.ReviewedByUuid, ValidFrom: row.ValidFrom, ExpiresAt: row.ExpiresAt,
		RevokedAt: timePointer(row.RevokedAt), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	if err := grant.Validate(); err != nil {
		return nil, fmt.Errorf("decode Agent Runtime promotion grant: %w", err)
	}
	return grant, nil
}

func sameAgentRuntimePromotionGrant(left, right application.AgentRuntimePromotionGrantV1) bool {
	return left.GrantUUID == right.GrantUUID && left.TenantID == right.TenantID && left.RuntimeID == right.RuntimeID &&
		left.CandidateVersion == right.CandidateVersion && left.DefinitionUUID == right.DefinitionUUID &&
		left.DefinitionVersion == right.DefinitionVersion && left.PolicyVersion == right.PolicyVersion &&
		left.EvidenceSHA256 == right.EvidenceSHA256 && left.EvalSuiteSHA256 == right.EvalSuiteSHA256 &&
		left.GrantedByUUID == right.GrantedByUUID && left.ReviewedByUUID == right.ReviewedByUUID &&
		left.ValidFrom.Equal(right.ValidFrom) && left.ExpiresAt.Equal(right.ExpiresAt) && left.RevokedAt == nil && right.RevokedAt == nil
}

func (r *AgentPolicyRepository) RevokeRuntimePromotionGrant(ctx context.Context, grantUUID string, revokedAt time.Time) (bool, error) {
	if strings.TrimSpace(grantUUID) == "" || revokedAt.IsZero() {
		return false, fmt.Errorf("validate Agent Runtime promotion revocation: %w", application.ErrAgentPolicyInvalid)
	}
	rows, err := r.queries.RevokeAgentRuntimePromotionGrant(ctx, generated.RevokeAgentRuntimePromotionGrantParams{
		RevokedAt: nullableTime(&revokedAt), GrantUuid: strings.TrimSpace(grantUUID),
	})
	if err != nil {
		return false, fmt.Errorf("revoke Agent Runtime promotion grant: %w", err)
	}
	return rows > 0, nil
}

func (r *AgentPolicyRepository) CreateEventSubscription(ctx context.Context, subscription application.AgentEventSubscriptionV1) (bool, error) {
	if err := subscription.Validate(); err != nil || subscription.Status != application.AgentSubscriptionStatusActive || subscription.RevokedAt != nil {
		return false, fmt.Errorf("validate Agent Event Subscription: %w", application.ErrAgentSubscriptionInvalid)
	}
	rows, err := r.queries.InsertAgentEventSubscription(ctx, generated.InsertAgentEventSubscriptionParams{
		SubscriptionUuid: subscription.SubscriptionUUID, DefinitionUuid: subscription.DefinitionUUID,
		DefinitionVersion: subscription.DefinitionVersion, TenantID: subscription.TenantID, AgentUuid: subscription.AgentUUID,
		Status: string(subscription.Status), EventType: subscription.EventType, ResourceType: subscription.ResourceType,
		ResourceID: subscription.ResourceID, FilterKind: string(subscription.FilterKind), FilterJson: subscription.FilterJSON,
		CreatedByUuid: subscription.CreatedByUUID, RevokedAt: nullableTime(subscription.RevokedAt),
		RevokedByUuid: nullableString(subscription.RevokedByUUID), RevokeReason: nullableString(subscription.RevokeReason),
	})
	if err != nil {
		return false, fmt.Errorf("create Agent Event Subscription: %w", err)
	}
	return rows > 0, nil
}

func (r *AgentPolicyRepository) GetEventSubscription(ctx context.Context, subscriptionUUID string) (*application.AgentEventSubscriptionV1, error) {
	row, err := r.queries.GetAgentEventSubscription(ctx, strings.TrimSpace(subscriptionUUID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get Agent Event Subscription: %w", err)
	}
	return mapAgentEventSubscription(row), nil
}

func (r *AgentPolicyRepository) ListMatchingEventSubscriptions(ctx context.Context, request application.AgentEventSubscriptionMatchRequestV1) ([]application.AgentEventSubscriptionV1, error) {
	rows, err := r.queries.ListMatchingAgentEventSubscriptions(ctx, generated.ListMatchingAgentEventSubscriptionsParams{
		TenantID: request.TenantID, AgentUuid: request.AgentUUID, EventType: request.EventType,
		ResourceType: request.ResourceType, ResourceID: request.ResourceID,
	})
	if err != nil {
		return nil, fmt.Errorf("list matching Agent Event Subscriptions: %w", err)
	}
	result := make([]application.AgentEventSubscriptionV1, 0, len(rows))
	for _, row := range rows {
		result = append(result, *mapAgentEventSubscription(row))
	}
	return result, nil
}

func (r *AgentPolicyRepository) ListOwnedEventSubscriptions(ctx context.Context, tenantID, ownerUUID, afterUUID string, limit int) ([]application.AgentEventSubscriptionV1, error) {
	if limit <= 0 || limit > 101 {
		return nil, application.ErrAgentSubscriptionInvalid
	}
	rows, err := r.queries.ListOwnedAgentEventSubscriptions(ctx, generated.ListOwnedAgentEventSubscriptionsParams{
		TenantID: strings.TrimSpace(tenantID), CreatedByUuid: strings.TrimSpace(ownerUUID), OwnerUuid: strings.TrimSpace(ownerUUID),
		SubscriptionUuid: strings.TrimSpace(afterUUID), Limit: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list owned Agent Event Subscriptions: %w", err)
	}
	result := make([]application.AgentEventSubscriptionV1, 0, len(rows))
	for _, row := range rows {
		result = append(result, *mapAgentEventSubscription(row))
	}
	return result, nil
}

func mapAgentEventSubscription(row generated.AgentEventSubscription) *application.AgentEventSubscriptionV1 {
	return &application.AgentEventSubscriptionV1{
		SubscriptionUUID: row.SubscriptionUuid, DefinitionUUID: row.DefinitionUuid, DefinitionVersion: row.DefinitionVersion,
		TenantID: row.TenantID, AgentUUID: row.AgentUuid, Status: application.AgentSubscriptionStatusV1(row.Status),
		EventType: row.EventType, ResourceType: row.ResourceType, ResourceID: row.ResourceID,
		FilterKind: application.AgentSubscriptionFilterKindV1(row.FilterKind), FilterJSON: row.FilterJson,
		CreatedByUUID: row.CreatedByUuid, RevokedByUUID: row.RevokedByUuid.String, RevokeReason: row.RevokeReason.String,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, RevokedAt: timePointer(row.RevokedAt),
	}
}

func (r *AgentPolicyRepository) RevokeEventSubscription(ctx context.Context, subscriptionUUID, revokedByUUID, reason string, revokedAt time.Time) (bool, error) {
	rows, err := r.queries.RevokeAgentEventSubscription(ctx, generated.RevokeAgentEventSubscriptionParams{
		RevokedAt: nullableTime(&revokedAt), RevokedByUuid: nullableString(revokedByUUID), RevokeReason: nullableString(reason),
		UpdatedAt: revokedAt, SubscriptionUuid: strings.TrimSpace(subscriptionUUID),
	})
	if err != nil {
		return false, fmt.Errorf("revoke Agent Event Subscription: %w", err)
	}
	return rows > 0, nil
}

func (r *AgentPolicyRepository) CreateTask(ctx context.Context, task application.AgentTaskV1) (bool, error) {
	if err := task.Validate(); err != nil {
		return false, fmt.Errorf("validate Agent Task: %w", err)
	}
	insert := func(q generated.Querier) (bool, error) {
		rows, err := q.InsertAgentTask(ctx, generated.InsertAgentTaskParams{
			TaskUuid: task.TaskUUID, DefinitionUuid: task.DefinitionUUID, DefinitionVersion: task.DefinitionVersion,
			TenantID: task.TenantID, PrincipalUuid: task.PrincipalUUID, AgentUuid: task.AgentUUID,
			Status: string(task.Status), TriggerType: task.TriggerType, TriggerRef: task.TriggerRef, Goal: task.Goal,
			TriggerSubscriptionUuid: nullableString(task.TriggerSubscriptionUUID),
		})
		if err != nil {
			return false, fmt.Errorf("create Agent Task: %w", err)
		}
		if rows == 0 {
			return false, nil
		}
		if _, err := appendAgentTaskTimelineEvent(ctx, q, timelineEvent(task.TaskUUID, "", application.AgentTaskTimelineEventTask, string(task.Status))); err != nil {
			return false, err
		}
		return true, nil
	}
	var created bool
	var err error
	if r.store != nil {
		err = r.store.WithinTx(ctx, nil, func(q *generated.Queries) error {
			var callbackErr error
			created, callbackErr = insert(q)
			return callbackErr
		})
	} else {
		created, err = insert(r.queries)
	}
	if err != nil {
		return false, err
	}
	if created {
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
	task := &application.AgentTaskV1{
		TaskUUID: row.TaskUuid, DefinitionUUID: row.DefinitionUuid, DefinitionVersion: row.DefinitionVersion,
		TenantID: row.TenantID, PrincipalUUID: row.PrincipalUuid, AgentUUID: row.AgentUuid,
		Status: application.AgentTaskStatusV1(row.Status), TriggerType: row.TriggerType, TriggerRef: row.TriggerRef,
		TriggerSubscriptionUUID: row.TriggerSubscriptionUuid.String,
		Goal:                    row.Goal, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	if row.WorkflowID.Valid && row.WorkflowRunID.Valid && row.WorkflowStatus.Valid && row.WorkflowRevision.Valid && row.WorkflowUpdatedAt.Valid {
		task.Workflow = &application.AgentTaskWorkflowProjectionV1{
			TaskUUID: row.TaskUuid, WorkflowID: row.WorkflowID.String, RunID: row.WorkflowRunID.String,
			Status: application.AgentTaskWorkflowStatusV1(row.WorkflowStatus.String), Revision: uint64(row.WorkflowRevision.Int64),
			UpdatedAt: row.WorkflowUpdatedAt.Time,
		}
	}
	return task, nil
}

func (r *AgentPolicyRepository) TransitionTaskStatus(ctx context.Context, taskUUID string, from, to application.AgentTaskStatusV1) (bool, error) {
	if err := application.ValidateAgentTaskTransitionV1(from, to); err != nil {
		return false, fmt.Errorf("validate Agent Task transition: %w", err)
	}
	transition := func(q generated.Querier) (bool, error) {
		rows, err := q.TransitionAgentTaskStatus(ctx, generated.TransitionAgentTaskStatusParams{
			Status: string(to), TaskUuid: taskUUID, Status_2: string(from),
		})
		if err != nil {
			return false, fmt.Errorf("transition Agent Task status: %w", err)
		}
		if rows == 0 {
			return false, nil
		}
		if _, err := appendAgentTaskTimelineEvent(ctx, q, timelineEvent(taskUUID, "", application.AgentTaskTimelineEventTask, string(to))); err != nil {
			return false, err
		}
		return true, nil
	}
	var changed bool
	var err error
	if r.store != nil {
		err = r.store.WithinTx(ctx, nil, func(q *generated.Queries) error {
			var callbackErr error
			changed, callbackErr = transition(q)
			return callbackErr
		})
	} else {
		changed, err = transition(r.queries)
	}
	if err != nil {
		return false, err
	}
	return changed, nil
}

func (r *AgentPolicyRepository) ProjectTaskWorkflowState(ctx context.Context, projection application.AgentTaskWorkflowProjectionV1) (bool, error) {
	if err := projection.Validate(); err != nil {
		return false, fmt.Errorf("validate Agent Task Workflow projection: %w", err)
	}
	workflowID := sql.NullString{String: strings.TrimSpace(projection.WorkflowID), Valid: true}
	workflowRunID := sql.NullString{String: strings.TrimSpace(projection.RunID), Valid: true}
	revision := sql.NullInt64{Int64: int64(projection.Revision), Valid: true}
	rows, err := r.queries.ProjectAgentTaskWorkflowState(ctx, generated.ProjectAgentTaskWorkflowStateParams{
		WorkflowID: workflowID, WorkflowRunID: workflowRunID,
		WorkflowStatus: sql.NullString{String: string(projection.Status), Valid: true}, WorkflowRevision: revision,
		TaskUuid: projection.TaskUUID, WorkflowID_2: workflowID, WorkflowRunID_2: workflowRunID, WorkflowRevision_2: revision,
	})
	if err != nil {
		return false, fmt.Errorf("project Agent Task Workflow state: %w", err)
	}
	if rows > 0 {
		return true, nil
	}
	existing, err := r.GetTask(ctx, projection.TaskUUID)
	if err != nil {
		return false, err
	}
	if existing != nil && sameAgentTaskWorkflowProjection(existing.Workflow, &projection) {
		return false, nil
	}
	return false, fmt.Errorf("%w: %w: task_uuid=%s", ErrAgentPolicyConflict, application.ErrAgentWorkflowProjectionConflict, projection.TaskUUID)
}

func (r *AgentPolicyRepository) ListTaskWorkflowProjectionSnapshots(ctx context.Context, runtimeID, mode, afterTaskUUID string, limit int) ([]application.AgentTaskWorkflowProjectionSnapshotV1, error) {
	if strings.TrimSpace(runtimeID) == "" || strings.TrimSpace(mode) == "" || limit < 1 || limit > 1000 {
		return nil, fmt.Errorf("%w: invalid Agent Task Workflow projection page", application.ErrAgentPolicyInvalid)
	}
	rows, err := r.queries.ListAgentTaskWorkflowProjectionSnapshots(ctx, generated.ListAgentTaskWorkflowProjectionSnapshotsParams{
		RuntimeID: strings.TrimSpace(runtimeID), Mode: strings.TrimSpace(mode), TaskUuid: strings.TrimSpace(afterTaskUUID), Limit: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list Agent Task Workflow projection snapshots: %w", err)
	}
	result := make([]application.AgentTaskWorkflowProjectionSnapshotV1, 0, len(rows))
	for _, row := range rows {
		snapshot := application.AgentTaskWorkflowProjectionSnapshotV1{TaskUUID: row.TaskUuid}
		if row.WorkflowID.Valid && row.WorkflowRunID.Valid && row.WorkflowStatus.Valid && row.WorkflowRevision.Valid && row.WorkflowUpdatedAt.Valid {
			snapshot.Workflow = &application.AgentTaskWorkflowProjectionV1{
				TaskUUID: row.TaskUuid, WorkflowID: row.WorkflowID.String, RunID: row.WorkflowRunID.String,
				Status: application.AgentTaskWorkflowStatusV1(row.WorkflowStatus.String), Revision: uint64(row.WorkflowRevision.Int64), UpdatedAt: row.WorkflowUpdatedAt.Time,
			}
		}
		result = append(result, snapshot)
	}
	return result, nil
}

func (r *AgentPolicyRepository) CreateRun(ctx context.Context, run application.AgentRunV1) (bool, error) {
	if err := run.Validate(); err != nil {
		return false, fmt.Errorf("validate Agent Run: %w", err)
	}
	insert := func(q generated.Querier) error {
		_, err := q.InsertAgentRun(ctx, generated.InsertAgentRunParams{
			RunUuid: run.RunUUID, TaskUuid: run.TaskUUID, RuntimeID: run.RuntimeID,
			CandidateVersion: sql.NullString{String: run.CandidateVersion, Valid: run.CandidateVersion != ""}, Mode: run.Mode,
		})
		if err == nil {
			_, err = appendAgentTaskTimelineEvent(ctx, q, timelineEvent(run.TaskUUID, run.RunUUID, application.AgentTaskTimelineEventRun, string(application.AgentRunStatusRunning)))
		}
		return err
	}
	var err error
	if r.store != nil {
		err = r.store.WithinTx(ctx, nil, func(q *generated.Queries) error { return insert(q) })
	} else {
		err = insert(r.queries)
	}
	if err == nil {
		return true, nil
	}
	var duplicate *mysqlDriver.MySQLError
	if !errors.As(err, &duplicate) || duplicate.Number != 1062 {
		return false, fmt.Errorf("create Agent Run: %w", err)
	}
	existing, lookupErr := r.GetRun(ctx, run.RunUUID)
	if lookupErr != nil {
		return false, lookupErr
	}
	if existing == nil || existing.TaskUUID != run.TaskUUID || existing.RuntimeID != run.RuntimeID || existing.CandidateVersion != run.CandidateVersion || existing.Mode != run.Mode {
		return false, fmt.Errorf("%w: run_uuid=%s", ErrAgentPolicyConflict, run.RunUUID)
	}
	return false, nil
}

func (r *AgentPolicyRepository) GetRun(ctx context.Context, runUUID string) (*application.AgentRunV1, error) {
	row, err := r.queries.GetAgentRun(ctx, runUUID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get Agent Run: %w", err)
	}
	return &application.AgentRunV1{
		RunUUID: row.RunUuid, TaskUUID: row.TaskUuid, RuntimeID: row.RuntimeID, CandidateVersion: row.CandidateVersion.String, Mode: row.Mode,
		Status: application.AgentRunStatusV1(row.Status), StartedAt: row.StartedAt,
		CompletedAt: timePointer(row.CompletedAt), LastError: row.LastError.String,
	}, nil
}

func (r *AgentPolicyRepository) TransitionRunStatus(ctx context.Context, runUUID string, from, to application.AgentRunStatusV1, lastError string) (bool, error) {
	if from != application.AgentRunStatusRunning || (to != application.AgentRunStatusCompleted && to != application.AgentRunStatusFailed && to != application.AgentRunStatusCancelled) {
		return false, fmt.Errorf("validate Agent Run transition: %w", application.ErrAgentPolicyInvalid)
	}
	transition := func(q generated.Querier) (bool, error) {
		rows, err := q.TransitionAgentRunStatus(ctx, generated.TransitionAgentRunStatusParams{
			Status: string(to), LastError: sql.NullString{String: lastError, Valid: lastError != ""}, RunUuid: runUUID, Status_2: string(from),
		})
		if err != nil {
			return false, fmt.Errorf("transition Agent Run status: %w", err)
		}
		if rows == 0 {
			return false, nil
		}
		run, err := q.GetAgentRun(ctx, runUUID)
		if err != nil {
			return false, fmt.Errorf("load Agent Run for timeline: %w", err)
		}
		if _, err := appendAgentTaskTimelineEvent(ctx, q, timelineEvent(run.TaskUuid, runUUID, application.AgentTaskTimelineEventRun, string(to))); err != nil {
			return false, err
		}
		return true, nil
	}
	var changed bool
	var err error
	if r.store != nil {
		err = r.store.WithinTx(ctx, nil, func(q *generated.Queries) error {
			var callbackErr error
			changed, callbackErr = transition(q)
			return callbackErr
		})
	} else {
		changed, err = transition(r.queries)
	}
	if err != nil {
		return false, err
	}
	return changed, nil
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
	if err == nil {
		return nil
	}
	var duplicate *mysqlDriver.MySQLError
	if !errors.As(err, &duplicate) || duplicate.Number != 1062 {
		return fmt.Errorf("create Agent Approval: %w", err)
	}
	existing, lookupErr := r.GetApproval(ctx, approval.ApprovalUUID)
	if lookupErr != nil {
		return lookupErr
	}
	if existing == nil || !sameAgentApproval(*existing, approval) {
		return fmt.Errorf("%w: approval_uuid=%s", ErrAgentPolicyConflict, approval.ApprovalUUID)
	}
	return nil
}

func (r *AgentPolicyRepository) GetApproval(ctx context.Context, approvalUUID string) (*application.AgentApprovalV1, error) {
	row, err := r.queries.GetAgentApproval(ctx, approvalUUID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get Agent Approval: %w", err)
	}
	return agentApprovalFromRowV1(row)
}

func (r *AgentPolicyRepository) ListApprovedAgentApprovalGrants(ctx context.Context, taskUUID, capabilityID, scopeSHA256, argumentsSHA256 string, at time.Time, limit int) ([]application.AgentApprovalV1, error) {
	if strings.TrimSpace(taskUUID) == "" || strings.TrimSpace(capabilityID) == "" || len(strings.TrimSpace(scopeSHA256)) != 64 ||
		len(strings.TrimSpace(argumentsSHA256)) != 64 || at.IsZero() || limit < 1 || limit > 2 {
		return nil, fmt.Errorf("validate Agent Approval grant lookup: %w", application.ErrAgentPolicyInvalid)
	}
	rows, err := r.queries.ListApprovedAgentApprovalGrants(ctx, generated.ListApprovedAgentApprovalGrantsParams{
		TaskUuid: taskUUID, CapabilityID: capabilityID, ScopeSha256: scopeSHA256, ArgumentsSha256: argumentsSHA256,
		ExpiresAt: at.UTC(), Limit: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list approved Agent Approval grants: %w", err)
	}
	result := make([]application.AgentApprovalV1, 0, len(rows))
	for _, row := range rows {
		approval, mapErr := agentApprovalFromRowV1(row)
		if mapErr != nil {
			return nil, mapErr
		}
		result = append(result, *approval)
	}
	return result, nil
}

func agentApprovalFromRowV1(row generated.AgentApproval) (*application.AgentApprovalV1, error) {
	var scope application.AgentResourceScopeV1
	if err := json.Unmarshal(row.ResourceScopeJson, &scope); err != nil {
		return nil, fmt.Errorf("decode Agent Approval scope: %w", err)
	}
	return &application.AgentApprovalV1{
		ApprovalUUID: row.ApprovalUuid, TaskUUID: row.TaskUuid, CapabilityID: row.CapabilityID,
		ResourceScope: scope, ScopeSHA256: row.ScopeSha256, ArgumentsSHA256: row.ArgumentsSha256,
		NonceSHA256: row.NonceSha256, Status: application.AgentApprovalStatusV1(row.Status),
		ApprovedByUUID: row.ApprovedByUuid, ExpiresAt: row.ExpiresAt,
		ConsumedAt: timePointer(row.ConsumedAt), RevokedAt: timePointer(row.RevokedAt),
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
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

func (r *AgentPolicyRepository) DenyApproval(ctx context.Context, approvalUUID string, deniedAt time.Time) (bool, error) {
	if strings.TrimSpace(approvalUUID) == "" || deniedAt.IsZero() {
		return false, fmt.Errorf("validate Agent Approval denial: %w", application.ErrAgentPolicyInvalid)
	}
	rows, err := r.queries.DenyAgentApproval(ctx, generated.DenyAgentApprovalParams{
		RevokedAt: nullableTime(&deniedAt), ApprovalUuid: approvalUUID,
	})
	if err != nil {
		return false, fmt.Errorf("deny Agent Approval: %w", err)
	}
	return rows > 0, nil
}

func (r *AgentPolicyRepository) GetWorkflowRepairOperatorGrant(ctx context.Context, userUUID string) (*application.AgentWorkflowRepairOperatorGrantV1, error) {
	row, err := r.queries.GetAgentWorkflowRepairOperatorGrant(ctx, strings.TrimSpace(userUUID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get Workflow repair operator grant: %w", err)
	}
	return &application.AgentWorkflowRepairOperatorGrantV1{UserUUID: row.UserUuid, CanPropose: row.CanPropose, CanApprove: row.CanApprove,
		GrantedByUUID: row.GrantedByUuid, ValidFrom: row.ValidFrom, ExpiresAt: timePointer(row.ExpiresAt), RevokedAt: timePointer(row.RevokedAt)}, nil
}

func (r *AgentPolicyRepository) CreateWorkflowRepairProposal(ctx context.Context, proposal application.AgentWorkflowRepairProposalV1) (bool, error) {
	projected, err := json.Marshal(proposal.Projected)
	if err != nil {
		return false, fmt.Errorf("marshal projected repair evidence: %w", err)
	}
	temporal, err := json.Marshal(proposal.Temporal)
	if err != nil {
		return false, fmt.Errorf("marshal Temporal repair evidence: %w", err)
	}
	rows, err := r.queries.InsertAgentWorkflowRepairProposal(ctx, generated.InsertAgentWorkflowRepairProposalParams{
		ProposalUuid: proposal.ProposalUUID, TaskUuid: proposal.TaskUUID, Outcome: string(proposal.Outcome), ProposerUuid: proposal.ProposerUUID,
		TicketRef: proposal.TicketRef, Reason: proposal.Reason, ProjectedJson: projected, TemporalJson: temporal,
		EvidenceSha256: proposal.EvidenceSHA256, ProposedAt: proposal.ProposedAt, ExpiresAt: proposal.ExpiresAt,
	})
	if err != nil {
		return false, fmt.Errorf("create Workflow repair proposal: %w", err)
	}
	if rows > 0 {
		return true, nil
	}
	existing, err := r.GetWorkflowRepairProposal(ctx, proposal.ProposalUUID)
	if err != nil {
		return false, err
	}
	if existing == nil || !sameWorkflowRepairProposalV1(*existing, proposal) {
		return false, fmt.Errorf("%w: proposal_uuid=%s", ErrAgentPolicyConflict, proposal.ProposalUUID)
	}
	return false, nil
}

func (r *AgentPolicyRepository) GetWorkflowRepairProposal(ctx context.Context, proposalUUID string) (*application.AgentWorkflowRepairProposalV1, error) {
	row, err := r.queries.GetAgentWorkflowRepairProposal(ctx, strings.TrimSpace(proposalUUID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get Workflow repair proposal: %w", err)
	}
	var projected *application.AgentWorkflowEvidenceV1
	if len(row.ProjectedJson) > 0 && string(row.ProjectedJson) != "null" {
		projected = &application.AgentWorkflowEvidenceV1{}
		if err := json.Unmarshal(row.ProjectedJson, projected); err != nil {
			return nil, fmt.Errorf("decode projected repair evidence: %w", err)
		}
	}
	var temporal application.AgentWorkflowEvidenceV1
	if err := json.Unmarshal(row.TemporalJson, &temporal); err != nil {
		return nil, fmt.Errorf("decode Temporal repair evidence: %w", err)
	}
	return &application.AgentWorkflowRepairProposalV1{ProposalUUID: row.ProposalUuid, TaskUUID: row.TaskUuid, Outcome: application.AgentWorkflowRepairOutcomeV1(row.Outcome),
		Action: row.Action, ProposerUUID: row.ProposerUuid, TicketRef: row.TicketRef, Reason: row.Reason, Projected: projected, Temporal: temporal,
		EvidenceSHA256: row.EvidenceSha256, Status: application.AgentWorkflowRepairStatusV1(row.Status), RequiredApprovals: row.RequiredApprovals,
		ProposedAt: row.ProposedAt, ExpiresAt: row.ExpiresAt, DecidedAt: timePointer(row.DecidedAt)}, nil
}

func (r *AgentPolicyRepository) RecordWorkflowRepairDecision(ctx context.Context, decision application.AgentWorkflowRepairDecisionRecordV1) (bool, error) {
	rows, err := r.queries.InsertAgentWorkflowRepairDecision(ctx, generated.InsertAgentWorkflowRepairDecisionParams{
		ProposalUuid: decision.ProposalUUID, ApproverUuid: decision.ApproverUUID, Decision: string(decision.Decision),
		ProposalUuid_2: decision.ProposalUUID, ProposerUuid: decision.ApproverUUID,
	})
	if err != nil {
		return false, fmt.Errorf("record Workflow repair decision: %w", err)
	}
	if rows > 0 {
		return true, nil
	}
	existing, err := r.GetWorkflowRepairDecision(ctx, decision.ProposalUUID, decision.ApproverUUID)
	if err != nil {
		return false, err
	}
	if existing == nil {
		return false, fmt.Errorf("%w: repair decision cannot be recorded", application.ErrAgentWorkflowRepairDenied)
	}
	if existing.Decision != decision.Decision {
		return false, fmt.Errorf("%w: immutable repair decision differs", application.ErrAgentWorkflowRepairConflict)
	}
	return false, nil
}

func (r *AgentPolicyRepository) GetWorkflowRepairDecision(ctx context.Context, proposalUUID, approverUUID string) (*application.AgentWorkflowRepairDecisionRecordV1, error) {
	row, err := r.queries.GetAgentWorkflowRepairDecision(ctx, generated.GetAgentWorkflowRepairDecisionParams{ProposalUuid: proposalUUID, ApproverUuid: approverUUID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get Workflow repair decision: %w", err)
	}
	return &application.AgentWorkflowRepairDecisionRecordV1{ProposalUUID: row.ProposalUuid, ApproverUUID: row.ApproverUuid, Decision: application.AgentWorkflowRepairDecisionV1(row.Decision), DecidedAt: row.DecidedAt}, nil
}

func (r *AgentPolicyRepository) CountWorkflowRepairDecisions(ctx context.Context, proposalUUID string) (application.AgentWorkflowRepairDecisionCountsV1, error) {
	row, err := r.queries.CountAgentWorkflowRepairDecisions(ctx, proposalUUID)
	if err != nil {
		return application.AgentWorkflowRepairDecisionCountsV1{}, fmt.Errorf("count Workflow repair decisions: %w", err)
	}
	return application.AgentWorkflowRepairDecisionCountsV1{Approved: uint64(row.ApprovedCount), Rejected: uint64(row.RejectedCount)}, nil
}

func (r *AgentPolicyRepository) FinalizeWorkflowRepairProposal(ctx context.Context, proposalUUID string) error {
	if _, err := r.queries.ExpireAgentWorkflowRepairProposal(ctx, proposalUUID); err != nil {
		return fmt.Errorf("expire Workflow repair proposal: %w", err)
	}
	if _, err := r.queries.RejectAgentWorkflowRepairProposal(ctx, proposalUUID); err != nil {
		return fmt.Errorf("reject Workflow repair proposal: %w", err)
	}
	if _, err := r.queries.ApproveAgentWorkflowRepairProposal(ctx, proposalUUID); err != nil {
		return fmt.Errorf("approve Workflow repair proposal: %w", err)
	}
	return nil
}

func sameAgentTask(left, right application.AgentTaskV1) bool {
	left.Status, right.Status = "", ""
	left.Workflow, right.Workflow = nil, nil
	left.CreatedAt, left.UpdatedAt = time.Time{}, time.Time{}
	right.CreatedAt, right.UpdatedAt = time.Time{}, time.Time{}
	return reflect.DeepEqual(left, right)
}

func sameWorkflowRepairProposalV1(left, right application.AgentWorkflowRepairProposalV1) bool {
	left.Status, right.Status = "", ""
	left.DecidedAt, right.DecidedAt = nil, nil
	left.ProposedAt, right.ProposedAt = left.ProposedAt.UTC().Truncate(time.Millisecond), right.ProposedAt.UTC().Truncate(time.Millisecond)
	left.ExpiresAt, right.ExpiresAt = left.ExpiresAt.UTC().Truncate(time.Millisecond), right.ExpiresAt.UTC().Truncate(time.Millisecond)
	return reflect.DeepEqual(left, right)
}

func sameAgentTaskWorkflowProjection(left, right *application.AgentTaskWorkflowProjectionV1) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	leftCopy, rightCopy := *left, *right
	leftCopy.UpdatedAt, rightCopy.UpdatedAt = time.Time{}, time.Time{}
	return reflect.DeepEqual(leftCopy, rightCopy)
}

func nullableTime(value *time.Time) sql.NullTime {
	if value == nil || value.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: value.UTC(), Valid: true}
}

func nullableString(value string) sql.NullString {
	value = strings.TrimSpace(value)
	return sql.NullString{String: value, Valid: value != ""}
}

func sameAgentApproval(left, right application.AgentApprovalV1) bool {
	left.CreatedAt, left.UpdatedAt = time.Time{}, time.Time{}
	right.CreatedAt, right.UpdatedAt = time.Time{}, time.Time{}
	return reflect.DeepEqual(left, right)
}

func timePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time
	return &t
}
