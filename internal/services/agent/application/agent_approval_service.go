package agentapplication

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
)

type PersistentAgentApprovalServiceV1 struct {
	store application.AgentPolicyStoreV1
	now   func() time.Time
}

func NewPersistentAgentApprovalServiceV1(store application.AgentPolicyStoreV1) (*PersistentAgentApprovalServiceV1, error) {
	return NewPersistentAgentApprovalServiceV1WithClock(store, time.Now)
}

// NewPersistentAgentApprovalServiceV1WithClock keeps time-dependent policy tests deterministic.
func NewPersistentAgentApprovalServiceV1WithClock(store application.AgentPolicyStoreV1, now func() time.Time) (*PersistentAgentApprovalServiceV1, error) {
	if store == nil || now == nil {
		return nil, fmt.Errorf("Agent Policy Store is required")
	}
	return &PersistentAgentApprovalServiceV1{store: store, now: now}, nil
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

// AutoApproveSubscriptionMessage replaces the owner Signal for an autonomous
// subscription reply. Core re-verifies that the Task is subscription-triggered,
// the pinned Definition still authorizes message writes for this owner, the
// Subscription is owner-consistent, and the scope targets exactly the owner's
// direct Agent conversation, then persists an already-approved grant. Idempotent
// on an identical binding so a retried reply converges on one grant.
func (s *PersistentAgentApprovalServiceV1) AutoApproveSubscriptionMessage(ctx context.Context, request application.AgentApprovalRequestV1) (*application.AgentApprovalV1, error) {
	if strings.TrimSpace(request.Mode) != "active" {
		return nil, fmt.Errorf("%w: subscription message approval requires active mode", application.ErrAgentApprovalDenied)
	}
	task, err := s.boundTask(ctx, request.TaskUUID, request.RunUUID, request.RuntimeID, request.Mode)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(task.TriggerSubscriptionUUID) == "" {
		return nil, fmt.Errorf("%w: Task is not subscription-triggered", application.ErrAgentApprovalDenied)
	}
	binding := request.Approval
	if binding.TaskUUID != strings.TrimSpace(request.TaskUUID) || binding.CapabilityID != application.AgentCapabilitySystemMessageSend {
		return nil, fmt.Errorf("%w: subscription message approval binding is invalid", application.ErrAgentApprovalDenied)
	}
	expectedScope := application.AgentResourceScopeV1{
		ResourceType: application.AgentResourceTypeConversation,
		ResourceID:   model.DirectConversationKey(task.PrincipalUUID, task.AgentUUID),
		Actions:      []string{application.AgentResourceActionWrite},
	}
	expectedScopeSHA256, err := application.AgentResourceScopeSHA256V1(expectedScope)
	if err != nil {
		return nil, fmt.Errorf("derive subscription message scope: %w", err)
	}
	if strings.TrimSpace(binding.ScopeSHA256) != expectedScopeSHA256 {
		return nil, fmt.Errorf("%w: subscription message scope must target the owner Agent conversation", application.ErrAgentApprovalDenied)
	}
	definition, err := s.store.GetDefinitionVersion(ctx, task.DefinitionUUID, task.DefinitionVersion)
	if err != nil || definition == nil {
		return nil, fmt.Errorf("%w: pinned Agent Definition unavailable", application.ErrAgentApprovalDenied)
	}
	if strings.TrimSpace(definition.OwnerUUID) != strings.TrimSpace(task.PrincipalUUID) {
		return nil, fmt.Errorf("%w: Agent Definition owner binding is invalid", application.ErrAgentApprovalDenied)
	}
	capabilities, err := application.ProjectAgentApprovedCapabilitiesV1(*definition)
	if err != nil || !containsStringV1(capabilities, application.AgentCapabilitySystemMessageSend) {
		return nil, fmt.Errorf("%w: pinned Agent Definition does not authorize message writes", application.ErrAgentApprovalDenied)
	}
	reader, ok := s.store.(agentEventSubscriptionReaderV1)
	if !ok {
		return nil, fmt.Errorf("%w: Agent Event Subscription reader unavailable", application.ErrAgentApprovalDenied)
	}
	subscription, err := reader.GetEventSubscription(ctx, strings.TrimSpace(task.TriggerSubscriptionUUID))
	if err != nil || subscription == nil || strings.TrimSpace(subscription.CreatedByUUID) != strings.TrimSpace(task.PrincipalUUID) {
		return nil, fmt.Errorf("%w: Agent Event Subscription owner binding is invalid", application.ErrAgentApprovalDenied)
	}
	approval := binding
	approval.ResourceScope = expectedScope
	approval.Status = application.AgentApprovalStatusApproved
	approval.ApprovedByUUID = task.PrincipalUUID
	approval.ConsumedAt = nil
	approval.RevokedAt = nil
	if !s.now().Before(approval.ExpiresAt) || approval.Validate() != nil {
		return nil, fmt.Errorf("%w: invalid subscription message Approval", application.ErrAgentApprovalDenied)
	}
	existing, err := s.store.GetApproval(ctx, approval.ApprovalUUID)
	if err != nil {
		return nil, fmt.Errorf("get Agent Approval: %w", err)
	}
	if existing != nil {
		if !sameApprovalBindingV1(*existing, approval) || existing.ApprovedByUUID != approval.ApprovedByUUID {
			return nil, fmt.Errorf("%w: subscription message Approval binding conflict", application.ErrAgentApprovalDenied)
		}
		return existing, nil
	}
	if err := s.store.CreateApproval(ctx, approval); err != nil {
		return nil, fmt.Errorf("create Agent Approval: %w", err)
	}
	return &approval, nil
}

// AutoApproveInteractiveReply mints an already-approved assistant-reply grant for
// an interactive Agent task whose owner is the requesting principal. Unlike the
// human-in-the-loop /send path, a self-initiated interactive reply needs no owner
// Signal: the owner is the requester. Core still re-verifies the Task is
// interactive-triggered (never subscription-triggered), the pinned Definition
// authorizes assistant-reply writes for this owner, and the scope targets exactly
// the owner's direct Agent conversation, then persists an already-approved grant.
// Idempotent on an identical binding so a retried reply converges on one grant.
func (s *PersistentAgentApprovalServiceV1) AutoApproveInteractiveReply(ctx context.Context, request application.AgentApprovalRequestV1) (*application.AgentApprovalV1, error) {
	if strings.TrimSpace(request.Mode) != "active" {
		return nil, fmt.Errorf("%w: interactive reply approval requires active mode", application.ErrAgentApprovalDenied)
	}
	task, err := s.boundTask(ctx, request.TaskUUID, request.RunUUID, request.RuntimeID, request.Mode)
	if err != nil {
		return nil, err
	}
	if task.TriggerType != interactiveAgentTriggerTypeV1 || strings.TrimSpace(task.TriggerSubscriptionUUID) != "" {
		return nil, fmt.Errorf("%w: Task is not interactive-triggered", application.ErrAgentApprovalDenied)
	}
	binding := request.Approval
	if binding.TaskUUID != strings.TrimSpace(request.TaskUUID) || binding.CapabilityID != application.AgentCapabilityAssistantReplySend {
		return nil, fmt.Errorf("%w: interactive reply approval binding is invalid", application.ErrAgentApprovalDenied)
	}
	expectedScope := application.AgentResourceScopeV1{
		ResourceType: application.AgentResourceTypeConversation,
		ResourceID:   model.DirectConversationKey(task.PrincipalUUID, task.AgentUUID),
		Actions:      []string{application.AgentResourceActionWrite},
	}
	expectedScopeSHA256, err := application.AgentResourceScopeSHA256V1(expectedScope)
	if err != nil {
		return nil, fmt.Errorf("derive interactive reply scope: %w", err)
	}
	if strings.TrimSpace(binding.ScopeSHA256) != expectedScopeSHA256 {
		return nil, fmt.Errorf("%w: interactive reply scope must target the owner Agent conversation", application.ErrAgentApprovalDenied)
	}
	definition, err := s.store.GetDefinitionVersion(ctx, task.DefinitionUUID, task.DefinitionVersion)
	if err != nil || definition == nil {
		return nil, fmt.Errorf("%w: pinned Agent Definition unavailable", application.ErrAgentApprovalDenied)
	}
	if strings.TrimSpace(definition.OwnerUUID) != strings.TrimSpace(task.PrincipalUUID) {
		return nil, fmt.Errorf("%w: Agent Definition owner binding is invalid", application.ErrAgentApprovalDenied)
	}
	capabilities, err := application.ProjectAgentApprovedCapabilitiesV1(*definition)
	if err != nil || !containsStringV1(capabilities, application.AgentCapabilityAssistantReplySend) {
		return nil, fmt.Errorf("%w: pinned Agent Definition does not authorize assistant replies", application.ErrAgentApprovalDenied)
	}
	approval := binding
	approval.ResourceScope = expectedScope
	approval.Status = application.AgentApprovalStatusApproved
	approval.ApprovedByUUID = task.PrincipalUUID
	approval.ConsumedAt = nil
	approval.RevokedAt = nil
	if !s.now().Before(approval.ExpiresAt) || approval.Validate() != nil {
		return nil, fmt.Errorf("%w: invalid interactive reply Approval", application.ErrAgentApprovalDenied)
	}
	existing, err := s.store.GetApproval(ctx, approval.ApprovalUUID)
	if err != nil {
		return nil, fmt.Errorf("get Agent Approval: %w", err)
	}
	if existing != nil {
		if !sameApprovalBindingV1(*existing, approval) || existing.ApprovedByUUID != approval.ApprovedByUUID {
			return nil, fmt.Errorf("%w: interactive reply Approval binding conflict", application.ErrAgentApprovalDenied)
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
	requestedMode := strings.TrimSpace(mode)
	// The persisted Run is authoritative when an older RPC omits its mode.
	if run == nil || run.TaskUUID != strings.TrimSpace(taskUUID) || run.RuntimeID != strings.TrimSpace(runtimeID) || (requestedMode != "" && run.Mode != requestedMode) || run.Status != application.AgentRunStatusRunning {
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

func containsStringV1(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func sameApprovalBindingV1(left, right application.AgentApprovalV1) bool {
	return left.ApprovalUUID == right.ApprovalUUID && left.TaskUUID == right.TaskUUID && left.CapabilityID == right.CapabilityID &&
		left.ScopeSHA256 == right.ScopeSHA256 && left.ArgumentsSHA256 == right.ArgumentsSHA256 && left.NonceSHA256 == right.NonceSHA256 && left.ExpiresAt.Equal(right.ExpiresAt)
}
