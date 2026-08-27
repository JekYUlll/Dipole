package agentgrpc

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	grpccommon "github.com/JekYUlll/Dipole/internal/transport/grpc/common"
	agentv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/agent/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type resolverStub struct{ invocation application.AgentInvocationV1 }

func (s resolverStub) Resolve(context.Context, string, string) (application.AgentInvocationV1, error) {
	return s.invocation, nil
}

type admissionStub struct {
	result         application.AgentRunAdmissionV1
	completedTask  string
	completedRun   string
	finishedStatus application.AgentRunStatusV1
	finishedError  string
}

func (s *admissionStub) Admit(context.Context, application.AgentRunAdmissionRequestV1) (*application.AgentRunAdmissionV1, error) {
	return &s.result, nil
}

func (s *admissionStub) Complete(_ context.Context, taskUUID, runUUID, _, _ string) error {
	s.completedTask, s.completedRun = taskUUID, runUUID
	return nil
}

func (s *admissionStub) Finish(_ context.Context, taskUUID, runUUID, _, _ string, runStatus application.AgentRunStatusV1, lastError string) error {
	s.completedTask, s.completedRun = taskUUID, runUUID
	s.finishedStatus, s.finishedError = runStatus, lastError
	return nil
}

type capabilityStub struct {
	application.AgentCapabilityV1
	invocation application.AgentInvocationV1
}

type approvalServiceStub struct {
	requested application.AgentApprovalRequestV1
	resolved  application.AgentApprovalResolutionV1
}

type taskControlAuthorizerStub struct {
	taskUUID      string
	principalUUID string
	result        application.AgentTaskControlAuthorizationV1
	err           error
}

func (s *taskControlAuthorizerStub) AuthorizeTaskControl(_ context.Context, taskUUID, principalUUID string) (*application.AgentTaskControlAuthorizationV1, error) {
	s.taskUUID, s.principalUUID = taskUUID, principalUUID
	if s.err != nil {
		return nil, s.err
	}
	return &s.result, nil
}

func (s *approvalServiceStub) Request(_ context.Context, request application.AgentApprovalRequestV1) (*application.AgentApprovalV1, error) {
	s.requested = request
	approval := request.Approval
	return &approval, nil
}

func (s *approvalServiceStub) Resolve(_ context.Context, resolution application.AgentApprovalResolutionV1) (*application.AgentApprovalV1, error) {
	s.resolved = resolution
	status := application.AgentApprovalStatusRevoked
	actor := ""
	if resolution.Decision == application.AgentApprovalDecisionApproved {
		status, actor = application.AgentApprovalStatusApproved, resolution.ActorUUID
	}
	return &application.AgentApprovalV1{ApprovalUUID: resolution.ApprovalUUID, Status: status, ApprovedByUUID: actor}, nil
}

func (s *capabilityStub) ListConversations(_ context.Context, invocation application.AgentInvocationV1, limit int) ([]*model.Conversation, error) {
	s.invocation = invocation
	return []*model.Conversation{{ConversationKey: "group:G1", TargetUUID: "G1", LastMessageSeq: uint64(limit)}}, nil
}

func TestListConversationsResolvesTrustedTaskIdentity(t *testing.T) {
	capability := &capabilityStub{}
	server, err := NewServer(capability, resolverStub{invocation: application.AgentInvocationV1{PrincipalUUID: "U100", AgentUUID: "UAI"}}, &admissionStub{})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	response, err := server.ListConversations(context.Background(), &agentv1.ListConversationsRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), TaskId: "TASK-1", RunId: "RUN-1", Limit: 20,
	})
	if err != nil {
		t.Fatalf("list conversations: %v", err)
	}
	if capability.invocation.PrincipalUUID != "U100" || len(response.GetConversations()) != 1 || response.GetConversations()[0].GetLastMessageSeq() != 20 {
		t.Fatalf("unexpected trusted response: invocation=%+v response=%+v", capability.invocation, response)
	}
}

func TestListConversationsRejectsClientPrincipal(t *testing.T) {
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{})
	_, err := server.ListConversations(context.Background(), &agentv1.ListConversationsRequest{
		Context: grpccommon.RequestContext("U999", "dipole-agent"), TaskId: "TASK-1", RunId: "RUN-1", Limit: 20,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("forged principal code = %s, want %s", status.Code(err), codes.InvalidArgument)
	}
}

func TestAuthorizeTaskControlUsesExplicitAuthenticatedPrincipal(t *testing.T) {
	controls := &taskControlAuthorizerStub{result: application.AgentTaskControlAuthorizationV1{TaskUUID: "TASK-1", Status: application.AgentTaskStatusWaitingApproval}}
	server, err := NewServerWithControl(&capabilityStub{}, resolverStub{}, &admissionStub{}, &approvalServiceStub{}, controls)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	response, err := server.AuthorizeTaskControl(context.Background(), &agentv1.AuthorizeTaskControlRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), TaskId: "TASK-1", PrincipalUserId: "U100",
	})
	if err != nil || response.GetTaskId() != "TASK-1" || response.GetTaskStatus() != "waiting_approval" || controls.taskUUID != "TASK-1" || controls.principalUUID != "U100" {
		t.Fatalf("unexpected authorization: response=%+v controls=%+v err=%v", response, controls, err)
	}
}

func TestAuthorizeTaskControlHidesForeignTaskAndRejectsContextPrincipal(t *testing.T) {
	controls := &taskControlAuthorizerStub{err: application.ErrAgentExecutionPolicyDenied}
	server, _ := NewServerWithControl(&capabilityStub{}, resolverStub{}, &admissionStub{}, &approvalServiceStub{}, controls)
	for _, request := range []*agentv1.AuthorizeTaskControlRequest{
		{Context: grpccommon.RequestContext("", "dipole-agent"), TaskId: "TASK-1", PrincipalUserId: "U999"},
		{Context: grpccommon.RequestContext("U999", "dipole-agent"), TaskId: "TASK-1", PrincipalUserId: "U100"},
	} {
		_, err := server.AuthorizeTaskControl(context.Background(), request)
		if request.GetContext().GetPrincipalUserId() == "" && status.Code(err) != codes.NotFound {
			t.Fatalf("foreign Task code = %s, want %s", status.Code(err), codes.NotFound)
		}
		if request.GetContext().GetPrincipalUserId() != "" && status.Code(err) != codes.InvalidArgument {
			t.Fatalf("context principal code = %s, want %s", status.Code(err), codes.InvalidArgument)
		}
	}
}

func TestAdmitRunReturnsServerDerivedIdentity(t *testing.T) {
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{result: application.AgentRunAdmissionV1{TaskUUID: "TASK-1", RunUUID: "RUN-1", RunStatus: application.AgentRunStatusRunning}})
	response, err := server.AdmitRun(context.Background(), &agentv1.AdmitRunRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), TenantId: "dipole", PrincipalUserId: "U100",
		AgentId: "UAI", TriggerType: "message.direct.created", TriggerRef: "M100", EventId: "E1",
		RuntimeId: "dipole-agent", Mode: "shadow",
	})
	if err != nil || response.GetTaskId() != "TASK-1" || response.GetRunId() != "RUN-1" || response.GetRunStatus() != "running" {
		t.Fatalf("unexpected admission response: response=%+v err=%v", response, err)
	}
}

func TestAdmitRunRejectsForgedRuntimeMode(t *testing.T) {
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{})
	_, err := server.AdmitRun(context.Background(), &agentv1.AdmitRunRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), TenantId: "dipole", PrincipalUserId: "U100",
		AgentId: "UAI", TriggerType: "message.direct.created", TriggerRef: "M100",
		RuntimeId: "forged-runtime", Mode: "active",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("forged Runtime code = %s, want %s", status.Code(err), codes.InvalidArgument)
	}
}

func TestCompleteRunUsesServerRuntimeBinding(t *testing.T) {
	admission := &admissionStub{}
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, admission)
	response, err := server.CompleteRun(context.Background(), &agentv1.CompleteRunRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), TaskId: "TASK-1", RunId: "RUN-1",
	})
	if err != nil || response.GetRunStatus() != "completed" || admission.completedTask != "TASK-1" || admission.completedRun != "RUN-1" {
		t.Fatalf("unexpected completion response: response=%+v admission=%+v err=%v", response, admission, err)
	}
}

func TestFinishRunUsesServerRuntimeBinding(t *testing.T) {
	admission := &admissionStub{}
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, admission)
	response, err := server.FinishRun(context.Background(), &agentv1.FinishRunRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), TaskId: "TASK-1", RunId: "RUN-1",
		RunStatus: "failed", LastError: "Activity retries exhausted",
	})
	if err != nil || response.GetRunStatus() != "failed" || admission.completedTask != "TASK-1" || admission.completedRun != "RUN-1" ||
		admission.finishedStatus != application.AgentRunStatusFailed || admission.finishedError != "Activity retries exhausted" {
		t.Fatalf("unexpected finish response: response=%+v admission=%+v err=%v", response, admission, err)
	}
}

func TestFinishRunRejectsInvalidStatusAndClientPrincipal(t *testing.T) {
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{})
	for _, request := range []*agentv1.FinishRunRequest{
		{Context: grpccommon.RequestContext("", "dipole-agent"), TaskId: "TASK-1", RunId: "RUN-1", RunStatus: "running"},
		{Context: grpccommon.RequestContext("U999", "dipole-agent"), TaskId: "TASK-1", RunId: "RUN-1", RunStatus: "failed", LastError: "failed"},
	} {
		if _, err := server.FinishRun(context.Background(), request); status.Code(err) != codes.InvalidArgument {
			t.Fatalf("FinishRun code = %s, want %s", status.Code(err), codes.InvalidArgument)
		}
	}
}

func TestApprovalRPCUsesServerRuntimeAndExactBinding(t *testing.T) {
	approvals := &approvalServiceStub{}
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{}, approvals)
	response, err := server.RequestApproval(context.Background(), &agentv1.RequestApprovalRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), TaskId: "TASK-1", RunId: "RUN-1", ApprovalId: "APR-1",
		CapabilityId: "message.bulk.send", ResourceScope: &agentv1.AgentResourceScope{ResourceType: "conversation", ResourceId: "G1", Actions: []string{"write"}},
		ScopeSha256: strings.Repeat("a", 64), ArgumentsSha256: strings.Repeat("b", 64), NonceSha256: strings.Repeat("c", 64), ExpiresAtUnixMs: time.Now().Add(time.Hour).UnixMilli(),
	})
	if err != nil || response.GetStatus() != "pending" || approvals.requested.RuntimeID != "dipole-agent" || approvals.requested.Mode != "shadow" {
		t.Fatalf("request Approval response=%+v request=%+v err=%v", response, approvals.requested, err)
	}
	resolved, err := server.ResolveApproval(context.Background(), &agentv1.ResolveApprovalRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), TaskId: "TASK-1", RunId: "RUN-1", ApprovalId: "APR-1", ActorUserId: "U100", Decision: "approved",
	})
	if err != nil || resolved.GetStatus() != "approved" || approvals.resolved.ActorUUID != "U100" || approvals.resolved.Decision != application.AgentApprovalDecisionApproved {
		t.Fatalf("resolve Approval response=%+v resolution=%+v err=%v", resolved, approvals.resolved, err)
	}
}
