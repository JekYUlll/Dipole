package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
)

type persistentAgentToolInvocationAuditServiceV1 struct {
	store     application.AgentToolInvocationStoreV1
	resolver  application.AgentInvocationResolverV1
	approvals application.AgentToolApprovalReaderV1
	receipts  application.MessageCommandReceiptQuery
	now       func() time.Time
}

var _ application.AgentToolInvocationAuditServiceV1 = (*persistentAgentToolInvocationAuditServiceV1)(nil)

func newPersistentAgentToolInvocationAuditServiceV1(store application.AgentToolInvocationStoreV1, resolver application.AgentInvocationResolverV1, approvals application.AgentToolApprovalReaderV1, receipts application.MessageCommandReceiptQuery, now func() time.Time) (*persistentAgentToolInvocationAuditServiceV1, error) {
	if store == nil || resolver == nil || approvals == nil || receipts == nil || now == nil {
		return nil, errors.New("persistent Agent Tool invocation audit dependencies are required")
	}
	return &persistentAgentToolInvocationAuditServiceV1{store: store, resolver: resolver, approvals: approvals, receipts: receipts, now: now}, nil
}

func NewPersistentAgentToolInvocationAuditServiceV1(store application.AgentToolInvocationStoreV1, resolver application.AgentInvocationResolverV1, approvals application.AgentToolApprovalReaderV1, receipts application.MessageCommandReceiptQuery) (application.AgentToolInvocationAuditServiceV1, error) {
	return newPersistentAgentToolInvocationAuditServiceV1(store, resolver, approvals, receipts, time.Now)
}

func (s *persistentAgentToolInvocationAuditServiceV1) Begin(ctx context.Context, begin application.AgentToolInvocationBeginV1) (*application.AgentToolInvocationV1, error) {
	if err := begin.Validate(); err != nil {
		return nil, err
	}
	begin.InvocationUUID, begin.TaskUUID, begin.RunUUID = strings.TrimSpace(begin.InvocationUUID), strings.TrimSpace(begin.TaskUUID), strings.TrimSpace(begin.RunUUID)
	begin.ToolName, begin.CapabilityID, begin.ArgumentsSHA256 = strings.TrimSpace(begin.ToolName), strings.TrimSpace(begin.CapabilityID), strings.TrimSpace(begin.ArgumentsSHA256)
	begin.RequestID, begin.TraceID, begin.ApprovalUUID = strings.TrimSpace(begin.RequestID), strings.TrimSpace(begin.TraceID), strings.TrimSpace(begin.ApprovalUUID)
	invocation, err := s.resolver.Resolve(ctx, begin.TaskUUID, begin.RunUUID)
	if err != nil {
		return nil, fmt.Errorf("%w: invocation context unavailable", application.ErrAgentToolInvocationDenied)
	}
	descriptor, ok := application.AgentCapabilityDescriptorByIDV1(begin.CapabilityID)
	if !ok || application.AuthorizeAgentCapabilityV1(invocation, descriptor) != nil {
		return nil, fmt.Errorf("%w: capability is unavailable", application.ErrAgentToolInvocationDenied)
	}
	if descriptor.Risk == application.AgentCapabilityRiskRead {
		if begin.ApprovalUUID != "" {
			return nil, fmt.Errorf("%w: read capability cannot bind an approval", application.ErrAgentToolInvocationDenied)
		}
	} else if err := s.authorizeWriteApproval(ctx, begin, invocation, descriptor); err != nil {
		return nil, err
	}
	record := application.AgentToolInvocationV1{
		InvocationUUID: begin.InvocationUUID, TenantID: invocation.TenantID, PrincipalUUID: invocation.PrincipalUUID, AgentUUID: invocation.AgentUUID,
		TaskUUID: begin.TaskUUID, RunUUID: begin.RunUUID, Transport: begin.Transport, ToolName: begin.ToolName, CapabilityID: begin.CapabilityID,
		ArgumentsSHA256: begin.ArgumentsSHA256, ProfileID: begin.ProfileID, ServerID: begin.ServerID, ArgumentsJSON: begin.ArgumentsJSON,
		Status: application.AgentToolInvocationStatusRunning, RequestID: begin.RequestID, TraceID: begin.TraceID, ApprovalUUID: begin.ApprovalUUID,
		StartedAt: s.now().UTC(),
	}
	created, err := s.store.BeginToolInvocation(ctx, record)
	if err != nil {
		return nil, fmt.Errorf("begin Agent Tool invocation: %w", err)
	}
	if !created {
		return nil, application.ErrAgentToolInvocationConflict
	}
	return &record, nil
}

func (s *persistentAgentToolInvocationAuditServiceV1) ResolveCommand(ctx context.Context, taskUUID, runUUID, invocationUUID string) (*application.AgentMCPToolCommandV1, error) {
	taskUUID, runUUID, invocationUUID = strings.TrimSpace(taskUUID), strings.TrimSpace(runUUID), strings.TrimSpace(invocationUUID)
	if !validToolCommandLookupV1(taskUUID, runUUID, invocationUUID) {
		return nil, application.ErrAgentToolInvocationInvalid
	}
	record, err := s.store.GetToolInvocation(ctx, invocationUUID)
	if err != nil {
		return nil, fmt.Errorf("load Agent Tool command: %w", err)
	}
	if record == nil || record.TaskUUID != taskUUID || record.RunUUID != runUUID || record.Status != application.AgentToolInvocationStatusRunning ||
		record.StartedAt.IsZero() || application.ValidateAgentMCPToolCommandV1(record.ProfileID, record.ServerID, record.ArgumentsJSON, record.ArgumentsSHA256) != nil {
		return nil, application.ErrAgentToolInvocationDenied
	}
	return &application.AgentMCPToolCommandV1{
		InvocationUUID: record.InvocationUUID, TenantID: record.TenantID, PrincipalUUID: record.PrincipalUUID, AgentUUID: record.AgentUUID,
		TaskUUID: record.TaskUUID, RunUUID: record.RunUUID, ProfileID: record.ProfileID, ServerID: record.ServerID,
		ToolName: record.ToolName, CapabilityID: record.CapabilityID, ArgumentsJSON: record.ArgumentsJSON, ArgumentsSHA256: record.ArgumentsSHA256,
		StartedAt: record.StartedAt,
	}, nil
}

func validToolCommandLookupV1(values ...string) bool {
	for _, value := range values {
		if value == "" || len(value) > 64 {
			return false
		}
	}
	return true
}

func (s *persistentAgentToolInvocationAuditServiceV1) Finish(ctx context.Context, finish application.AgentToolInvocationFinishV1) error {
	finish.InvocationUUID, finish.TaskUUID, finish.RunUUID = strings.TrimSpace(finish.InvocationUUID), strings.TrimSpace(finish.TaskUUID), strings.TrimSpace(finish.RunUUID)
	finish.ResultSHA256, finish.ErrorCode = strings.TrimSpace(finish.ResultSHA256), strings.TrimSpace(finish.ErrorCode)
	if finish.ActionReference != nil {
		reference := *finish.ActionReference
		reference.ResourceUUID, reference.CommandID = strings.TrimSpace(reference.ResourceUUID), strings.TrimSpace(reference.CommandID)
		finish.ActionReference = &reference
	}
	if err := finish.Validate(); err != nil {
		return err
	}
	invocation, err := s.store.GetToolInvocation(ctx, finish.InvocationUUID)
	if err != nil {
		return fmt.Errorf("load Agent Tool invocation: %w", err)
	}
	if invocation == nil || invocation.TaskUUID != finish.TaskUUID || invocation.RunUUID != finish.RunUUID || invocation.Status != application.AgentToolInvocationStatusRunning {
		return application.ErrAgentToolInvocationConflict
	}
	descriptor, ok := application.AgentCapabilityDescriptorByIDV1(invocation.CapabilityID)
	if !ok {
		return application.ErrAgentToolInvocationConflict
	}
	if descriptor.Risk == application.AgentCapabilityRiskRead {
		if finish.ActionReference != nil {
			return application.ErrAgentToolInvocationConflict
		}
	} else if finish.Status == application.AgentToolInvocationStatusCompleted {
		if err := s.verifyMessageActionReference(invocation, finish.ActionReference); err != nil {
			return err
		}
	}
	changed, err := s.store.FinishToolInvocation(ctx, finish)
	if err != nil {
		return fmt.Errorf("finish Agent Tool invocation: %w", err)
	}
	if !changed {
		return application.ErrAgentToolInvocationConflict
	}
	return nil
}

func (s *persistentAgentToolInvocationAuditServiceV1) authorizeWriteApproval(ctx context.Context, begin application.AgentToolInvocationBeginV1, invocation application.AgentInvocationV1, descriptor application.AgentCapabilityDescriptorV1) error {
	if descriptor.Risk != application.AgentCapabilityRiskWrite || begin.ApprovalUUID == "" {
		return fmt.Errorf("%w: write approval is required", application.ErrAgentToolInvocationDenied)
	}
	run, err := s.approvals.GetRun(ctx, begin.RunUUID)
	if err != nil || run == nil || run.RunUUID != begin.RunUUID || run.TaskUUID != begin.TaskUUID || run.RuntimeID != "dipole-agent" || run.Mode != "active" || run.Status != application.AgentRunStatusRunning {
		return fmt.Errorf("%w: active Agent Run is unavailable", application.ErrAgentToolInvocationDenied)
	}
	approval, err := s.approvals.GetApproval(ctx, begin.ApprovalUUID)
	if err != nil || approval == nil || approval.ApprovalUUID != begin.ApprovalUUID || approval.TaskUUID != begin.TaskUUID ||
		approval.CapabilityID != begin.CapabilityID || approval.ArgumentsSHA256 != begin.ArgumentsSHA256 || approval.Status != application.AgentApprovalStatusConsumed || approval.ConsumedAt == nil ||
		len(approval.ResourceScope.Actions) != 1 || approval.ResourceScope.Actions[0] != application.AgentResourceActionWrite {
		return fmt.Errorf("%w: consumed approval binding is unavailable", application.ErrAgentToolInvocationDenied)
	}
	if err := application.AuthorizeAgentCapabilityForResourceV1(invocation, descriptor, approval.ResourceScope.ResourceType, approval.ResourceScope.ResourceID, approval.ResourceScope.Actions[0]); err != nil {
		return fmt.Errorf("%w: approved resource is unavailable", application.ErrAgentToolInvocationDenied)
	}
	return nil
}

func (s *persistentAgentToolInvocationAuditServiceV1) verifyMessageActionReference(invocation *application.AgentToolInvocationV1, reference *application.AgentToolActionReferenceV1) error {
	if reference == nil || reference.Validate() != nil || invocation.ApprovalUUID == "" {
		return application.ErrAgentToolInvocationConflict
	}
	wantCapability := application.AgentCapabilityAssistantReplySend
	wantType := int8(model.MessageTypeAIText)
	if reference.CommandKind == application.AgentMessageCommandSystemMessageV1 {
		wantCapability = application.AgentCapabilitySystemMessageSend
		wantType = model.MessageTypeSystem
	}
	if invocation.CapabilityID != wantCapability {
		return application.ErrAgentToolInvocationConflict
	}
	clientMessageID, err := application.AgentCommandClientMessageIDV1(reference.CommandKind, reference.CommandID)
	if err != nil {
		return application.ErrAgentToolInvocationConflict
	}
	receipt, err := s.receipts.GetMessageCommandReceipt(invocation.AgentUUID, clientMessageID)
	if err != nil {
		return fmt.Errorf("verify Agent Tool Message receipt: %w", err)
	}
	if receipt == nil {
		return application.ErrAgentToolInvocationConflict
	}
	message := receipt.Message
	if receipt.Status != application.MessageCommandReceiptStatusCommitted || message == nil || message.UUID != strings.TrimSpace(reference.ResourceUUID) ||
		message.ClientMessageID != clientMessageID || message.SenderUUID != invocation.AgentUUID || message.TargetUUID != invocation.PrincipalUUID ||
		message.TargetType != model.MessageTargetDirect || message.ConversationKey != model.DirectConversationKey(invocation.AgentUUID, invocation.PrincipalUUID) || message.MessageType != wantType {
		return application.ErrAgentToolInvocationConflict
	}
	return nil
}
