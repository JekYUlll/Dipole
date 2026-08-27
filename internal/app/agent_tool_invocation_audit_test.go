package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
)

type agentToolAuditStoreStub struct {
	begun      application.AgentToolInvocationV1
	finished   application.AgentToolInvocationFinishV1
	invocation *application.AgentToolInvocationV1
	beginErr   error
	finishErr  error
}

func (s *agentToolAuditStoreStub) BeginToolInvocation(_ context.Context, invocation application.AgentToolInvocationV1) (bool, error) {
	s.begun = invocation
	return s.beginErr == nil, s.beginErr
}

func (s *agentToolAuditStoreStub) FinishToolInvocation(_ context.Context, finish application.AgentToolInvocationFinishV1) (bool, error) {
	s.finished = finish
	return s.finishErr == nil, s.finishErr
}

func (s *agentToolAuditStoreStub) GetToolInvocation(context.Context, string) (*application.AgentToolInvocationV1, error) {
	return s.invocation, nil
}

type agentToolApprovalReaderStub struct {
	approval *application.AgentApprovalV1
	run      *application.AgentRunV1
}

func (s agentToolApprovalReaderStub) GetApproval(context.Context, string) (*application.AgentApprovalV1, error) {
	return s.approval, nil
}

func (s agentToolApprovalReaderStub) GetRun(context.Context, string) (*application.AgentRunV1, error) {
	return s.run, nil
}

type agentToolReceiptQueryStub struct {
	receipt *application.MessageCommandReceipt
}

func (s agentToolReceiptQueryStub) GetMessageCommandReceipt(string, string) (*application.MessageCommandReceipt, error) {
	return s.receipt, nil
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
	}}, agentToolApprovalReaderStub{}, agentToolReceiptQueryStub{}, func() time.Time { return time.UnixMilli(1000) })
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
	service, _ := newPersistentAgentToolInvocationAuditServiceV1(store, agentToolAuditResolverStub{invocation: invocation}, agentToolApprovalReaderStub{}, agentToolReceiptQueryStub{}, time.Now)
	_, err := service.Begin(context.Background(), application.AgentToolInvocationBeginV1{
		InvocationUUID: "INV-1", TaskUUID: "TASK-1", RunUUID: "RUN-1", Transport: application.AgentToolTransportMCP,
		ToolName: "send", CapabilityID: application.AgentCapabilitySystemMessageSend, ArgumentsSHA256: testAuditSHA,
	})
	if !errors.Is(err, application.ErrAgentToolInvocationDenied) || store.begun.InvocationUUID != "" {
		t.Fatalf("write capability should be denied before persistence: err=%v stored=%+v", err, store.begun)
	}
	service, _ = newPersistentAgentToolInvocationAuditServiceV1(store, agentToolAuditResolverStub{err: application.ErrAgentExecutionPolicyDenied}, agentToolApprovalReaderStub{}, agentToolReceiptQueryStub{}, time.Now)
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
	store.invocation = &application.AgentToolInvocationV1{InvocationUUID: "INV-1", TaskUUID: "TASK-1", RunUUID: "RUN-1", CapabilityID: application.AgentCapabilityConversationsList, Status: application.AgentToolInvocationStatusRunning}
	service, _ := newPersistentAgentToolInvocationAuditServiceV1(store, agentToolAuditResolverStub{}, agentToolApprovalReaderStub{}, agentToolReceiptQueryStub{}, time.Now)
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

func TestPersistentAgentToolInvocationAuditBindsConsumedWriteApproval(t *testing.T) {
	consumedAt := time.UnixMilli(900)
	store := &agentToolAuditStoreStub{}
	invocation := application.AgentInvocationV1{
		TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI",
		Permissions:          []string{application.AgentPermissionMessageWrite},
		ApprovedCapabilities: []string{application.AgentCapabilitySystemMessageSend},
		ResourceScopes:       []application.AgentResourceScopeV1{{ResourceType: "conversation", ResourceID: "direct:U100:UAI", Actions: []string{"write"}}},
	}
	approval := &application.AgentApprovalV1{
		ApprovalUUID: "APR-1", TaskUUID: "TASK-1", CapabilityID: application.AgentCapabilitySystemMessageSend,
		ResourceScope:   application.AgentResourceScopeV1{ResourceType: "conversation", ResourceID: "direct:U100:UAI", Actions: []string{"write"}},
		ArgumentsSHA256: testAuditSHA, Status: application.AgentApprovalStatusConsumed, ConsumedAt: &consumedAt,
	}
	reader := agentToolApprovalReaderStub{approval: approval, run: &application.AgentRunV1{
		RunUUID: "RUN-1", TaskUUID: "TASK-1", RuntimeID: "dipole-agent", Mode: "active", Status: application.AgentRunStatusRunning,
	}}
	service, _ := newPersistentAgentToolInvocationAuditServiceV1(store, agentToolAuditResolverStub{invocation: invocation}, reader, agentToolReceiptQueryStub{}, time.Now)
	record, err := service.Begin(context.Background(), application.AgentToolInvocationBeginV1{
		InvocationUUID: "INV-W", TaskUUID: "TASK-1", RunUUID: "RUN-1", Transport: application.AgentToolTransportMCP,
		ToolName: "dipole_message_send", CapabilityID: application.AgentCapabilitySystemMessageSend,
		ArgumentsSHA256: testAuditSHA, ApprovalUUID: "APR-1",
	})
	if err != nil {
		t.Fatalf("begin approved write invocation: %v", err)
	}
	if record.ApprovalUUID != "APR-1" || store.begun.ApprovalUUID != "APR-1" {
		t.Fatalf("write approval was not persisted: record=%+v stored=%+v", record, store.begun)
	}

	approval.Status = application.AgentApprovalStatusApproved
	approval.ConsumedAt = nil
	_, err = service.Begin(context.Background(), application.AgentToolInvocationBeginV1{
		InvocationUUID: "INV-W2", TaskUUID: "TASK-1", RunUUID: "RUN-1", Transport: application.AgentToolTransportMCP,
		ToolName: "dipole_message_send", CapabilityID: application.AgentCapabilitySystemMessageSend,
		ArgumentsSHA256: testAuditSHA, ApprovalUUID: "APR-1",
	})
	if !errors.Is(err, application.ErrAgentToolInvocationDenied) {
		t.Fatalf("unconsumed approval error = %v", err)
	}
	approval.Status = application.AgentApprovalStatusConsumed
	approval.ConsumedAt = &consumedAt
	reader.run.Mode = "shadow"
	service, _ = newPersistentAgentToolInvocationAuditServiceV1(store, agentToolAuditResolverStub{invocation: invocation}, reader, agentToolReceiptQueryStub{}, time.Now)
	_, err = service.Begin(context.Background(), application.AgentToolInvocationBeginV1{
		InvocationUUID: "INV-W3", TaskUUID: "TASK-1", RunUUID: "RUN-1", Transport: application.AgentToolTransportMCP,
		ToolName: "dipole_message_send", CapabilityID: application.AgentCapabilitySystemMessageSend,
		ArgumentsSHA256: testAuditSHA, ApprovalUUID: "APR-1",
	})
	if !errors.Is(err, application.ErrAgentToolInvocationDenied) {
		t.Fatalf("shadow Run approval error = %v", err)
	}
}

func TestPersistentAgentToolInvocationAuditVerifiesMessageActionReference(t *testing.T) {
	clientMessageID, err := application.AgentCommandClientMessageIDV1(application.AgentMessageCommandSystemMessageV1, "CMD-1")
	if err != nil {
		t.Fatalf("derive client message ID: %v", err)
	}
	store := &agentToolAuditStoreStub{invocation: &application.AgentToolInvocationV1{
		InvocationUUID: "INV-W", TaskUUID: "TASK-1", RunUUID: "RUN-1", PrincipalUUID: "U100", AgentUUID: "UAI",
		CapabilityID: application.AgentCapabilitySystemMessageSend, ApprovalUUID: "APR-1", Status: application.AgentToolInvocationStatusRunning,
	}}
	receipt := &application.MessageCommandReceipt{Status: application.MessageCommandReceiptStatusCommitted, Message: &model.Message{
		UUID: "MSG-1", ClientMessageID: clientMessageID, ConversationKey: model.DirectConversationKey("UAI", "U100"),
		SenderUUID: "UAI", TargetUUID: "U100", TargetType: model.MessageTargetDirect, MessageType: model.MessageTypeSystem,
	}}
	service, _ := newPersistentAgentToolInvocationAuditServiceV1(store, agentToolAuditResolverStub{}, agentToolApprovalReaderStub{}, agentToolReceiptQueryStub{receipt: receipt}, time.Now)
	finish := application.AgentToolInvocationFinishV1{
		InvocationUUID: "INV-W", TaskUUID: "TASK-1", RunUUID: "RUN-1", Status: application.AgentToolInvocationStatusCompleted,
		ResultSHA256: testAuditSHA, ResultBytes: 64, LatencyMS: 9,
		ActionReference: &application.AgentToolActionReferenceV1{ResourceType: application.AgentToolActionResourceMessage, ResourceUUID: "MSG-1", CommandKind: application.AgentMessageCommandSystemMessageV1, CommandID: "CMD-1"},
	}
	if err := service.Finish(context.Background(), finish); err != nil {
		t.Fatalf("finish write invocation: %v", err)
	}
	if store.finished.ActionReference == nil || store.finished.ActionReference.ResourceUUID != "MSG-1" {
		t.Fatalf("action reference was not persisted: %+v", store.finished)
	}

	finish.ActionReference.ResourceUUID = "MSG-OTHER"
	if err := service.Finish(context.Background(), finish); !errors.Is(err, application.ErrAgentToolInvocationConflict) {
		t.Fatalf("conflicting Message reference error = %v", err)
	}
	service, _ = newPersistentAgentToolInvocationAuditServiceV1(store, agentToolAuditResolverStub{}, agentToolApprovalReaderStub{}, agentToolReceiptQueryStub{}, time.Now)
	finish.ActionReference.ResourceUUID = "MSG-1"
	if err := service.Finish(context.Background(), finish); !errors.Is(err, application.ErrAgentToolInvocationConflict) {
		t.Fatalf("missing Message receipt error = %v", err)
	}
}

const testAuditSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
