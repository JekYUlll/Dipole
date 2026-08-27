package agentgrpc

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	grpcauth "github.com/JekYUlll/Dipole/internal/transport/grpc/auth"
	grpccommon "github.com/JekYUlll/Dipole/internal/transport/grpc/common"
	agentv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/agent/v1"
	commonv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/common/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type resolverStub struct{ invocation application.AgentInvocationV1 }

func (s resolverStub) Resolve(context.Context, string, string) (application.AgentInvocationV1, error) {
	return s.invocation, nil
}

type admissionStub struct {
	result         application.AgentRunAdmissionV1
	admitted       application.AgentRunAdmissionRequestV1
	completedTask  string
	completedRun   string
	finishedStatus application.AgentRunStatusV1
	finishedError  string
}

func (s *admissionStub) Admit(_ context.Context, request application.AgentRunAdmissionRequestV1) (*application.AgentRunAdmissionV1, error) {
	s.admitted = request
	return &s.result, nil
}

type eventSubscriptionResolverStub struct {
	request application.AgentEventSubscriptionMatchRequestV1
	items   []application.AgentEventSubscriptionV1
	err     error
}

type agentMemoryResolverStub struct {
	taskUUID, runUUID, resourceType, resourceID string
	limit                                       int
	items                                       []application.AgentMemoryV1
	err                                         error
}

func (s *agentMemoryResolverStub) ResolveContextMemories(_ context.Context, taskUUID, runUUID, resourceType, resourceID string, limit int) ([]application.AgentMemoryV1, error) {
	s.taskUUID, s.runUUID, s.resourceType, s.resourceID, s.limit = taskUUID, runUUID, resourceType, resourceID, limit
	return s.items, s.err
}

func (s *eventSubscriptionResolverStub) MatchEventSubscriptions(_ context.Context, request application.AgentEventSubscriptionMatchRequestV1) ([]application.AgentEventSubscriptionV1, error) {
	s.request = request
	return s.items, s.err
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

type taskWorkflowProjectionStub struct {
	request application.AgentTaskWorkflowProjectionRequestV1
	result  application.AgentTaskWorkflowProjectionV1
	err     error
}

func (s *taskWorkflowProjectionStub) Project(_ context.Context, request application.AgentTaskWorkflowProjectionRequestV1) (*application.AgentTaskWorkflowProjectionV1, error) {
	s.request = request
	if s.err != nil {
		return nil, s.err
	}
	return &s.result, nil
}

func (s *taskWorkflowProjectionStub) ListProjectionSnapshots(_ context.Context, _ string, _ int) (*application.AgentTaskWorkflowProjectionPageV1, error) {
	return &application.AgentTaskWorkflowProjectionPageV1{Tasks: []application.AgentTaskWorkflowProjectionSnapshotV1{{
		TaskUUID: "TASK-1", Workflow: &s.result,
	}}}, s.err
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
	controls := &taskControlAuthorizerStub{result: application.AgentTaskControlAuthorizationV1{
		TaskUUID: "TASK-1", Status: application.AgentTaskStatusWaitingApproval,
		Workflow: &application.AgentTaskWorkflowProjectionV1{WorkflowID: "dipole-agent-task/TASK-1", RunID: "temporal-run-1", Status: application.AgentTaskWorkflowStatusWaitingApproval, Revision: 2},
	}}
	server, err := NewServerWithControl(&capabilityStub{}, resolverStub{}, &admissionStub{}, &approvalServiceStub{}, controls)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	response, err := server.AuthorizeTaskControl(context.Background(), &agentv1.AuthorizeTaskControlRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), TaskId: "TASK-1", PrincipalUserId: "U100",
	})
	if err != nil || response.GetTaskId() != "TASK-1" || response.GetTaskStatus() != "waiting_approval" || response.GetWorkflowRevision() != 2 ||
		response.GetWorkflowStatus() != "waiting_approval" || controls.taskUUID != "TASK-1" || controls.principalUUID != "U100" {
		t.Fatalf("unexpected authorization: response=%+v controls=%+v err=%v", response, controls, err)
	}
}

func TestProjectTaskWorkflowStateUsesFixedRuntimeBinding(t *testing.T) {
	projection := &taskWorkflowProjectionStub{result: application.AgentTaskWorkflowProjectionV1{
		TaskUUID: "TASK-1", WorkflowID: "dipole-agent-task/TASK-1", RunID: "temporal-run-1",
		Status: application.AgentTaskWorkflowStatusWaitingInput, Revision: 2,
	}}
	server, err := NewServerWithControlAndProjection(
		&capabilityStub{}, resolverStub{}, &admissionStub{}, &approvalServiceStub{}, &taskControlAuthorizerStub{}, projection,
	)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	response, err := server.ProjectTaskWorkflowState(context.Background(), &agentv1.ProjectTaskWorkflowStateRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), TaskId: "TASK-1", RunId: "RUN-1",
		WorkflowId: "dipole-agent-task/TASK-1", WorkflowRunId: "temporal-run-1", WorkflowStatus: "waiting_input", WorkflowRevision: 2,
	})
	if err != nil || response.GetWorkflowRevision() != 2 || projection.request.RuntimeID != "dipole-agent" || projection.request.Mode != "shadow" || projection.request.RunUUID != "RUN-1" {
		t.Fatalf("unexpected Workflow projection: response=%+v request=%+v err=%v", response, projection.request, err)
	}
}

func TestProjectTaskWorkflowStateRejectsClientPrincipalAndConflict(t *testing.T) {
	projection := &taskWorkflowProjectionStub{err: application.ErrAgentWorkflowProjectionConflict}
	server, _ := NewServerWithControlAndProjection(
		&capabilityStub{}, resolverStub{}, &admissionStub{}, &approvalServiceStub{}, &taskControlAuthorizerStub{}, projection,
	)
	for _, request := range []*agentv1.ProjectTaskWorkflowStateRequest{
		{Context: grpccommon.RequestContext("", "dipole-agent"), TaskId: "TASK-1"},
		{Context: grpccommon.RequestContext("U999", "dipole-agent"), TaskId: "TASK-1"},
	} {
		_, err := server.ProjectTaskWorkflowState(context.Background(), request)
		if request.GetContext().GetPrincipalUserId() == "" && status.Code(err) != codes.FailedPrecondition {
			t.Fatalf("projection conflict code = %s", status.Code(err))
		}
		if request.GetContext().GetPrincipalUserId() != "" && status.Code(err) != codes.InvalidArgument {
			t.Fatalf("projection principal code = %s", status.Code(err))
		}
	}
}

func TestListTaskWorkflowProjectionSnapshotsUsesServiceIdentity(t *testing.T) {
	projection := &taskWorkflowProjectionStub{result: application.AgentTaskWorkflowProjectionV1{
		TaskUUID: "TASK-1", WorkflowID: "dipole-agent-task/TASK-1", RunID: "temporal-run-1",
		Status: application.AgentTaskWorkflowStatusRunning, Revision: 1,
	}}
	server, _ := NewServerWithControlAndProjection(
		&capabilityStub{}, resolverStub{}, &admissionStub{}, &approvalServiceStub{}, &taskControlAuthorizerStub{}, projection,
	)
	response, err := server.ListTaskWorkflowProjectionSnapshots(context.Background(), &agentv1.ListTaskWorkflowProjectionSnapshotsRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), PageSize: 100,
	})
	if err != nil || len(response.GetTasks()) != 1 || !response.GetTasks()[0].GetHasWorkflow() || response.GetTasks()[0].GetWorkflowRevision() != 1 {
		t.Fatalf("projection snapshots: response=%+v err=%v", response, err)
	}
	_, err = server.ListTaskWorkflowProjectionSnapshots(context.Background(), &agentv1.ListTaskWorkflowProjectionSnapshotsRequest{
		Context: grpccommon.RequestContext("U999", "dipole-agent"), PageSize: 100,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("projection snapshot principal code = %s", status.Code(err))
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
	admission := &admissionStub{result: application.AgentRunAdmissionV1{TaskUUID: "TASK-1", RunUUID: "RUN-1", RunStatus: application.AgentRunStatusRunning}}
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, admission)
	response, err := server.AdmitRun(context.Background(), &agentv1.AdmitRunRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), TenantId: "dipole", PrincipalUserId: "U100",
		AgentId: "UAI", TriggerType: "message.direct.created", TriggerRef: "M100", EventId: "E1",
		RuntimeId: "dipole-agent", Mode: "shadow", SubscriptionId: "SUB-1",
	})
	if err != nil || response.GetTaskId() != "TASK-1" || response.GetRunId() != "RUN-1" || response.GetRunStatus() != "running" || admission.admitted.SubscriptionUUID != "SUB-1" {
		t.Fatalf("unexpected admission response: response=%+v request=%+v err=%v", response, admission.admitted, err)
	}
}

func TestMatchEventSubscriptionsUsesAuthenticatedRuntimeIdentity(t *testing.T) {
	resolver := &eventSubscriptionResolverStub{items: []application.AgentEventSubscriptionV1{{
		SubscriptionUUID: "SUB-1", DefinitionUUID: "DEF-1", DefinitionVersion: 2,
		TenantID: "dipole", AgentUUID: "UAI", Status: application.AgentSubscriptionStatusActive,
		EventType: "message.direct.created", ResourceType: "conversation", ResourceID: "group:G1",
		FilterKind: application.AgentSubscriptionFilterMessageContainsAny, FilterJSON: []byte(`{"terms":["incident"]}`),
	}}}
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{})
	server, _ = server.WithEventSubscriptions(resolver)
	request := &agentv1.MatchEventSubscriptionsRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), TenantId: "dipole", AgentId: "UAI",
		EventType: "message.direct.created", ResourceType: "conversation", ResourceId: "group:G1",
	}
	response, err := invokeAuthenticatedAgentRPC(t, "dipole-agent", func(ctx context.Context) (any, error) {
		return server.MatchEventSubscriptions(ctx, request)
	})
	if err != nil {
		t.Fatalf("match Event Subscriptions: %v", err)
	}
	matched := response.(*agentv1.MatchEventSubscriptionsResponse)
	if resolver.request.ResourceID != "group:G1" || len(matched.GetSubscriptions()) != 1 || matched.GetSubscriptions()[0].GetSubscriptionId() != "SUB-1" {
		t.Fatalf("unexpected Subscription match: request=%+v response=%+v", resolver.request, matched)
	}

	request.Context.PrincipalUserId = "U999"
	_, err = invokeAuthenticatedAgentRPC(t, "dipole-agent", func(ctx context.Context) (any, error) {
		return server.MatchEventSubscriptions(ctx, request)
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("forged principal code = %s, want %s", status.Code(err), codes.PermissionDenied)
	}
}

func TestListContextMemoriesUsesTaskIdentityWithoutClientPrincipal(t *testing.T) {
	resolver := &agentMemoryResolverStub{items: []application.AgentMemoryV1{{
		MemoryUUID: "MEM-1", MemoryType: application.AgentMemoryTypeSemantic, Content: "Owner is Alice", CompactContent: "Owner: Alice", Priority: 90,
		Provenance: application.AgentMemoryProvenanceV1{SourceType: "message", SourceID: "M1", Sequence: "42"},
	}}}
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{})
	server, _ = server.WithMemories(resolver)
	request := &agentv1.ListContextMemoriesRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), TaskId: "TASK-1", RunId: "RUN-1",
		ResourceType: "conversation", ResourceId: "group:G1", Limit: 20,
	}
	response, err := invokeAuthenticatedAgentRPC(t, "dipole-agent", func(ctx context.Context) (any, error) {
		return server.ListContextMemories(ctx, request)
	})
	if err != nil {
		t.Fatalf("list Context Memories: %v", err)
	}
	memories := response.(*agentv1.ListContextMemoriesResponse).GetMemories()
	if resolver.taskUUID != "TASK-1" || resolver.resourceID != "group:G1" || resolver.limit != 20 || len(memories) != 1 || memories[0].GetProvenance().GetSourceId() != "M1" {
		t.Fatalf("unexpected Memory request=%+v response=%+v", resolver, memories)
	}
	request.Context.PrincipalUserId = "U999"
	_, err = invokeAuthenticatedAgentRPC(t, "dipole-agent", func(ctx context.Context) (any, error) { return server.ListContextMemories(ctx, request) })
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("forged principal code = %s", status.Code(err))
	}
}

func invokeAuthenticatedAgentRPC(t *testing.T, caller string, handler func(context.Context) (any, error)) (any, error) {
	t.Helper()
	interceptor, err := grpcauth.NewUnaryServerInterceptor("secret", caller)
	if err != nil {
		t.Fatalf("new auth interceptor: %v", err)
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"x-dipole-caller-service", caller, "x-dipole-service-token", "secret",
	))
	return interceptor(ctx, nil, nil, func(ctx context.Context, _ any) (any, error) { return handler(ctx) })
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

func TestWorkflowRepairRPCRejectsUnauthenticatedDirectAndAgentCalls(t *testing.T) {
	repairs := &workflowRepairAuditServiceStub{proposal: &application.AgentWorkflowRepairProposalV1{
		ProposalUUID: "repair:" + strings.Repeat("a", 64), TaskUUID: "TASK-1", Outcome: application.AgentWorkflowRepairOutcomeStale,
		Action: application.AgentWorkflowRepairActionV1, ProposerUUID: "U-OPS", EvidenceSHA256: strings.Repeat("a", 64),
		Status: application.AgentWorkflowRepairStatusProposed, RequiredApprovals: 2, ProposedAt: time.Unix(1, 0), ExpiresAt: time.Unix(2, 0),
	}}
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{})
	server.repairs = repairs
	_, err := server.ProposeWorkflowRepair(context.Background(), &agentv1.ProposeWorkflowRepairRequest{
		Context: grpccommon.RequestContext("U-OPS", "dipole-gateway"), TaskId: "TASK-1", Outcome: "stale", TicketRef: "INC-1", Reason: "verified",
		Temporal:         &agentv1.WorkflowRepairEvidence{WorkflowId: "dipole-agent-task/TASK-1", WorkflowRunId: "WR-1", Status: "completed", Revision: 3},
		ProposedAtUnixMs: 1000, ExpiresAtUnixMs: 2000,
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("direct Gateway claim code = %s", status.Code(err))
	}
	for _, requestContext := range []*commonv1.RequestContext{
		grpccommon.RequestContext("", "dipole-gateway"), grpccommon.RequestContext("U-OPS", "dipole-agent"),
	} {
		_, err := server.GetWorkflowRepair(context.Background(), &agentv1.GetWorkflowRepairRequest{Context: requestContext, ProposalId: repairs.proposal.ProposalUUID})
		if status.Code(err) != codes.Unauthenticated && status.Code(err) != codes.PermissionDenied {
			t.Fatalf("repair auth code = %s", status.Code(err))
		}
	}
}

type workflowRepairAuditServiceStub struct {
	proposal *application.AgentWorkflowRepairProposalV1
	operator string
}

func (s *workflowRepairAuditServiceStub) Propose(_ context.Context, operator string, _ application.AgentWorkflowRepairProposalRequestV1) (*application.AgentWorkflowRepairProposalV1, error) {
	s.operator = operator
	return s.proposal, nil
}
func (s *workflowRepairAuditServiceStub) Decide(_ context.Context, operator, _ string, _ application.AgentWorkflowRepairDecisionV1) (*application.AgentWorkflowRepairProposalV1, error) {
	s.operator = operator
	return s.proposal, nil
}
func (s *workflowRepairAuditServiceStub) Get(_ context.Context, operator, _ string) (*application.AgentWorkflowRepairProposalV1, error) {
	s.operator = operator
	return s.proposal, nil
}
