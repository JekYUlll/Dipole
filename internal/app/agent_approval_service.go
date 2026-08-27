package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

type PersistentAgentApprovalServiceV1 struct {
	store application.AgentPolicyStoreV1
	now   func() time.Time
}

func NewPersistentAgentApprovalServiceV1(store application.AgentPolicyStoreV1) (*PersistentAgentApprovalServiceV1, error) {
	if store == nil {
		return nil, fmt.Errorf("Agent Policy Store is required")
	}
	return &PersistentAgentApprovalServiceV1{store: store, now: time.Now}, nil
}

func (s *PersistentAgentApprovalServiceV1) Request(ctx context.Context, request application.AgentApprovalRequestV1) (*application.AgentApprovalV1, error) {
	if _, err := s.boundTask(ctx, request.TaskUUID, request.RunUUID, request.RuntimeID, request.Mode); err != nil {
		return nil, err
	}
	approval := request.Approval
	if approval.TaskUUID != strings.TrimSpace(request.TaskUUID) || approval.Status != application.AgentApprovalStatusPending || !s.now().Before(approval.ExpiresAt) || approval.Validate() != nil {
		return nil, fmt.Errorf("%w: invalid pending Approval", application.ErrAgentApprovalDenied)
	}
	existing, err := s.store.GetApproval(ctx, approval.ApprovalUUID)
	if err != nil {
		return nil, fmt.Errorf("get Agent Approval: %w", err)
	}
	if existing != nil {
		if !sameApprovalBindingV1(*existing, approval) {
			return nil, fmt.Errorf("%w: Agent Approval binding conflict", application.ErrAgentApprovalDenied)
		}
		return existing, nil
	}
	if err := s.store.CreateApproval(ctx, approval); err != nil {
		return nil, fmt.Errorf("create Agent Approval: %w", err)
	}
	return &approval, nil
}

func (s *PersistentAgentApprovalServiceV1) Resolve(ctx context.Context, resolution application.AgentApprovalResolutionV1) (*application.AgentApprovalV1, error) {
	task, err := s.boundTask(ctx, resolution.TaskUUID, resolution.RunUUID, resolution.RuntimeID, resolution.Mode)
	if err != nil {
		return nil, err
	}
	actor := strings.TrimSpace(resolution.ActorUUID)
	if actor == "" || actor != task.PrincipalUUID {
		return nil, fmt.Errorf("%w: Approval actor does not match Task principal", application.ErrAgentApprovalDenied)
	}
	approval, err := s.store.GetApproval(ctx, strings.TrimSpace(resolution.ApprovalUUID))
	if err != nil {
		return nil, fmt.Errorf("get Agent Approval: %w", err)
	}
	if approval == nil || approval.TaskUUID != task.TaskUUID {
		return nil, fmt.Errorf("%w: Approval binding mismatch", application.ErrAgentApprovalDenied)
	}
	if resolution.Decision == application.AgentApprovalDecisionApproved && approval.Status == application.AgentApprovalStatusApproved && approval.ApprovedByUUID == actor {
		return approval, nil
	}
	if resolution.Decision == application.AgentApprovalDecisionDenied && approval.Status == application.AgentApprovalStatusRevoked {
		return approval, nil
	}
	if approval.Status != application.AgentApprovalStatusPending || !s.now().Before(approval.ExpiresAt) {
		return nil, fmt.Errorf("%w: Approval cannot transition", application.ErrAgentApprovalDenied)
	}
	switch resolution.Decision {
	case application.AgentApprovalDecisionApproved:
		_, transitionErr := s.store.ApproveApproval(ctx, approval.ApprovalUUID, actor, s.now())
		if transitionErr != nil {
			return nil, fmt.Errorf("approve Agent Approval: %w", transitionErr)
		}
	case application.AgentApprovalDecisionDenied:
		if _, err := s.store.DenyApproval(ctx, approval.ApprovalUUID, s.now()); err != nil {
			return nil, fmt.Errorf("deny Agent Approval: %w", err)
		}
	default:
		return nil, fmt.Errorf("%w: invalid Approval decision", application.ErrAgentApprovalDenied)
	}
	resolved, err := s.store.GetApproval(ctx, approval.ApprovalUUID)
	if err != nil {
		return nil, fmt.Errorf("read resolved Agent Approval: %w", err)
	}
	if resolved == nil {
		return nil, fmt.Errorf("%w: resolved Approval is missing", application.ErrAgentApprovalDenied)
	}
	if resolution.Decision == application.AgentApprovalDecisionApproved && (resolved.Status != application.AgentApprovalStatusApproved || resolved.ApprovedByUUID != actor) {
		return nil, fmt.Errorf("%w: Approval resolution conflict", application.ErrAgentApprovalDenied)
	}
	if resolution.Decision == application.AgentApprovalDecisionDenied && resolved.Status != application.AgentApprovalStatusRevoked {
		return nil, fmt.Errorf("%w: Approval resolution conflict", application.ErrAgentApprovalDenied)
	}
	return resolved, nil
}

func (s *PersistentAgentApprovalServiceV1) Consume(ctx context.Context, consumption application.AgentApprovalConsumptionV1) error {
	if _, err := s.boundTask(ctx, consumption.TaskUUID, consumption.RunUUID, consumption.RuntimeID, consumption.Mode); err != nil {
		return err
	}
	approvalUUID := strings.TrimSpace(consumption.ApprovalUUID)
	if approvalUUID == "" || strings.TrimSpace(consumption.Claim.TaskUUID) != strings.TrimSpace(consumption.TaskUUID) || consumption.Claim.Validate() != nil {
		return fmt.Errorf("%w: invalid Approval consumption", application.ErrAgentApprovalDenied)
	}
	consumed, err := s.store.ConsumeApproval(ctx, approvalUUID, consumption.Claim, s.now())
	if err != nil {
		return fmt.Errorf("consume Agent Approval: %w", err)
	}
	if !consumed {
		return fmt.Errorf("%w: Approval is unavailable or binding changed", application.ErrAgentApprovalDenied)
	}
	return nil
}

func (s *PersistentAgentApprovalServiceV1) boundTask(ctx context.Context, taskUUID, runUUID, runtimeID, mode string) (*application.AgentTaskV1, error) {
	run, err := s.store.GetRun(ctx, strings.TrimSpace(runUUID))
	if err != nil {
		return nil, fmt.Errorf("get Agent Approval Run: %w", err)
	}
	if run == nil || run.TaskUUID != strings.TrimSpace(taskUUID) || run.RuntimeID != strings.TrimSpace(runtimeID) || run.Mode != strings.TrimSpace(mode) || run.Status != application.AgentRunStatusRunning {
		return nil, fmt.Errorf("%w: Approval Run binding mismatch", application.ErrAgentApprovalDenied)
	}
	task, err := s.store.GetTask(ctx, run.TaskUUID)
	if err != nil {
		return nil, fmt.Errorf("get Agent Approval Task: %w", err)
	}
	if task == nil {
		return nil, fmt.Errorf("%w: Approval Task is unavailable", application.ErrAgentApprovalDenied)
	}
	return task, nil
}

func sameApprovalBindingV1(left, right application.AgentApprovalV1) bool {
	return left.ApprovalUUID == right.ApprovalUUID && left.TaskUUID == right.TaskUUID && left.CapabilityID == right.CapabilityID &&
		left.ScopeSHA256 == right.ScopeSHA256 && left.ArgumentsSHA256 == right.ArgumentsSHA256 && left.NonceSHA256 == right.NonceSHA256 && left.ExpiresAt.Equal(right.ExpiresAt)
}
