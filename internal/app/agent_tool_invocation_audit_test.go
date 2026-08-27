package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

type agentToolAuditStoreStub struct {
	begun     application.AgentToolInvocationV1
	finished  application.AgentToolInvocationFinishV1
	beginErr  error
	finishErr error
}

func (s *agentToolAuditStoreStub) BeginToolInvocation(_ context.Context, invocation application.AgentToolInvocationV1) (bool, error) {
	s.begun = invocation
	return s.beginErr == nil, s.beginErr
}

func (s *agentToolAuditStoreStub) FinishToolInvocation(_ context.Context, finish application.AgentToolInvocationFinishV1) (bool, error) {
	s.finished = finish
	return s.finishErr == nil, s.finishErr
}

type agentToolAuditResolverStub struct {
	invocation application.AgentInvocationV1
	err        error
}

func (s agentToolAuditResolverStub) Resolve(context.Context, string, string) (application.AgentInvocationV1, error) {
	return s.invocation, s.err
}

func TestPersistentAgentToolInvocationAuditBindsAuthoritativeInvocation(t *testing.T) {
	store := &agentToolAuditStoreStub{}
	service, err := newPersistentAgentToolInvocationAuditServiceV1(store, agentToolAuditResolverStub{invocation: application.AgentInvocationV1{
		TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", DelegatedByUUID: "U100",
		Permissions:    []string{application.AgentPermissionConversationList},
		ResourceScopes: []application.AgentResourceScopeV1{{ResourceType: "conversation", ResourceID: "*", Actions: []string{"list"}}},
	}}, func() time.Time { return time.UnixMilli(1000) })
	if err != nil {
		t.Fatalf("new audit service: %v", err)
	}
	record, err := service.Begin(context.Background(), application.AgentToolInvocationBeginV1{
		InvocationUUID: "INV-1", TaskUUID: "TASK-1", RunUUID: "RUN-1", Transport: application.AgentToolTransportMCP,
		ToolName: "dipole_conversation_list", CapabilityID: application.AgentCapabilityConversationsList,
		ArgumentsSHA256: testAuditSHA, RequestID: "REQ-1", TraceID: "TRACE-1",
	})
	if err != nil {
		t.Fatalf("begin invocation: %v", err)
	}
	if record.PrincipalUUID != "U100" || store.begun.AgentUUID != "UAI" || store.begun.Status != application.AgentToolInvocationStatusRunning || !store.begun.StartedAt.Equal(time.UnixMilli(1000)) {
		t.Fatalf("unexpected authoritative audit: record=%+v stored=%+v", record, store.begun)
	}
}

func TestPersistentAgentToolInvocationAuditRejectsWriteCapabilityAndResolverFailure(t *testing.T) {
	store := &agentToolAuditStoreStub{}
	invocation := application.AgentInvocationV1{
		TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI",
		Permissions:    []string{application.AgentPermissionMessageWrite},
		ResourceScopes: []application.AgentResourceScopeV1{{ResourceType: "conversation", ResourceID: "*", Actions: []string{"write"}}},
	}
	service, _ := newPersistentAgentToolInvocationAuditServiceV1(store, agentToolAuditResolverStub{invocation: invocation}, time.Now)
	_, err := service.Begin(context.Background(), application.AgentToolInvocationBeginV1{
		InvocationUUID: "INV-1", TaskUUID: "TASK-1", RunUUID: "RUN-1", Transport: application.AgentToolTransportMCP,
		ToolName: "send", CapabilityID: application.AgentCapabilitySystemMessageSend, ArgumentsSHA256: testAuditSHA,
	})
	if !errors.Is(err, application.ErrAgentToolInvocationDenied) || store.begun.InvocationUUID != "" {
		t.Fatalf("write capability should be denied before persistence: err=%v stored=%+v", err, store.begun)
	}
	service, _ = newPersistentAgentToolInvocationAuditServiceV1(store, agentToolAuditResolverStub{err: application.ErrAgentExecutionPolicyDenied}, time.Now)
	_, err = service.Begin(context.Background(), application.AgentToolInvocationBeginV1{
		InvocationUUID: "INV-2", TaskUUID: "TASK-1", RunUUID: "RUN-1", Transport: application.AgentToolTransportMCP,
		ToolName: "list", CapabilityID: application.AgentCapabilityConversationsList, ArgumentsSHA256: testAuditSHA,
	})
	if !errors.Is(err, application.ErrAgentToolInvocationDenied) {
		t.Fatalf("resolver failure should be denied: %v", err)
	}
}

func TestPersistentAgentToolInvocationAuditFinishesWithBoundedEvidence(t *testing.T) {
	store := &agentToolAuditStoreStub{}
	service, _ := newPersistentAgentToolInvocationAuditServiceV1(store, agentToolAuditResolverStub{}, time.Now)
	finish := application.AgentToolInvocationFinishV1{
		InvocationUUID: "INV-1", TaskUUID: "TASK-1", RunUUID: "RUN-1",
		Status: application.AgentToolInvocationStatusCompleted, ResultSHA256: testAuditSHA, ResultBytes: 128, LatencyMS: 12,
	}
	if err := service.Finish(context.Background(), finish); err != nil {
		t.Fatalf("finish invocation: %v", err)
	}
	if store.finished != finish {
		t.Fatalf("unexpected finish evidence: %+v", store.finished)
	}
	store.finishErr = application.ErrAgentToolInvocationConflict
	if err := service.Finish(context.Background(), finish); !errors.Is(err, application.ErrAgentToolInvocationConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

const testAuditSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
