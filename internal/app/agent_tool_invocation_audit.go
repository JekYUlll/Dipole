package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

type persistentAgentToolInvocationAuditServiceV1 struct {
	store    application.AgentToolInvocationStoreV1
	resolver application.AgentInvocationResolverV1
	now      func() time.Time
}

var _ application.AgentToolInvocationAuditServiceV1 = (*persistentAgentToolInvocationAuditServiceV1)(nil)

func newPersistentAgentToolInvocationAuditServiceV1(store application.AgentToolInvocationStoreV1, resolver application.AgentInvocationResolverV1, now func() time.Time) (*persistentAgentToolInvocationAuditServiceV1, error) {
	if store == nil || resolver == nil || now == nil {
		return nil, errors.New("persistent Agent Tool invocation audit dependencies are required")
	}
	return &persistentAgentToolInvocationAuditServiceV1{store: store, resolver: resolver, now: now}, nil
}

func NewPersistentAgentToolInvocationAuditServiceV1(store application.AgentToolInvocationStoreV1, resolver application.AgentInvocationResolverV1) (application.AgentToolInvocationAuditServiceV1, error) {
	return newPersistentAgentToolInvocationAuditServiceV1(store, resolver, time.Now)
}

func (s *persistentAgentToolInvocationAuditServiceV1) Begin(ctx context.Context, begin application.AgentToolInvocationBeginV1) (*application.AgentToolInvocationV1, error) {
	if err := begin.Validate(); err != nil {
		return nil, err
	}
	begin.InvocationUUID, begin.TaskUUID, begin.RunUUID = strings.TrimSpace(begin.InvocationUUID), strings.TrimSpace(begin.TaskUUID), strings.TrimSpace(begin.RunUUID)
	begin.ToolName, begin.CapabilityID, begin.ArgumentsSHA256 = strings.TrimSpace(begin.ToolName), strings.TrimSpace(begin.CapabilityID), strings.TrimSpace(begin.ArgumentsSHA256)
	begin.RequestID, begin.TraceID = strings.TrimSpace(begin.RequestID), strings.TrimSpace(begin.TraceID)
	invocation, err := s.resolver.Resolve(ctx, begin.TaskUUID, begin.RunUUID)
	if err != nil {
		return nil, fmt.Errorf("%w: invocation context unavailable", application.ErrAgentToolInvocationDenied)
	}
	descriptor, ok := application.AgentCapabilityDescriptorByIDV1(begin.CapabilityID)
	if !ok || descriptor.Risk != application.AgentCapabilityRiskRead || application.AuthorizeAgentCapabilityV1(invocation, descriptor) != nil {
		return nil, fmt.Errorf("%w: capability is unavailable", application.ErrAgentToolInvocationDenied)
	}
	record := application.AgentToolInvocationV1{
		InvocationUUID: begin.InvocationUUID, TenantID: invocation.TenantID, PrincipalUUID: invocation.PrincipalUUID, AgentUUID: invocation.AgentUUID,
		TaskUUID: begin.TaskUUID, RunUUID: begin.RunUUID, Transport: begin.Transport, ToolName: begin.ToolName, CapabilityID: begin.CapabilityID,
		ArgumentsSHA256: begin.ArgumentsSHA256, Status: application.AgentToolInvocationStatusRunning, RequestID: begin.RequestID, TraceID: begin.TraceID,
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

func (s *persistentAgentToolInvocationAuditServiceV1) Finish(ctx context.Context, finish application.AgentToolInvocationFinishV1) error {
	finish.InvocationUUID, finish.TaskUUID, finish.RunUUID = strings.TrimSpace(finish.InvocationUUID), strings.TrimSpace(finish.TaskUUID), strings.TrimSpace(finish.RunUUID)
	finish.ResultSHA256, finish.ErrorCode = strings.TrimSpace(finish.ResultSHA256), strings.TrimSpace(finish.ErrorCode)
	if err := finish.Validate(); err != nil {
		return err
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
