package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math"
	"sort"
	"strings"
	"time"
)

const AgentPolicyPersistenceVersionV1 = "dipole.agent.policy.persistence.v1"
const AgentResourceScopeHashVersionV1 = "dipole.agent.scope.v1"
const AgentRunIDVersionV1 = "dipole.agent.run.v1"

var ErrAgentApprovalDenied = errors.New("agent approval denied")
var ErrAgentPolicyInvalid = errors.New("agent policy record is invalid")
var ErrAgentWorkflowProjectionConflict = errors.New("agent workflow projection conflict")

type AgentDefinitionStatusV1 string

const (
	AgentDefinitionStatusActive  AgentDefinitionStatusV1 = "active"
	AgentDefinitionStatusRevoked AgentDefinitionStatusV1 = "revoked"
)

type AgentTaskStatusV1 string

type AgentTaskWorkflowStatusV1 string

type AgentRunStatusV1 string

const (
	AgentRunStatusRunning   AgentRunStatusV1 = "running"
	AgentRunStatusCompleted AgentRunStatusV1 = "completed"
	AgentRunStatusFailed    AgentRunStatusV1 = "failed"
	AgentRunStatusCancelled AgentRunStatusV1 = "cancelled"
)

const (
	AgentTaskWorkflowStatusCreated         AgentTaskWorkflowStatusV1 = "created"
	AgentTaskWorkflowStatusRunning         AgentTaskWorkflowStatusV1 = "running"
	AgentTaskWorkflowStatusWaitingInput    AgentTaskWorkflowStatusV1 = "waiting_input"
	AgentTaskWorkflowStatusWaitingApproval AgentTaskWorkflowStatusV1 = "waiting_approval"
	AgentTaskWorkflowStatusCompleted       AgentTaskWorkflowStatusV1 = "completed"
	AgentTaskWorkflowStatusFailed          AgentTaskWorkflowStatusV1 = "failed"
	AgentTaskWorkflowStatusCancelled       AgentTaskWorkflowStatusV1 = "cancelled"
)

const (
	AgentTaskStatusCreated         AgentTaskStatusV1 = "created"
	AgentTaskStatusRunning         AgentTaskStatusV1 = "running"
	AgentTaskStatusWaitingApproval AgentTaskStatusV1 = "waiting_approval"
	AgentTaskStatusCompleted       AgentTaskStatusV1 = "completed"
	AgentTaskStatusFailed          AgentTaskStatusV1 = "failed"
	AgentTaskStatusCancelled       AgentTaskStatusV1 = "cancelled"
)

type AgentApprovalStatusV1 string

const (
	AgentApprovalStatusPending  AgentApprovalStatusV1 = "pending"
	AgentApprovalStatusApproved AgentApprovalStatusV1 = "approved"
	AgentApprovalStatusConsumed AgentApprovalStatusV1 = "consumed"
	AgentApprovalStatusRevoked  AgentApprovalStatusV1 = "revoked"
)

type AgentResourceScopeV1 struct {
	ResourceType string   `json:"resource_type"`
	ResourceID   string   `json:"resource_id"`
	Actions      []string `json:"actions"`
}

type AgentDefinitionVersionV1 struct {
	DefinitionUUID string                  `json:"definition_uuid"`
	Version        uint64                  `json:"version"`
	TenantID       string                  `json:"tenant_id"`
	OwnerUUID      string                  `json:"owner_uuid"`
	AgentUUID      string                  `json:"agent_uuid"`
	Status         AgentDefinitionStatusV1 `json:"status"`
	Permissions    []string                `json:"permissions"`
	Scopes         []AgentResourceScopeV1  `json:"scopes"`
	ValidFrom      time.Time               `json:"valid_from"`
	ExpiresAt      *time.Time              `json:"expires_at,omitempty"`
	RevokedAt      *time.Time              `json:"revoked_at,omitempty"`
	CreatedAt      time.Time               `json:"created_at,omitempty"`
	UpdatedAt      time.Time               `json:"updated_at,omitempty"`
}

type AgentTaskV1 struct {
	TaskUUID                string                         `json:"task_uuid"`
	DefinitionUUID          string                         `json:"definition_uuid"`
	DefinitionVersion       uint64                         `json:"definition_version"`
	TenantID                string                         `json:"tenant_id"`
	PrincipalUUID           string                         `json:"principal_uuid"`
	AgentUUID               string                         `json:"agent_uuid"`
	Status                  AgentTaskStatusV1              `json:"status"`
	TriggerType             string                         `json:"trigger_type"`
	TriggerRef              string                         `json:"trigger_ref"`
	TriggerSubscriptionUUID string                         `json:"trigger_subscription_uuid,omitempty"`
	Goal                    string                         `json:"goal"`
	Workflow                *AgentTaskWorkflowProjectionV1 `json:"workflow,omitempty"`
	CreatedAt               time.Time                      `json:"created_at,omitempty"`
	UpdatedAt               time.Time                      `json:"updated_at,omitempty"`
}

type AgentTaskWorkflowProjectionV1 struct {
	TaskUUID   string                    `json:"task_uuid"`
	WorkflowID string                    `json:"workflow_id"`
	RunID      string                    `json:"run_id"`
	Status     AgentTaskWorkflowStatusV1 `json:"status"`
	Revision   uint64                    `json:"revision"`
	UpdatedAt  time.Time                 `json:"updated_at,omitempty"`
}

type AgentTaskWorkflowProjectionRequestV1 struct {
	Projection AgentTaskWorkflowProjectionV1
	RunUUID    string
	RuntimeID  string
	Mode       string
}

type AgentTaskWorkflowProjectionServiceV1 interface {
	Project(ctx context.Context, request AgentTaskWorkflowProjectionRequestV1) (*AgentTaskWorkflowProjectionV1, error)
	ListProjectionSnapshots(ctx context.Context, afterTaskUUID string, limit int) (*AgentTaskWorkflowProjectionPageV1, error)
}

type AgentTaskWorkflowProjectionSnapshotV1 struct {
	TaskUUID string                         `json:"task_uuid"`
	Workflow *AgentTaskWorkflowProjectionV1 `json:"workflow,omitempty"`
}

type AgentTaskWorkflowProjectionReaderV1 interface {
	ListTaskWorkflowProjectionSnapshots(ctx context.Context, runtimeID, mode, afterTaskUUID string, limit int) ([]AgentTaskWorkflowProjectionSnapshotV1, error)
}

type AgentTaskWorkflowProjectionPageV1 struct {
	Tasks      []AgentTaskWorkflowProjectionSnapshotV1 `json:"tasks"`
	NextCursor string                                  `json:"next_cursor,omitempty"`
}

type AgentRunV1 struct {
	RunUUID          string           `json:"run_uuid"`
	TaskUUID         string           `json:"task_uuid"`
	RuntimeID        string           `json:"runtime_id"`
	CandidateVersion string           `json:"candidate_version,omitempty"`
	TraceID          string           `json:"trace_id,omitempty"`
	Mode             string           `json:"mode"`
	Status           AgentRunStatusV1 `json:"status"`
	StartedAt        time.Time        `json:"started_at,omitempty"`
	CompletedAt      *time.Time       `json:"completed_at,omitempty"`
	LastError        string           `json:"last_error,omitempty"`
}

type AgentApprovalV1 struct {
	ApprovalUUID    string                `json:"approval_uuid"`
	TaskUUID        string                `json:"task_uuid"`
	CapabilityID    string                `json:"capability_id"`
	ResourceScope   AgentResourceScopeV1  `json:"resource_scope"`
	ScopeSHA256     string                `json:"scope_sha256"`
	ArgumentsSHA256 string                `json:"arguments_sha256"`
	NonceSHA256     string                `json:"nonce_sha256"`
	Status          AgentApprovalStatusV1 `json:"status"`
	ApprovedByUUID  string                `json:"approved_by_uuid"`
	ExpiresAt       time.Time             `json:"expires_at"`
	ConsumedAt      *time.Time            `json:"consumed_at,omitempty"`
	RevokedAt       *time.Time            `json:"revoked_at,omitempty"`
	CreatedAt       time.Time             `json:"created_at,omitempty"`
	UpdatedAt       time.Time             `json:"updated_at,omitempty"`
}

type AgentApprovalClaimV1 struct {
	TaskUUID        string `json:"task_uuid"`
	CapabilityID    string `json:"capability_id"`
	ScopeSHA256     string `json:"scope_sha256"`
	ArgumentsSHA256 string `json:"arguments_sha256"`
	NonceSHA256     string `json:"nonce_sha256"`
}

type AgentApprovalDecisionV1 string

const (
	AgentApprovalDecisionApproved AgentApprovalDecisionV1 = "approved"
	AgentApprovalDecisionDenied   AgentApprovalDecisionV1 = "denied"
)

type AgentApprovalRequestV1 struct {
	TaskUUID, RunUUID, RuntimeID, Mode string
	Approval                           AgentApprovalV1
}

type AgentApprovalResolutionV1 struct {
	TaskUUID, RunUUID, RuntimeID, Mode string
	ApprovalUUID, ActorUUID            string
	Decision                           AgentApprovalDecisionV1
}

type AgentApprovalConsumptionV1 struct {
	TaskUUID, RunUUID, RuntimeID, Mode string
	ApprovalUUID                       string
	Claim                              AgentApprovalClaimV1
}

type AgentApprovalServiceV1 interface {
	Request(ctx context.Context, request AgentApprovalRequestV1) (*AgentApprovalV1, error)
	Resolve(ctx context.Context, resolution AgentApprovalResolutionV1) (*AgentApprovalV1, error)
	Consume(ctx context.Context, consumption AgentApprovalConsumptionV1) error
}

func (d AgentDefinitionVersionV1) Validate() error {
	if anyBlank(d.DefinitionUUID, d.TenantID, d.OwnerUUID, d.AgentUUID) || d.Version == 0 || d.ValidFrom.IsZero() ||
		(d.Status != AgentDefinitionStatusActive && d.Status != AgentDefinitionStatusRevoked) || len(d.Permissions) == 0 || len(d.Scopes) == 0 {
		return ErrAgentPolicyInvalid
	}
	for _, permission := range d.Permissions {
		if strings.TrimSpace(permission) == "" {
			return ErrAgentPolicyInvalid
		}
	}
	for _, scope := range d.Scopes {
		if !validAgentResourceScopeV1(scope) {
			return ErrAgentPolicyInvalid
		}
	}
	if d.ExpiresAt != nil && !d.ValidFrom.Before(*d.ExpiresAt) {
		return ErrAgentPolicyInvalid
	}
	if (d.Status == AgentDefinitionStatusRevoked) != (d.RevokedAt != nil) {
		return ErrAgentPolicyInvalid
	}
	return nil
}

func (t AgentTaskV1) Validate() error {
	if anyBlank(t.TaskUUID, t.DefinitionUUID, t.TenantID, t.PrincipalUUID, t.AgentUUID, t.TriggerType, t.TriggerRef) || t.DefinitionVersion == 0 {
		return ErrAgentPolicyInvalid
	}
	switch t.Status {
	case AgentTaskStatusCreated, AgentTaskStatusRunning, AgentTaskStatusWaitingApproval, AgentTaskStatusCompleted, AgentTaskStatusFailed, AgentTaskStatusCancelled:
		return nil
	default:
		return ErrAgentPolicyInvalid
	}
}

func (p AgentTaskWorkflowProjectionV1) Validate() error {
	if anyBlank(p.TaskUUID, p.WorkflowID, p.RunID) || len(strings.TrimSpace(p.WorkflowID)) > 255 || len(strings.TrimSpace(p.RunID)) > 64 || p.Revision > math.MaxInt64 {
		return ErrAgentPolicyInvalid
	}
	switch p.Status {
	case AgentTaskWorkflowStatusCreated, AgentTaskWorkflowStatusRunning, AgentTaskWorkflowStatusWaitingInput,
		AgentTaskWorkflowStatusWaitingApproval, AgentTaskWorkflowStatusCompleted, AgentTaskWorkflowStatusFailed,
		AgentTaskWorkflowStatusCancelled:
		return nil
	default:
		return ErrAgentPolicyInvalid
	}
}

func (r AgentRunV1) Validate() error {
	if anyBlank(r.RunUUID, r.TaskUUID, r.RuntimeID, r.Mode) {
		return ErrAgentPolicyInvalid
	}
	if r.Mode != "embedded" && r.Mode != "shadow" && r.Mode != "active" {
		return ErrAgentPolicyInvalid
	}
	if (r.Mode == "active") != (strings.TrimSpace(r.CandidateVersion) != "") || len(r.CandidateVersion) > 128 || len(strings.TrimSpace(r.TraceID)) > 128 {
		return ErrAgentPolicyInvalid
	}
	switch r.Status {
	case AgentRunStatusRunning, AgentRunStatusCompleted, AgentRunStatusFailed, AgentRunStatusCancelled:
		return nil
	default:
		return ErrAgentPolicyInvalid
	}
}

func AgentRunUUIDV1(taskUUID, runtimeID, mode string) (string, error) {
	taskUUID, runtimeID, mode = strings.TrimSpace(taskUUID), strings.TrimSpace(runtimeID), strings.TrimSpace(mode)
	if taskUUID == "" || runtimeID == "" || (mode != "embedded" && mode != "shadow" && mode != "active") {
		return "", ErrAgentPolicyInvalid
	}
	digest := sha256.Sum256([]byte(AgentRunIDVersionV1 + "\n" + taskUUID + "\n" + runtimeID + "\n" + mode))
	return "run:" + hex.EncodeToString(digest[:])[:60], nil
}

func ValidateAgentTaskTransitionV1(from, to AgentTaskStatusV1) error {
	allowed := false
	switch from {
	case AgentTaskStatusCreated:
		allowed = to == AgentTaskStatusRunning || to == AgentTaskStatusCancelled
	case AgentTaskStatusRunning:
		allowed = to == AgentTaskStatusWaitingApproval || to == AgentTaskStatusCompleted || to == AgentTaskStatusFailed || to == AgentTaskStatusCancelled
	case AgentTaskStatusWaitingApproval:
		allowed = to == AgentTaskStatusRunning || to == AgentTaskStatusFailed || to == AgentTaskStatusCancelled
	}
	if !allowed {
		return ErrAgentPolicyInvalid
	}
	return nil
}

func ValidateAgentRunTerminalV1(status AgentRunStatusV1, lastError string) error {
	lastError = strings.TrimSpace(lastError)
	if len(lastError) > 1024 {
		return ErrAgentPolicyInvalid
	}
	switch status {
	case AgentRunStatusCompleted:
		if lastError != "" {
			return ErrAgentPolicyInvalid
		}
	case AgentRunStatusFailed:
		if lastError == "" {
			return ErrAgentPolicyInvalid
		}
	case AgentRunStatusCancelled:
	default:
		return ErrAgentPolicyInvalid
	}
	return nil
}

func (a AgentApprovalV1) Validate() error {
	if anyBlank(a.ApprovalUUID, a.TaskUUID, a.CapabilityID) || !validAgentResourceScopeV1(a.ResourceScope) ||
		!validSHA256V1(a.ScopeSHA256) || !validSHA256V1(a.ArgumentsSHA256) || !validSHA256V1(a.NonceSHA256) || a.ExpiresAt.IsZero() {
		return ErrAgentPolicyInvalid
	}
	scopeHash, err := AgentResourceScopeSHA256V1(a.ResourceScope)
	if err != nil || scopeHash != strings.TrimSpace(a.ScopeSHA256) {
		return ErrAgentPolicyInvalid
	}
	switch a.Status {
	case AgentApprovalStatusPending:
		if strings.TrimSpace(a.ApprovedByUUID) != "" || a.ConsumedAt != nil || a.RevokedAt != nil {
			return ErrAgentPolicyInvalid
		}
	case AgentApprovalStatusApproved:
		if strings.TrimSpace(a.ApprovedByUUID) == "" || a.ConsumedAt != nil || a.RevokedAt != nil {
			return ErrAgentPolicyInvalid
		}
	case AgentApprovalStatusConsumed:
		if strings.TrimSpace(a.ApprovedByUUID) == "" || a.ConsumedAt == nil || a.RevokedAt != nil {
			return ErrAgentPolicyInvalid
		}
	case AgentApprovalStatusRevoked:
		if a.RevokedAt == nil || a.ConsumedAt != nil {
			return ErrAgentPolicyInvalid
		}
	default:
		return ErrAgentPolicyInvalid
	}
	return nil
}

func (a AgentApprovalV1) Authorize(claim AgentApprovalClaimV1, at time.Time) error {
	if err := a.Validate(); err != nil || claim.Validate() != nil || a.Status != AgentApprovalStatusApproved || a.ConsumedAt != nil || a.RevokedAt != nil {
		return ErrAgentApprovalDenied
	}
	if a.ExpiresAt.IsZero() || !at.Before(a.ExpiresAt) {
		return ErrAgentApprovalDenied
	}
	if !sameAgentApprovalBinding(a, claim) {
		return ErrAgentApprovalDenied
	}
	return nil
}

func (c AgentApprovalClaimV1) Validate() error {
	if anyBlank(c.TaskUUID, c.CapabilityID) || !validSHA256V1(c.ScopeSHA256) || !validSHA256V1(c.ArgumentsSHA256) || !validSHA256V1(c.NonceSHA256) {
		return ErrAgentPolicyInvalid
	}
	return nil
}

func AgentResourceScopeSHA256V1(scope AgentResourceScopeV1) (string, error) {
	if !validAgentResourceScopeV1(scope) {
		return "", ErrAgentPolicyInvalid
	}
	actions := make([]string, 0, len(scope.Actions))
	seen := make(map[string]struct{}, len(scope.Actions))
	for _, action := range scope.Actions {
		action = strings.TrimSpace(action)
		if _, exists := seen[action]; exists {
			return "", ErrAgentPolicyInvalid
		}
		seen[action] = struct{}{}
		actions = append(actions, action)
	}
	sort.Strings(actions)
	canonical := AgentResourceScopeHashVersionV1 + "\n" + strings.TrimSpace(scope.ResourceType) + "\n" + strings.TrimSpace(scope.ResourceID) + "\n" + strings.Join(actions, "\n")
	digest := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(digest[:]), nil
}

func validAgentResourceScopeV1(scope AgentResourceScopeV1) bool {
	if anyBlank(scope.ResourceType, scope.ResourceID) || len(scope.Actions) == 0 {
		return false
	}
	seen := make(map[string]struct{}, len(scope.Actions))
	for _, action := range scope.Actions {
		action = strings.TrimSpace(action)
		if action == "" {
			return false
		}
		if _, exists := seen[action]; exists {
			return false
		}
		seen[action] = struct{}{}
	}
	return true
}

func validSHA256V1(value string) bool {
	value = strings.TrimSpace(value)
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32 && value == strings.ToLower(value)
}

func anyBlank(values ...string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return true
		}
	}
	return false
}

func sameAgentApprovalBinding(approval AgentApprovalV1, claim AgentApprovalClaimV1) bool {
	values := []string{
		approval.TaskUUID, approval.CapabilityID, approval.ScopeSHA256, approval.ArgumentsSHA256, approval.NonceSHA256,
		claim.TaskUUID, claim.CapabilityID, claim.ScopeSHA256, claim.ArgumentsSHA256, claim.NonceSHA256,
	}
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return false
		}
	}
	return strings.TrimSpace(approval.TaskUUID) == strings.TrimSpace(claim.TaskUUID) &&
		strings.TrimSpace(approval.CapabilityID) == strings.TrimSpace(claim.CapabilityID) &&
		strings.TrimSpace(approval.ScopeSHA256) == strings.TrimSpace(claim.ScopeSHA256) &&
		strings.TrimSpace(approval.ArgumentsSHA256) == strings.TrimSpace(claim.ArgumentsSHA256) &&
		strings.TrimSpace(approval.NonceSHA256) == strings.TrimSpace(claim.NonceSHA256)
}

type AgentPolicyStoreV1 interface {
	CreateDefinitionVersion(ctx context.Context, definition AgentDefinitionVersionV1) error
	GetLatestDefinition(ctx context.Context, tenantID, ownerUUID, agentUUID string) (*AgentDefinitionVersionV1, error)
	GetDefinitionVersion(ctx context.Context, definitionUUID string, version uint64) (*AgentDefinitionVersionV1, error)
	RevokeDefinitionVersion(ctx context.Context, definitionUUID string, version uint64, revokedAt time.Time) error
	CreateTask(ctx context.Context, task AgentTaskV1) (bool, error)
	GetTask(ctx context.Context, taskUUID string) (*AgentTaskV1, error)
	TransitionTaskStatus(ctx context.Context, taskUUID string, from, to AgentTaskStatusV1) (bool, error)
	ProjectTaskWorkflowState(ctx context.Context, projection AgentTaskWorkflowProjectionV1) (bool, error)
	ListTaskWorkflowProjectionSnapshots(ctx context.Context, runtimeID, mode, afterTaskUUID string, limit int) ([]AgentTaskWorkflowProjectionSnapshotV1, error)
	CreateRun(ctx context.Context, run AgentRunV1) (bool, error)
	GetRun(ctx context.Context, runUUID string) (*AgentRunV1, error)
	TransitionRunStatus(ctx context.Context, runUUID string, from, to AgentRunStatusV1, lastError string) (bool, error)
	CreateApproval(ctx context.Context, approval AgentApprovalV1) error
	GetApproval(ctx context.Context, approvalUUID string) (*AgentApprovalV1, error)
	ApproveApproval(ctx context.Context, approvalUUID, approvedByUUID string, approvedAt time.Time) (bool, error)
	ConsumeApproval(ctx context.Context, approvalUUID string, claim AgentApprovalClaimV1, consumedAt time.Time) (bool, error)
	RevokeApproval(ctx context.Context, approvalUUID string, revokedAt time.Time) error
	DenyApproval(ctx context.Context, approvalUUID string, deniedAt time.Time) (bool, error)
}
