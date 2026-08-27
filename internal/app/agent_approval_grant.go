package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

const agentApprovalGrantCandidateLimitV1 = 2

type PersistentAgentApprovalGrantResolverV1 struct {
	store application.AgentApprovalGrantStoreV1
	now   func() time.Time
}

func NewPersistentAgentApprovalGrantResolverV1(store application.AgentApprovalGrantStoreV1) (*PersistentAgentApprovalGrantResolverV1, error) {
	return newPersistentAgentApprovalGrantResolverV1(store, time.Now)
}

func newPersistentAgentApprovalGrantResolverV1(store application.AgentApprovalGrantStoreV1, now func() time.Time) (*PersistentAgentApprovalGrantResolverV1, error) {
	if store == nil || now == nil {
		return nil, fmt.Errorf("Agent Approval grant store and clock are required")
	}
	return &PersistentAgentApprovalGrantResolverV1{store: store, now: now}, nil
}

func (r *PersistentAgentApprovalGrantResolverV1) ResolveGrant(ctx context.Context, request application.AgentApprovalGrantRequestV1) (*application.AgentApprovalV1, error) {
	taskUUID, runUUID := strings.TrimSpace(request.TaskUUID), strings.TrimSpace(request.RunUUID)
	runtimeID, mode := strings.TrimSpace(request.RuntimeID), strings.TrimSpace(request.Mode)
	capabilityID, argumentsSHA256 := strings.TrimSpace(request.CapabilityID), strings.TrimSpace(request.ArgumentsSHA256)
	if taskUUID == "" || runUUID == "" || runtimeID == "" || mode != "active" || capabilityID == "" || len(argumentsSHA256) != 64 {
		return nil, fmt.Errorf("%w: invalid Approval grant request", application.ErrAgentApprovalDenied)
	}
	scopeSHA256, err := application.AgentResourceScopeSHA256V1(request.ResourceScope)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid Approval grant scope", application.ErrAgentApprovalDenied)
	}
	run, err := r.store.GetRun(ctx, runUUID)
	if err != nil {
		return nil, fmt.Errorf("get Agent Approval grant Run: %w", err)
	}
	if run == nil || run.TaskUUID != taskUUID || run.RuntimeID != runtimeID || run.Mode != mode || run.Status != application.AgentRunStatusRunning {
		return nil, fmt.Errorf("%w: Approval grant Run binding mismatch", application.ErrAgentApprovalDenied)
	}
	task, err := r.store.GetTask(ctx, taskUUID)
	if err != nil {
		return nil, fmt.Errorf("get Agent Approval grant Task: %w", err)
	}
	if task == nil || task.Status != application.AgentTaskStatusRunning {
		return nil, fmt.Errorf("%w: Approval grant Task is unavailable", application.ErrAgentApprovalDenied)
	}
	at := r.now().UTC()
	grants, err := r.store.ListApprovedAgentApprovalGrants(ctx, taskUUID, capabilityID, scopeSHA256, argumentsSHA256, at, agentApprovalGrantCandidateLimitV1)
	if err != nil {
		return nil, fmt.Errorf("list Agent Approval grants: %w", err)
	}
	if len(grants) != 1 {
		return nil, fmt.Errorf("%w: exact Approval grant is unavailable", application.ErrAgentApprovalDenied)
	}
	grant := grants[0]
	if grant.Validate() != nil || grant.TaskUUID != taskUUID || grant.CapabilityID != capabilityID || grant.ScopeSHA256 != scopeSHA256 ||
		grant.ArgumentsSHA256 != argumentsSHA256 || grant.Status != application.AgentApprovalStatusApproved || grant.ConsumedAt != nil || grant.RevokedAt != nil ||
		grant.ApprovedByUUID != task.PrincipalUUID || !at.Before(grant.ExpiresAt) {
		return nil, fmt.Errorf("%w: Approval grant binding changed", application.ErrAgentApprovalDenied)
	}
	return &grant, nil
}
