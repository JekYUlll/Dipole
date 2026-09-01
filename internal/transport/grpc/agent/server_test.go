package agentgrpc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	agentv1 "github.com/JekYUlll/Dipole/api/gen/go/agent/v1"
	commonv1 "github.com/JekYUlll/Dipole/api/gen/go/common/v1"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	grpcauth "github.com/JekYUlll/Dipole/internal/transport/grpc/auth"
	grpccommon "github.com/JekYUlll/Dipole/internal/transport/grpc/common"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type resolverStub struct{ invocation application.AgentInvocationV1 }

func TestArtifactTimelineEventUsesContentAddressedEventID(t *testing.T) {
	artifactID := strings.Repeat("a", application.AgentTaskTimelineEventUUIDMaxLengthV1)
	event := artifactTimelineEventV1(&application.AgentArtifactV1{
		ArtifactUUID: artifactID,
		TaskUUID:     "task-1",
		RunUUID:      "run-1",
		CreatedAt:    time.Now().UTC(),
	})
	if event.EventUUID != artifactID || len(event.EventUUID) > application.AgentTaskTimelineEventUUIDMaxLengthV1 {
		t.Fatalf("timeline event ID = %q, want content-addressed Artifact ID within persisted limit", event.EventUUID)
	}
	if err := event.Validate(); err != nil {
		t.Fatalf("artifact timeline event validation = %v", err)
	}
}

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

type eventSubscriptionControlStub struct {
	principal string
	created   application.AgentEventSubscriptionCreateRequestV1
	listed    application.AgentEventSubscriptionListRequestV1
	revoked   application.AgentEventSubscriptionRevokeRequestV1
	item      application.AgentEventSubscriptionV1
	options   application.AgentSubscriptionConversationOptionsRequestV1
}

type agentDefinitionCatalogStub struct {
	principal string
	request   application.AgentDefinitionCatalogListRequestV1
	page      application.AgentDefinitionCatalogPageV1
}

func (s *agentDefinitionCatalogStub) List(_ context.Context, principal string, request application.AgentDefinitionCatalogListRequestV1) (*application.AgentDefinitionCatalogPageV1, error) {
	s.principal, s.request = principal, request
	copy := s.page
	return &copy, nil
}

func (s *eventSubscriptionControlStub) Create(_ context.Context, principal string, request application.AgentEventSubscriptionCreateRequestV1) (*application.AgentEventSubscriptionV1, error) {
	s.principal, s.created = principal, request
	copy := s.item
	return &copy, nil
}

func (s *eventSubscriptionControlStub) ListEligibleConversations(_ context.Context, principal string, request application.AgentSubscriptionConversationOptionsRequestV1) ([]application.AgentSubscriptionConversationOptionV1, error) {
	s.principal, s.options = principal, request
	return []application.AgentSubscriptionConversationOptionV1{{ConversationKey: "group:G1", EventType: "message.group.created"}}, nil
}

func (s *eventSubscriptionControlStub) List(_ context.Context, principal string, request application.AgentEventSubscriptionListRequestV1) (*application.AgentEventSubscriptionPageV1, error) {
	s.principal, s.listed = principal, request
	return &application.AgentEventSubscriptionPageV1{Subscriptions: []application.AgentEventSubscriptionV1{s.item}, NextCursor: "NEXT"}, nil
}

func (s *eventSubscriptionControlStub) Revoke(_ context.Context, principal string, request application.AgentEventSubscriptionRevokeRequestV1) (*application.AgentEventSubscriptionV1, error) {
	s.principal, s.revoked = principal, request
	copy := s.item
	copy.Status = application.AgentSubscriptionStatusRevoked
	now := time.Unix(3, 0)
	copy.RevokedAt, copy.UpdatedAt, copy.RevokedByUUID, copy.RevokeReason = &now, now, principal, request.Reason
	return &copy, nil
}

type agentMemoryResolverStub struct {
	taskUUID, runUUID, resourceType, resourceID string
	limit                                       int
	items                                       []application.AgentMemoryV1
	err                                         error
}

type agentMemoryOwnerControlStub struct {
	listRequest       application.AgentMemoryOwnerListRequestV1
	revokeRequest     application.AgentMemoryOwnerRevokeRequestV1
	correctionRequest application.AgentMemoryOwnerCorrectionRequestV1
	page              application.AgentMemoryOwnerPageV1
	item              application.AgentMemoryV1
	correction        application.AgentMemoryOwnerCorrectionResultV1
}

type agentMemoryCandidatePromotionStub struct {
	request application.AgentMemoryCandidatePromotionRequestV1
	item    application.AgentMemoryV1
}

type agentMemoryPromotionReceiptCommitStub struct {
	request application.AgentMemoryPromotionReceiptCommitRequestV1
	item    application.AgentMemoryV1
}

func (s *agentMemoryPromotionReceiptCommitStub) CommitMemoryPromotionReceipt(_ context.Context, request application.AgentMemoryPromotionReceiptCommitRequestV1) (*application.AgentMemoryV1, error) {
	s.request = request
	item := s.item
	return &item, nil
}

func (s *agentMemoryCandidatePromotionStub) Promote(_ context.Context, request application.AgentMemoryCandidatePromotionRequestV1) (*application.AgentMemoryV1, error) {
	s.request = request
	item := s.item
	return &item, nil
}

func (s *agentMemoryOwnerControlStub) ListOwnedMemories(_ context.Context, request application.AgentMemoryOwnerListRequestV1) (*application.AgentMemoryOwnerPageV1, error) {
	s.listRequest = request
	copy := s.page
	return &copy, nil
}

func (s *agentMemoryOwnerControlStub) RevokeOwnedMemory(_ context.Context, request application.AgentMemoryOwnerRevokeRequestV1) (*application.AgentMemoryV1, error) {
	s.revokeRequest = request
	copy := s.item
	return &copy, nil
}

func (s *agentMemoryOwnerControlStub) CorrectOwnedMemory(_ context.Context, request application.AgentMemoryOwnerCorrectionRequestV1) (*application.AgentMemoryOwnerCorrectionResultV1, error) {
	s.correctionRequest = request
	copy := s.correction
	return &copy, nil
}

type agentToolAuditStub struct {
	begin   application.AgentToolInvocationBeginV1
	finish  application.AgentToolInvocationFinishV1
	command *application.AgentMCPToolCommandV1
	err     error
}

type agentMCPToolRoundStub struct {
	claim  application.AgentMCPToolRoundClaimV1
	finish application.AgentMCPToolRoundFinishV1
	result *application.AgentMCPToolRoundClaimResultV1
	err    error
}

type agentMCPToolTerminalStub struct {
	request    application.AgentMCPToolInvocationTerminalRequestV1
	invocation *application.AgentToolInvocationV1
	err        error
}

func (s *agentMCPToolTerminalStub) FinishFromRound(_ context.Context, request application.AgentMCPToolInvocationTerminalRequestV1) (*application.AgentToolInvocationV1, error) {
	s.request = request
	return s.invocation, s.err
}

func (s *agentMCPToolRoundStub) Claim(_ context.Context, claim application.AgentMCPToolRoundClaimV1) (*application.AgentMCPToolRoundClaimResultV1, error) {
	s.claim = claim
	return s.result, s.err
}

func (s *agentMCPToolRoundStub) Finish(_ context.Context, finish application.AgentMCPToolRoundFinishV1) error {
	s.finish = finish
	return s.err
}

type agentMessageCommandExecutionStub struct {
	request application.AgentMessageCommandExecutionRequestV1
	result  application.AgentMessageCommandExecutionResultV1
	err     error
}

func (s *agentMessageCommandExecutionStub) Execute(_ context.Context, request application.AgentMessageCommandExecutionRequestV1) (*application.AgentMessageCommandExecutionResultV1, error) {
	s.request = request
	return &s.result, s.err
}

func (s *agentToolAuditStub) Begin(_ context.Context, begin application.AgentToolInvocationBeginV1) (*application.AgentToolInvocationV1, error) {
	s.begin = begin
	if s.err != nil {
		return nil, s.err
	}
	return &application.AgentToolInvocationV1{InvocationUUID: begin.InvocationUUID, Status: application.AgentToolInvocationStatusRunning}, nil
}

func (s *agentToolAuditStub) Finish(_ context.Context, finish application.AgentToolInvocationFinishV1) error {
	s.finish = finish
	return s.err
}

func (s *agentToolAuditStub) ResolveCommand(_ context.Context, _, _, _ string) (*application.AgentMCPToolCommandV1, error) {
	return s.command, s.err
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
	readTarget string
}

type approvalServiceStub struct {
	requested application.AgentApprovalRequestV1
	resolved  application.AgentApprovalResolutionV1
	consumed  application.AgentApprovalConsumptionV1
}

type approvalGrantResolverStub struct {
	request application.AgentApprovalGrantRequestV1
	grant   application.AgentApprovalV1
	err     error
}

func (s *approvalGrantResolverStub) ResolveGrant(_ context.Context, request application.AgentApprovalGrantRequestV1) (*application.AgentApprovalV1, error) {
	s.request = request
	if s.err != nil {
		return nil, s.err
	}
	return &s.grant, nil
}

type taskControlAuthorizerStub struct {
	taskUUID      string
	principalUUID string
	result        application.AgentTaskControlAuthorizationV1
	err           error
}

type taskTimelineStub struct {
	events []application.AgentTaskTimelineEventV1
	err    error
}

func (s *taskTimelineStub) AppendAgentTaskTimelineEvent(_ context.Context, event application.AgentTaskTimelineEventV1) (uint64, error) {
	s.events = append(s.events, event)
	return uint64(len(s.events)), s.err
}

func (s *taskTimelineStub) ListAgentTaskTimelineEvents(_ context.Context, _ string, afterSeq uint64, limit int) ([]application.AgentTaskTimelineEventV1, error) {
	if s.err != nil {
		return nil, s.err
	}
	result := make([]application.AgentTaskTimelineEventV1, 0, limit)
	for _, event := range s.events {
		if event.EventSeq > afterSeq && len(result) < limit {
			result = append(result, event)
		}
	}
	return result, nil
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

func (s *approvalServiceStub) Consume(_ context.Context, consumption application.AgentApprovalConsumptionV1) error {
	s.consumed = consumption
	return nil
}

func (s *capabilityStub) ListConversations(_ context.Context, invocation application.AgentInvocationV1, limit int) ([]*model.Conversation, error) {
	s.invocation = invocation
	return []*model.Conversation{{ConversationKey: "group:G1", TargetUUID: "G1", LastMessageSeq: uint64(limit)}}, nil
}

func (s *capabilityStub) ReadConversation(_ context.Context, invocation application.AgentInvocationV1, targetUUID string, _ int) (*application.AgentConversationReadV1, error) {
	s.invocation, s.readTarget = invocation, targetUUID
	return &application.AgentConversationReadV1{
		Found: true, TargetUUID: targetUUID, TargetType: model.MessageTargetGroup,
		Messages: []*model.Message{{UUID: "M1", ConversationKey: "group:" + targetUUID, Seq: 7, Content: "evidence"}},
	}, nil
}

func (s *capabilityStub) SearchConversations(_ context.Context, invocation application.AgentInvocationV1, query string, limit int) ([]*application.AgentConversationSearchEvidenceV1, error) {
	s.invocation = invocation
	return []*application.AgentConversationSearchEvidenceV1{{
		MessageUUID: "M1", ConversationKey: "group:G1", MessageSeq: uint64(limit), Revision: 1,
		SenderUUID: "U200", MessageType: model.MessageTypeText, Content: query, SentAt: time.UnixMilli(1_000), QuerySHA256: strings.Repeat("a", 64),
	}}, nil
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

func TestReadConversationResolvesTrustedTaskIdentityAndMapsMessages(t *testing.T) {
	capability := &capabilityStub{}
	server, err := NewServer(capability, resolverStub{invocation: application.AgentInvocationV1{PrincipalUUID: "U100", AgentUUID: "UAI"}}, &admissionStub{})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	response, err := server.ReadConversation(context.Background(), &agentv1.ReadConversationRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), TaskId: "TASK-1", RunId: "RUN-1", TargetId: "G1", Limit: 20,
	})
	if err != nil {
		t.Fatalf("read conversation: %v", err)
	}
	if capability.invocation.PrincipalUUID != "U100" || capability.readTarget != "G1" || !response.GetFound() || len(response.GetMessages()) != 1 || response.GetMessages()[0].GetSequence() != 7 {
		t.Fatalf("unexpected trusted response: invocation=%+v target=%q response=%+v", capability.invocation, capability.readTarget, response)
	}
}

func TestReadConversationRejectsClientPrincipal(t *testing.T) {
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{})
	_, err := server.ReadConversation(context.Background(), &agentv1.ReadConversationRequest{
		Context: grpccommon.RequestContext("U999", "dipole-agent"), TaskId: "TASK-1", RunId: "RUN-1", TargetId: "G1", Limit: 20,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("forged principal code = %s, want %s", status.Code(err), codes.InvalidArgument)
	}
}

func TestSearchConversationsResolvesTrustedTaskIdentityAndMapsEvidence(t *testing.T) {
	capability := &capabilityStub{}
	server, err := NewServer(capability, resolverStub{invocation: application.AgentInvocationV1{PrincipalUUID: "U100", AgentUUID: "UAI"}}, &admissionStub{})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	response, err := server.SearchConversations(context.Background(), &agentv1.SearchConversationsRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), TaskId: "TASK-1", RunId: "RUN-1", Query: "migration", Limit: 20,
	})
	if err != nil || capability.invocation.PrincipalUUID != "U100" || len(response.GetEvidence()) != 1 ||
		response.GetEvidence()[0].GetMessageSeq() != 20 || response.GetEvidence()[0].GetContent() != "migration" {
		t.Fatalf("unexpected trusted search response: invocation=%+v response=%+v err=%v", capability.invocation, response, err)
	}
}

func TestSearchConversationsRejectsClientPrincipalAndInvalidBounds(t *testing.T) {
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{})
	_, err := server.SearchConversations(context.Background(), &agentv1.SearchConversationsRequest{
		Context: grpccommon.RequestContext("U999", "dipole-agent"), TaskId: "TASK-1", RunId: "RUN-1", Query: "migration", Limit: 20,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("forged principal code = %s, want %s", status.Code(err), codes.InvalidArgument)
	}
	_, err = server.SearchConversations(context.Background(), &agentv1.SearchConversationsRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), TaskId: "TASK-1", RunId: "RUN-1", Query: "", Limit: 21,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("invalid bounds code = %s, want %s", status.Code(err), codes.InvalidArgument)
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

func TestListAgentTaskTimelineUsesOwnerAuthorizationAndCursor(t *testing.T) {
	controls := &taskControlAuthorizerStub{result: application.AgentTaskControlAuthorizationV1{
		TaskUUID: "TASK-1", Status: application.AgentTaskStatusRunning,
		Workflow: &application.AgentTaskWorkflowProjectionV1{Revision: 7},
	}}
	timeline := &taskTimelineStub{events: []application.AgentTaskTimelineEventV1{
		{EventSeq: 4, EventUUID: "EV-4", TaskUUID: "TASK-1", Kind: application.AgentTaskTimelineEventArtifact, Status: "created", ArtifactUUID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", OccurredAt: time.UnixMilli(4_000)},
		{EventSeq: 5, EventUUID: "EV-5", TaskUUID: "TASK-1", Kind: application.AgentTaskTimelineEventTerminal, Status: "completed", OccurredAt: time.UnixMilli(5_000)},
	}}
	server, err := NewServerWithControl(&capabilityStub{}, resolverStub{}, &admissionStub{}, &approvalServiceStub{}, controls)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	if _, err = server.WithTaskTimeline(timeline); err != nil {
		t.Fatalf("configure timeline: %v", err)
	}
	response, err := server.ListAgentTaskTimeline(context.Background(), &agentv1.ListAgentTaskTimelineRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), TaskId: "TASK-1", PrincipalUserId: "U100", AfterSeq: 3, Limit: 1,
	})
	if err != nil {
		t.Fatalf("list timeline: %v", err)
	}
	if response.GetSchemaVersion() != application.AgentTaskTimelineSchemaVersionV1 || response.GetRevision() != 7 || len(response.GetEvents()) != 1 || response.GetEvents()[0].GetEventSeq() != 4 || response.GetEvents()[0].GetArtifactId() != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" || response.GetNextCursor() != "4" || controls.principalUUID != "U100" {
		t.Fatalf("unexpected timeline response: response=%+v controls=%+v", response, controls)
	}
}

func TestListAgentTaskTimelineHidesForeignTask(t *testing.T) {
	controls := &taskControlAuthorizerStub{err: errors.New("foreign task")}
	server, err := NewServerWithControl(&capabilityStub{}, resolverStub{}, &admissionStub{}, &approvalServiceStub{}, controls)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	if _, err = server.WithTaskTimeline(&taskTimelineStub{}); err != nil {
		t.Fatalf("configure timeline: %v", err)
	}
	_, err = server.ListAgentTaskTimeline(context.Background(), &agentv1.ListAgentTaskTimelineRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), TaskId: "TASK-1", PrincipalUserId: "U999", Limit: 20,
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("foreign task code = %s, want %s", status.Code(err), codes.NotFound)
	}
}

func TestListAgentTaskTimelineFailsClosedWhenUnconfigured(t *testing.T) {
	server, err := NewServerWithControl(&capabilityStub{}, resolverStub{}, &admissionStub{}, &approvalServiceStub{}, &taskControlAuthorizerStub{})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	_, err = server.ListAgentTaskTimeline(context.Background(), &agentv1.ListAgentTaskTimelineRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), TaskId: "TASK-1", PrincipalUserId: "U100", Limit: 20,
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("unconfigured code = %s, want %s", status.Code(err), codes.FailedPrecondition)
	}
}

func TestAppendAgentTaskTimelineEventRequiresRuntimeAndValidRunBinding(t *testing.T) {
	timeline := &taskTimelineStub{}
	server, err := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	if _, err = server.WithTaskTimeline(timeline); err != nil {
		t.Fatalf("configure timeline: %v", err)
	}
	response, err := server.AppendAgentTaskTimelineEvent(context.Background(), &agentv1.AppendAgentTaskTimelineEventRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), EventId: "MODEL-1", TaskId: "TASK-1", RunId: "RUN-1",
		Kind: "artifact", Status: "created", ArtifactId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", OccurredAtUnixMs: 1_000,
	})
	if err != nil || response.GetEventId() != "MODEL-1" || len(timeline.events) != 1 || timeline.events[0].Kind != application.AgentTaskTimelineEventArtifact || timeline.events[0].ArtifactUUID != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("unexpected append response=%+v events=%+v err=%v", response, timeline.events, err)
	}
	_, err = server.AppendAgentTaskTimelineEvent(context.Background(), &agentv1.AppendAgentTaskTimelineEventRequest{
		Context: grpccommon.RequestContext("U999", "dipole-agent"), EventId: "MODEL-2", TaskId: "TASK-1", RunId: "RUN-1",
		Kind: "model_call", Status: "completed", OccurredAtUnixMs: 1_000,
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("client principal code = %s, want %s", status.Code(err), codes.PermissionDenied)
	}
}

func TestResolveMcpContextUsesPinnedInvocationAndAuthenticatedPrincipal(t *testing.T) {
	invocation := application.AgentInvocationV1{
		TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", DelegatedByUUID: "U100",
		RuntimeID: "dipole-agent", Mode: "active",
		Permissions:          []string{"conversation.list"},
		ResourceScopes:       []application.AgentResourceScopeV1{{ResourceType: "conversation", ResourceID: "*", Actions: []string{"list"}}},
		ApprovedCapabilities: []string{application.AgentCapabilitySystemMessageSend},
	}
	server, err := NewServer(&capabilityStub{}, resolverStub{invocation: invocation}, &admissionStub{})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	response, err := server.ResolveMcpContext(context.Background(), &agentv1.ResolveMcpContextRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), TaskId: "TASK-1", RunId: "RUN-1", PrincipalUserId: "U100",
	})
	if err != nil || response.GetPrincipalUserId() != "U100" || response.GetAgentId() != "UAI" || response.GetRuntimeId() != "dipole-agent" || response.GetMode() != "active" || len(response.GetPermissions()) != 1 || len(response.GetResourceScopes()) != 1 || len(response.GetApprovedCapabilities()) != 1 {
		t.Fatalf("unexpected MCP context: response=%+v err=%v", response, err)
	}
	_, err = server.ResolveMcpContext(context.Background(), &agentv1.ResolveMcpContextRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), TaskId: "TASK-1", RunId: "RUN-1", PrincipalUserId: "U999",
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("foreign principal code = %s, want %s", status.Code(err), codes.NotFound)
	}
	server.resolver = resolverStub{invocation: application.AgentInvocationV1{
		TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", RuntimeID: "forged-runtime", Mode: "active",
		Permissions: []string{"conversation.list"}, ResourceScopes: invocation.ResourceScopes,
	}}
	_, err = server.ResolveMcpContext(context.Background(), &agentv1.ResolveMcpContextRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), TaskId: "TASK-1", RunId: "RUN-1", PrincipalUserId: "U100",
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("forged Runtime code = %s, want %s", status.Code(err), codes.NotFound)
	}
	server.resolver = resolverStub{invocation: application.AgentInvocationV1{TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", RuntimeID: "dipole-agent", Mode: "shadow", Permissions: []string{"conversation.list"}, ResourceScopes: invocation.ResourceScopes, ApprovedCapabilities: []string{application.AgentCapabilitySystemMessageSend}}}
	_, err = server.ResolveMcpContext(context.Background(), &agentv1.ResolveMcpContextRequest{Context: grpccommon.RequestContext("", "dipole-agent"), TaskId: "TASK-1", RunId: "RUN-1", PrincipalUserId: "U100"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("shadow approved Capability code = %s, want %s", status.Code(err), codes.NotFound)
	}
}

func TestMcpToolInvocationAuditUsesAuthenticatedRuntimeContext(t *testing.T) {
	audit := &agentToolAuditStub{command: &application.AgentMCPToolCommandV1{
		InvocationUUID: "INV-1", TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", TaskUUID: "TASK-1", RunUUID: "RUN-1",
		ProfileID: "calendar-prod", ServerID: "calendar.example", ToolName: "calendar.create", CapabilityID: application.AgentCapabilityConversationsList,
		ArgumentsJSON: `{"calendarId":"CAL-1"}`, ArgumentsSHA256: strings.Repeat("c", 64),
		StartedAt: time.UnixMilli(1_000),
	}}
	server, err := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	if _, err = server.WithToolAudits(audit); err != nil {
		t.Fatalf("configure Tool audit: %v", err)
	}
	timeline := &taskTimelineStub{}
	if _, err = server.WithTaskTimeline(timeline); err != nil {
		t.Fatalf("configure timeline: %v", err)
	}
	requestContext := grpccommon.RequestContext("", "dipole-agent")
	requestContext.RequestId, requestContext.TraceId = "REQ-1", "TRACE-1"
	response, err := server.BeginMcpToolInvocation(context.Background(), &agentv1.BeginMcpToolInvocationRequest{
		Context: requestContext, TaskId: "TASK-1", RunId: "RUN-1", InvocationId: "INV-1",
		ToolName: "dipole_conversation_list", CapabilityId: application.AgentCapabilityConversationsList,
		ArgumentsSha256: strings.Repeat("a", 64), ApprovalId: "APR-1",
	})
	if err != nil || response.GetStatus() != "running" || audit.begin.RequestID != "REQ-1" || audit.begin.Transport != application.AgentToolTransportMCP || audit.begin.ApprovalUUID != "APR-1" {
		t.Fatalf("unexpected Tool begin: response=%+v audit=%+v err=%v", response, audit.begin, err)
	}
	if len(timeline.events) != 1 || timeline.events[0].Kind != application.AgentTaskTimelineEventToolInvocation || timeline.events[0].Status != "running" {
		t.Fatalf("unexpected Tool timeline begin: %+v", timeline.events)
	}
	command, err := server.ResolveMcpToolCommand(context.Background(), &agentv1.ResolveMcpToolCommandRequest{
		Context: requestContext, TaskId: "TASK-1", RunId: "RUN-1", InvocationId: "INV-1",
	})
	if err != nil || command.GetProfileId() != "calendar-prod" || string(command.GetArgumentsJson()) != `{"calendarId":"CAL-1"}` || command.GetPrincipalUserId() != "U100" || command.GetStartedAtUnixMs() != 1_000 {
		t.Fatalf("unexpected Tool command: response=%+v err=%v", command, err)
	}
	finishResponse, err := server.FinishMcpToolInvocation(context.Background(), &agentv1.FinishMcpToolInvocationRequest{
		Context: requestContext, TaskId: "TASK-1", RunId: "RUN-1", InvocationId: "INV-1", Status: "completed",
		ResultSha256: strings.Repeat("b", 64), ResultBytes: 128, LatencyMs: 12,
		ActionReference: &agentv1.AgentToolActionReference{ResourceType: "message", ResourceId: "MSG-1", CommandKind: "system_message", CommandId: "CMD-1"},
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("external direct finish code = %s, want %s", status.Code(err), codes.PermissionDenied)
	}
	audit.command.ProfileID, audit.command.ServerID, audit.command.ArgumentsJSON = "", "", ""
	finishResponse, err = server.FinishMcpToolInvocation(context.Background(), &agentv1.FinishMcpToolInvocationRequest{
		Context: requestContext, TaskId: "TASK-1", RunId: "RUN-1", InvocationId: "INV-1", Status: "completed",
		ResultSha256: strings.Repeat("b", 64), ResultBytes: 128, LatencyMs: 12,
		ActionReference: &agentv1.AgentToolActionReference{ResourceType: "message", ResourceId: "MSG-1", CommandKind: "system_message", CommandId: "CMD-1"},
	})
	if err != nil || finishResponse.GetStatus() != "completed" || audit.finish.ResultBytes != 128 || audit.finish.ActionReference == nil || audit.finish.ActionReference.ResourceUUID != "MSG-1" {
		t.Fatalf("unexpected Tool finish: response=%+v audit=%+v err=%v", finishResponse, audit.finish, err)
	}
	if len(timeline.events) != 2 || timeline.events[1].Status != "completed" {
		t.Fatalf("unexpected Tool timeline finish: %+v", timeline.events)
	}
	_, err = server.BeginMcpToolInvocation(context.Background(), &agentv1.BeginMcpToolInvocationRequest{
		Context: grpccommon.RequestContext("U999", "dipole-agent"),
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("client principal code = %s, want %s", status.Code(err), codes.PermissionDenied)
	}
	audit.err = application.ErrAgentToolInvocationConflict
	_, err = server.FinishMcpToolInvocation(context.Background(), &agentv1.FinishMcpToolInvocationRequest{Context: requestContext})
	if status.Code(err) != codes.Aborted {
		t.Fatalf("conflict code = %s, want %s", status.Code(err), codes.Aborted)
	}
}

func TestMcpToolRoundReceiptUsesAuthenticatedRuntimeContext(t *testing.T) {
	rounds := &agentMCPToolRoundStub{result: &application.AgentMCPToolRoundClaimResultV1{
		Outcome: application.AgentMCPToolRoundReplayCompleted, ResultJSON: `{"content":[]}`, ResultSHA256: strings.Repeat("d", 64),
	}}
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{})
	if _, err := server.WithMCPToolRounds(rounds); err != nil {
		t.Fatalf("configure MCP Tool rounds: %v", err)
	}
	requestContext := grpccommon.RequestContext("", "dipole-agent")
	claimResponse, err := server.ClaimMcpToolRound(context.Background(), &agentv1.ClaimMcpToolRoundRequest{
		Context: requestContext, TaskId: "TASK-1", RunId: "RUN-1", InvocationId: "INV-1", RoundId: strings.Repeat("a", 64),
		RoundNumber: 1, RequestSha256: strings.Repeat("b", 64), OwnerTokenSha256: strings.Repeat("c", 64),
	})
	if err != nil || claimResponse.GetOutcome() != "replay_completed" || rounds.claim.RoundNumber != 1 || string(claimResponse.GetResultJson()) != `{"content":[]}` {
		t.Fatalf("unexpected Tool round claim: response=%+v claim=%+v err=%v", claimResponse, rounds.claim, err)
	}
	finishResponse, err := server.FinishMcpToolRound(context.Background(), &agentv1.FinishMcpToolRoundRequest{
		Context: requestContext, RoundId: strings.Repeat("a", 64), OwnerTokenSha256: strings.Repeat("c", 64),
		Status: "failed", ErrorCode: "transport_unavailable",
	})
	if err != nil || finishResponse.GetStatus() != "failed" || rounds.finish.ErrorCode != "transport_unavailable" {
		t.Fatalf("unexpected Tool round finish: response=%+v finish=%+v err=%v", finishResponse, rounds.finish, err)
	}
	_, err = server.ClaimMcpToolRound(context.Background(), &agentv1.ClaimMcpToolRoundRequest{
		Context: grpccommon.RequestContext("U999", "dipole-agent"),
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("client principal code = %s, want %s", status.Code(err), codes.PermissionDenied)
	}
	rounds.err = application.ErrAgentMCPToolRoundConflict
	_, err = server.FinishMcpToolRound(context.Background(), &agentv1.FinishMcpToolRoundRequest{Context: requestContext})
	if status.Code(err) != codes.Aborted {
		t.Fatalf("round conflict code = %s, want %s", status.Code(err), codes.Aborted)
	}
}

func TestMcpToolInvocationTerminalUsesOnlyBoundRoundEvidence(t *testing.T) {
	terminal := &agentMCPToolTerminalStub{invocation: &application.AgentToolInvocationV1{
		InvocationUUID: "INV-1", Status: application.AgentToolInvocationStatusCompleted,
	}}
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{})
	if _, err := server.WithMCPToolTerminals(terminal); err != nil {
		t.Fatalf("configure MCP Tool terminal: %v", err)
	}
	requestContext := grpccommon.RequestContext("", "dipole-agent")
	response, err := server.FinishMcpToolInvocationFromRound(context.Background(), &agentv1.FinishMcpToolInvocationFromRoundRequest{
		Context: requestContext, TaskId: "TASK-1", RunId: "RUN-1", InvocationId: "INV-1", RoundId: strings.Repeat("a", 64),
	})
	if err != nil || response.GetStatus() != "completed" || terminal.request.RoundUUID != strings.Repeat("a", 64) || terminal.request.InvocationUUID != "INV-1" {
		t.Fatalf("terminal response=%+v request=%+v err=%v", response, terminal.request, err)
	}
	_, err = server.FinishMcpToolInvocationFromRound(context.Background(), &agentv1.FinishMcpToolInvocationFromRoundRequest{
		Context: grpccommon.RequestContext("U999", "dipole-agent"),
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("client principal code = %s, want %s", status.Code(err), codes.PermissionDenied)
	}
	terminal.err = application.ErrAgentMCPToolRoundConflict
	_, err = server.FinishMcpToolInvocationFromRound(context.Background(), &agentv1.FinishMcpToolInvocationFromRoundRequest{Context: requestContext})
	if status.Code(err) != codes.Aborted {
		t.Fatalf("terminal conflict code = %s, want %s", status.Code(err), codes.Aborted)
	}
}

func TestExecuteMcpMessageCommandUsesBoundRuntimeService(t *testing.T) {
	commands := &agentMessageCommandExecutionStub{result: application.AgentMessageCommandExecutionResultV1{
		MessageUUID: "MSG-1", ClientMessageID: strings.Repeat("a", 64), CommandID: "tool:command-1", Kind: application.AgentMessageCommandSystemMessageV1,
	}}
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{})
	if _, err := server.WithMessageCommands(commands); err != nil {
		t.Fatalf("configure Message Commands: %v", err)
	}
	response, err := server.ExecuteMcpMessageCommand(context.Background(), &agentv1.ExecuteMcpMessageCommandRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), TaskId: "TASK-1", RunId: "RUN-1", InvocationId: "INV-1",
		CommandKind: "system_message", Content: "notice",
	})
	if err != nil || response.GetActionReference().GetResourceId() != "MSG-1" || response.GetClientMessageId() != strings.Repeat("a", 64) ||
		commands.request.InvocationUUID != "INV-1" || commands.request.Content != "notice" {
		t.Fatalf("unexpected Message Command response=%+v request=%+v err=%v", response, commands.request, err)
	}
	_, err = server.ExecuteMcpMessageCommand(context.Background(), &agentv1.ExecuteMcpMessageCommandRequest{
		Context: grpccommon.RequestContext("U999", "dipole-agent"),
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("forged principal code = %s", status.Code(err))
	}
	commands.err = application.ErrAgentCommandDenied
	_, err = server.ExecuteMcpMessageCommand(context.Background(), &agentv1.ExecuteMcpMessageCommandRequest{Context: grpccommon.RequestContext("", "dipole-agent")})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("denied command code = %s", status.Code(err))
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

func TestAdmitRunForwardsActiveRuntimeBinding(t *testing.T) {
	admission := &admissionStub{result: application.AgentRunAdmissionV1{TaskUUID: "TASK-1", RunUUID: "RUN-1", RunStatus: application.AgentRunStatusRunning}}
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, admission)
	_, err := server.AdmitRun(context.Background(), &agentv1.AdmitRunRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), TenantId: "dipole", PrincipalUserId: "U100",
		AgentId: "UAI", TriggerType: "message.direct.created", TriggerRef: "M100", EventId: "E1",
		RuntimeId: "dipole-agent", Mode: "active", CandidateVersion: "candidate-v1",
	})
	if err != nil {
		t.Fatalf("active admission: %v", err)
	}
	if admission.admitted.RuntimeID != "dipole-agent" || admission.admitted.Mode != "active" || admission.admitted.CandidateVersion != "candidate-v1" {
		t.Fatalf("active runtime binding was not forwarded: %+v", admission.admitted)
	}
}

func TestMatchEventSubscriptionsUsesAuthenticatedRuntimeIdentity(t *testing.T) {
	resolver := &eventSubscriptionResolverStub{items: []application.AgentEventSubscriptionV1{{
		SubscriptionUUID: "SUB-1", DefinitionUUID: "DEF-1", DefinitionVersion: 2,
		TenantID: "dipole", AgentUUID: "UAI", Status: application.AgentSubscriptionStatusActive,
		EventType: "message.direct.created", ResourceType: "conversation", ResourceID: "group:G1",
		FilterKind: application.AgentSubscriptionFilterMessageContainsAny, FilterJSON: []byte(`{"terms":["incident"]}`), CreatedByUUID: "U100",
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

func TestEventSubscriptionControlRPCUsesAuthenticatedGatewayPrincipal(t *testing.T) {
	control := &eventSubscriptionControlStub{item: application.AgentEventSubscriptionV1{
		SubscriptionUUID: strings.Repeat("a", 64), DefinitionUUID: "DEF-1", DefinitionVersion: 2,
		TenantID: "dipole", AgentUUID: "UAI", Status: application.AgentSubscriptionStatusActive,
		EventType: "message.direct.created", ResourceType: "conversation", ResourceID: "group:G1",
		FilterKind: application.AgentSubscriptionFilterAll, FilterJSON: []byte(`{}`), CreatedByUUID: "U100",
		CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0),
	}}
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{})
	server, _ = server.WithEventSubscriptionControls(control)
	requestContext := grpccommon.RequestContext("U100", "dipole-gateway")
	created, err := invokeAuthenticatedAgentRPC(t, "dipole-gateway", func(ctx context.Context) (any, error) {
		return server.CreateEventSubscription(ctx, &agentv1.CreateEventSubscriptionRequest{
			Context: requestContext, TenantId: "dipole", DefinitionId: "DEF-1", DefinitionVersion: 2,
			EventType: "message.direct.created", ResourceType: "conversation", ResourceId: "group:G1",
			FilterKind: "all", FilterJson: []byte(`{}`),
		})
	})
	if err != nil || created.(*agentv1.AgentEventSubscription).GetCreatedById() != "U100" || control.principal != "U100" || control.created.DefinitionVersion != 2 {
		t.Fatalf("create response=%+v control=%+v err=%v", created, control, err)
	}
	options, err := invokeAuthenticatedAgentRPC(t, "dipole-gateway", func(ctx context.Context) (any, error) {
		return server.ListEligibleSubscriptionConversations(ctx, &agentv1.ListEligibleSubscriptionConversationsRequest{
			Context: requestContext, TenantId: "dipole", DefinitionId: "DEF-1", DefinitionVersion: 2,
		})
	})
	listedOptions := options.(*agentv1.ListEligibleSubscriptionConversationsResponse)
	if err != nil || control.options.DefinitionUUID != "DEF-1" || len(listedOptions.GetConversations()) != 1 || listedOptions.GetConversations()[0].GetConversationKey() != "group:G1" {
		t.Fatalf("options response=%+v control=%+v err=%v", options, control, err)
	}
	listed, err := invokeAuthenticatedAgentRPC(t, "dipole-gateway", func(ctx context.Context) (any, error) {
		return server.ListEventSubscriptions(ctx, &agentv1.ListEventSubscriptionsRequest{Context: requestContext, TenantId: "dipole", Limit: 20})
	})
	if err != nil || len(listed.(*agentv1.ListEventSubscriptionsResponse).GetSubscriptions()) != 1 || listed.(*agentv1.ListEventSubscriptionsResponse).GetNextCursor() != "NEXT" || control.listed.Limit != 20 {
		t.Fatalf("list response=%+v control=%+v err=%v", listed, control, err)
	}
	revoked, err := invokeAuthenticatedAgentRPC(t, "dipole-gateway", func(ctx context.Context) (any, error) {
		return server.RevokeEventSubscription(ctx, &agentv1.RevokeEventSubscriptionRequest{Context: requestContext, TenantId: "dipole", SubscriptionId: control.item.SubscriptionUUID, Reason: "retired"})
	})
	if err != nil || revoked.(*agentv1.AgentEventSubscription).GetRevokeReason() != "retired" || control.revoked.SubscriptionUUID != control.item.SubscriptionUUID {
		t.Fatalf("revoke response=%+v control=%+v err=%v", revoked, control, err)
	}
	requestContext.CallerService = "dipole-agent"
	if _, err := invokeAuthenticatedAgentRPC(t, "dipole-agent", func(ctx context.Context) (any, error) {
		return server.ListEventSubscriptions(ctx, &agentv1.ListEventSubscriptionsRequest{Context: requestContext, TenantId: "dipole"})
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("Agent data-plane management code = %s", status.Code(err))
	}
}

func TestAgentDefinitionCatalogRPCUsesAuthenticatedGatewayPrincipal(t *testing.T) {
	catalog := &agentDefinitionCatalogStub{page: application.AgentDefinitionCatalogPageV1{
		Definitions: []application.AgentDefinitionCatalogItemV1{{
			DefinitionUUID: "DEF-1", Version: 2, AgentUUID: "UAI", ConversationScopes: []string{"group:G1"},
			ValidFrom: time.Unix(1, 0), CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(2, 0),
		}},
		NextDefinitionUUID: "DEF-1", NextVersion: 2,
	}}
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{})
	server, _ = server.WithDefinitionCatalog(catalog)
	requestContext := grpccommon.RequestContext("U100", "dipole-gateway")
	response, err := invokeAuthenticatedAgentRPC(t, "dipole-gateway", func(ctx context.Context) (any, error) {
		return server.ListAgentDefinitions(ctx, &agentv1.ListAgentDefinitionsRequest{
			Context: requestContext, TenantId: "dipole", AfterDefinitionId: "DEF-0", AfterVersion: 1, Limit: 20,
		})
	})
	if err != nil {
		t.Fatalf("list Agent Definitions: %v", err)
	}
	listed := response.(*agentv1.ListAgentDefinitionsResponse)
	if catalog.principal != "U100" || catalog.request.AfterDefinitionUUID != "DEF-0" || catalog.request.AfterVersion != 1 || catalog.request.Limit != 20 ||
		len(listed.GetDefinitions()) != 1 || listed.GetDefinitions()[0].GetDefinitionId() != "DEF-1" || listed.GetNextVersion() != 2 {
		t.Fatalf("unexpected Definition catalog request=%+v response=%+v", catalog, listed)
	}
	requestContext.CallerService = "dipole-agent"
	if _, err := invokeAuthenticatedAgentRPC(t, "dipole-agent", func(ctx context.Context) (any, error) {
		return server.ListAgentDefinitions(ctx, &agentv1.ListAgentDefinitionsRequest{Context: requestContext, TenantId: "dipole"})
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("Agent data-plane catalog code = %s", status.Code(err))
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

func TestMemoryOwnerRPCUsesAuthenticatedGatewayPrincipalAndOmitsSourceURI(t *testing.T) {
	now := time.UnixMilli(1_700_000_000_000).UTC()
	item := application.AgentMemoryV1{
		MemoryUUID: "MEM-1", TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI",
		MemoryRootUUID: "MEM-1", MemoryVersion: 1,
		MemoryType: application.AgentMemoryTypeSemantic, Status: application.AgentMemoryStatusActive,
		ResourceType: "conversation", ResourceID: "group:G1", Content: "Owner is Alice", CompactContent: "Owner: Alice", Priority: 80,
		Provenance: application.AgentMemoryProvenanceV1{SourceType: "message", SourceID: "M1", URI: "mysql://internal/secret", Sequence: "42"},
		ValidFrom:  now.Add(-time.Hour), CreatedAt: now.Add(-time.Minute),
	}
	control := &agentMemoryOwnerControlStub{page: application.AgentMemoryOwnerPageV1{
		Memories: []application.AgentMemoryV1{item}, NextCreatedAt: item.CreatedAt, NextMemoryUUID: item.MemoryUUID,
	}}
	control.item = item
	revokedAt := now
	control.item.Status, control.item.RevokedAt = application.AgentMemoryStatusRevoked, &revokedAt
	control.item.RevokedByUUID, control.item.RevokeReason = "U100", "outdated"
	control.correction = application.AgentMemoryOwnerCorrectionResultV1{
		Previous: control.item,
		Corrected: application.AgentMemoryV1{
			MemoryUUID: "MEM-2", MemoryRootUUID: "MEM-1", MemoryVersion: 2, SupersedesMemoryUUID: "MEM-1",
			TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", CorrectedByUUID: "U100", CorrectionReason: "fix owner",
			MemoryType: application.AgentMemoryTypeSemantic, Status: application.AgentMemoryStatusActive,
			ResourceType: "conversation", ResourceID: "group:G1", Content: "Owner is Bob", CompactContent: "Owner: Bob", Priority: 80,
			Provenance: application.AgentMemoryProvenanceV1{SourceType: application.AgentMemorySourceOwnerCorrectionV1, SourceID: "MEM-1", Sequence: "2", URI: "mysql://internal/corrected"},
			ValidFrom:  now, CreatedAt: now,
		},
	}
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{})
	server, _ = server.WithMemoryOwnerControls(control)

	response, err := invokeAuthenticatedAgentRPC(t, "dipole-gateway", func(ctx context.Context) (any, error) {
		return server.ListOwnedMemories(ctx, &agentv1.ListOwnedMemoriesRequest{
			Context: grpccommon.RequestContext("U100", "dipole-gateway"), TenantId: "dipole", Limit: 20,
		})
	})
	if err != nil {
		t.Fatalf("list owned Memories: %v", err)
	}
	listed := response.(*agentv1.ListOwnedMemoriesResponse)
	if control.listRequest.PrincipalUUID != "U100" || control.listRequest.TenantID != "dipole" || control.listRequest.Limit != 20 ||
		len(listed.GetMemories()) != 1 || listed.GetMemories()[0].GetProvenance().GetUri() != "" || listed.GetNextMemoryId() != "MEM-1" {
		t.Fatalf("unexpected Memory request=%+v response=%+v", control.listRequest, listed)
	}
	_, err = invokeAuthenticatedAgentRPC(t, "dipole-gateway", func(ctx context.Context) (any, error) {
		return server.RevokeOwnedMemory(ctx, &agentv1.RevokeOwnedMemoryRequest{
			Context: grpccommon.RequestContext("U100", "dipole-gateway"), TenantId: "dipole", MemoryId: "MEM-1", Reason: "outdated",
		})
	})
	if err != nil || control.revokeRequest.PrincipalUUID != "U100" || control.revokeRequest.Reason != "outdated" {
		t.Fatalf("revoke owned Memory request=%+v err=%v", control.revokeRequest, err)
	}
	response, err = invokeAuthenticatedAgentRPC(t, "dipole-gateway", func(ctx context.Context) (any, error) {
		return server.CorrectOwnedMemory(ctx, &agentv1.CorrectOwnedMemoryRequest{
			Context: grpccommon.RequestContext("U100", "dipole-gateway"), TenantId: "dipole", MemoryId: "MEM-1",
			ExpectedVersion: 1, Content: "Owner is Bob", CompactContent: "Owner: Bob", Reason: "fix owner",
		})
	})
	corrected := response.(*agentv1.CorrectOwnedMemoryResponse)
	if err != nil || control.correctionRequest.PrincipalUUID != "U100" || control.correctionRequest.ExpectedVersion != 1 ||
		corrected.GetPrevious().GetStatus() != "revoked" || corrected.GetCorrected().GetMemoryVersion() != 2 ||
		corrected.GetCorrected().GetMemoryRootId() != "MEM-1" || corrected.GetCorrected().GetProvenance().GetUri() != "" {
		t.Fatalf("correct owned Memory request=%+v response=%+v err=%v", control.correctionRequest, corrected, err)
	}
	if _, err = invokeAuthenticatedAgentRPC(t, "dipole-agent", func(ctx context.Context) (any, error) {
		return server.ListOwnedMemories(ctx, &agentv1.ListOwnedMemoriesRequest{Context: grpccommon.RequestContext("U100", "dipole-agent"), TenantId: "dipole"})
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("Agent data-plane owner Memory code = %s", status.Code(err))
	}
}

func TestPromoteMemoryCandidateBindsGatewayPrincipal(t *testing.T) {
	now := time.UnixMilli(1_700_000_000_000).UTC()
	promotion := &agentMemoryCandidatePromotionStub{item: application.AgentMemoryV1{
		MemoryUUID: "MEM-CAND-1", TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI",
		MemoryType: application.AgentMemoryTypeObservational, Status: application.AgentMemoryStatusActive,
		ResourceType: "conversation", ResourceID: "group:G1", Content: "Decision", CompactContent: "Decision", Priority: 60,
		Provenance: application.AgentMemoryProvenanceV1{SourceType: "memory_candidate", SourceID: "CAND-1", Sequence: "REV-1"},
		ValidFrom:  now, CreatedAt: now,
	}}
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{})
	server, _ = server.WithMemoryCandidatePromotions(promotion)
	response, err := invokeAuthenticatedAgentRPC(t, "dipole-gateway", func(ctx context.Context) (any, error) {
		return server.PromoteMemoryCandidate(ctx, &agentv1.PromoteMemoryCandidateRequest{
			Context: grpccommon.RequestContext("U100", "dipole-gateway"), TenantId: "dipole", CandidateId: "CAND-1",
			CandidateSha256: strings.Repeat("a", 64), ReviewId: "REV-1", TargetMemoryType: "semantic",
		})
	})
	if err != nil {
		t.Fatalf("promote candidate: %v", err)
	}
	item := response.(*agentv1.AgentOwnedMemory)
	if promotion.request.PrincipalUUID != "U100" || promotion.request.TenantID != "dipole" || promotion.request.CandidateUUID != "CAND-1" || promotion.request.TargetMemoryType != application.AgentMemoryTypeSemantic || item.GetMemoryId() != "MEM-CAND-1" {
		t.Fatalf("unexpected promotion request=%+v response=%+v", promotion.request, item)
	}
	_, err = invokeAuthenticatedAgentRPC(t, "dipole-agent", func(ctx context.Context) (any, error) {
		return server.PromoteMemoryCandidate(ctx, &agentv1.PromoteMemoryCandidateRequest{Context: grpccommon.RequestContext("U100", "dipole-agent"), TenantId: "dipole"})
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("Agent candidate promotion code = %s", status.Code(err))
	}
}

func TestCommitMemoryPromotionReceiptIsAgentBoundAndOptIn(t *testing.T) {
	now := time.UnixMilli(1_700_000_000_000).UTC()
	commit := &agentMemoryPromotionReceiptCommitStub{item: application.AgentMemoryV1{
		MemoryUUID: "MEM-CAND-1", TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI",
		MemoryType: application.AgentMemoryTypeSemantic, Status: application.AgentMemoryStatusActive,
		ResourceType: "conversation", ResourceID: "group:G1", Content: "Decision", CompactContent: "Decision", Priority: 60,
		Provenance: application.AgentMemoryProvenanceV1{SourceType: "memory_candidate", SourceID: "CAND-1", Sequence: "REV-1"}, ValidFrom: now, CreatedAt: now,
	}}
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{})
	request := &agentv1.CommitMemoryPromotionReceiptRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), ReceiptId: "MEM-PROMOTE-" + strings.Repeat("a", 64), ReceiptSha256: strings.Repeat("a", 64),
		SchemaVersion: application.AgentMemoryPromotionReceiptSchemaV2, Status: application.AgentMemoryPromotionReceiptPrepared,
		TaskId: "TASK-1", RunId: "RUN-1", CandidateId: "CAND-1", CandidateSha256: strings.Repeat("b", 64), ReviewId: "REV-1", PolicyVersion: "memory-v1", TargetMemoryType: "semantic",
		CreatedAtUnixMs: now.UnixMilli(), ExpiresAtUnixMs: now.Add(time.Minute).UnixMilli(),
	}
	_, err := invokeAuthenticatedAgentRPC(t, "dipole-agent", func(ctx context.Context) (any, error) { return server.CommitMemoryPromotionReceipt(ctx, request) })
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("unconfigured commit code = %s", status.Code(err))
	}
	server, _ = server.WithMemoryPromotionReceiptCommits(commit)
	response, err := invokeAuthenticatedAgentRPC(t, "dipole-agent", func(ctx context.Context) (any, error) { return server.CommitMemoryPromotionReceipt(ctx, request) })
	if err != nil || response.(*agentv1.CommitMemoryPromotionReceiptResponse).GetMemoryId() != "MEM-CAND-1" || response.(*agentv1.CommitMemoryPromotionReceiptResponse).GetReceiptSha256() != request.ReceiptSha256 || commit.request.TaskUUID != "TASK-1" || commit.request.TargetMemoryType != application.AgentMemoryTypeSemantic {
		t.Fatalf("commit response=%+v request=%+v err=%v", response, commit.request, err)
	}
	request.Context = grpccommon.RequestContext("", "dipole-gateway")
	_, err = invokeAuthenticatedAgentRPC(t, "dipole-gateway", func(ctx context.Context) (any, error) { return server.CommitMemoryPromotionReceipt(ctx, request) })
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("Gateway commit code = %s", status.Code(err))
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
	timeline := &taskTimelineStub{}
	server, _ = server.WithTaskTimeline(timeline)
	response, err := server.RequestApproval(context.Background(), &agentv1.RequestApprovalRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), TaskId: "TASK-1", RunId: "RUN-1", ApprovalId: "APR-1",
		CapabilityId: "message.bulk.send", ResourceScope: &agentv1.AgentResourceScope{ResourceType: "conversation", ResourceId: "G1", Actions: []string{"write"}},
		ScopeSha256: strings.Repeat("a", 64), ArgumentsSha256: strings.Repeat("b", 64), NonceSha256: strings.Repeat("c", 64), ExpiresAtUnixMs: time.Now().Add(time.Hour).UnixMilli(),
	})
	if err != nil || response.GetStatus() != "pending" || approvals.requested.RuntimeID != "dipole-agent" || approvals.requested.Mode != "shadow" {
		t.Fatalf("request Approval response=%+v request=%+v err=%v", response, approvals.requested, err)
	}
	if len(timeline.events) != 1 || timeline.events[0].Kind != application.AgentTaskTimelineEventApproval || timeline.events[0].Status != "pending" {
		t.Fatalf("unexpected Approval timeline request: %+v", timeline.events)
	}
	resolved, err := server.ResolveApproval(context.Background(), &agentv1.ResolveApprovalRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), TaskId: "TASK-1", RunId: "RUN-1", ApprovalId: "APR-1", ActorUserId: "U100", Decision: "approved",
	})
	if err != nil || resolved.GetStatus() != "approved" || approvals.resolved.ActorUUID != "U100" || approvals.resolved.Decision != application.AgentApprovalDecisionApproved {
		t.Fatalf("resolve Approval response=%+v resolution=%+v err=%v", resolved, approvals.resolved, err)
	}
	if len(timeline.events) != 2 || timeline.events[1].Kind != application.AgentTaskTimelineEventApproval || timeline.events[1].Status != "approved" {
		t.Fatalf("unexpected Approval timeline resolution: %+v", timeline.events)
	}
}

func TestConsumeApprovalRPCRequiresActiveModeAndExactClaim(t *testing.T) {
	approvals := &approvalServiceStub{}
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{}, approvals)
	request := &agentv1.ConsumeApprovalRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), TaskId: "TASK-1", RunId: "RUN-1", ApprovalId: "APR-1",
		CapabilityId: "message.system.send", ScopeSha256: strings.Repeat("a", 64), ArgumentsSha256: strings.Repeat("b", 64),
		NonceSha256: strings.Repeat("c", 64), Mode: "active",
	}
	response, err := server.ConsumeApproval(context.Background(), request)
	if err != nil || response.GetStatus() != "consumed" || approvals.consumed.RuntimeID != "dipole-agent" || approvals.consumed.Mode != "active" ||
		approvals.consumed.Claim.ArgumentsSHA256 != strings.Repeat("b", 64) {
		t.Fatalf("consume Approval response=%+v consumption=%+v err=%v", response, approvals.consumed, err)
	}
	request.Mode = "shadow"
	if _, err := server.ConsumeApproval(context.Background(), request); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("shadow Approval consumption code = %s, want %s", status.Code(err), codes.InvalidArgument)
	}
	request.Mode = "active"
	request.Context = grpccommon.RequestContext("", "dipole-gateway")
	if _, err := server.ConsumeApproval(context.Background(), request); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("forged caller Approval consumption code = %s, want %s", status.Code(err), codes.PermissionDenied)
	}
}

func TestResolveApprovalGrantRPCUsesActiveServerIdentityAndHidesDenial(t *testing.T) {
	scope := application.AgentResourceScopeV1{ResourceType: "conversation", ResourceID: "direct:U100:UAI", Actions: []string{"write"}}
	resolver := &approvalGrantResolverStub{grant: application.AgentApprovalV1{
		ApprovalUUID: "APR-1", CapabilityID: "message.system.send", ResourceScope: scope,
		ScopeSHA256: strings.Repeat("a", 64), ArgumentsSHA256: strings.Repeat("b", 64), NonceSHA256: strings.Repeat("c", 64),
		ExpiresAt: time.Unix(100, 0),
	}}
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{})
	if _, err := server.WithApprovalGrants(resolver); err != nil {
		t.Fatalf("configure grant resolver: %v", err)
	}
	response, err := server.ResolveApprovalGrant(context.Background(), &agentv1.ResolveApprovalGrantRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), TaskId: "TASK-1", RunId: "RUN-1", CapabilityId: "message.system.send",
		ResourceScope:   &agentv1.AgentResourceScope{ResourceType: scope.ResourceType, ResourceId: scope.ResourceID, Actions: scope.Actions},
		ArgumentsSha256: strings.Repeat("b", 64),
	})
	if err != nil || response.GetApprovalId() != "APR-1" || response.GetNonceSha256() != strings.Repeat("c", 64) ||
		resolver.request.RuntimeID != "dipole-agent" || resolver.request.Mode != "active" {
		t.Fatalf("response=%+v request=%+v err=%v", response, resolver.request, err)
	}
	resolver.err = application.ErrAgentApprovalDenied
	if _, err := server.ResolveApprovalGrant(context.Background(), &agentv1.ResolveApprovalGrantRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), TaskId: "TASK-1", RunId: "RUN-1", CapabilityId: "message.system.send",
		ResourceScope: &agentv1.AgentResourceScope{}, ArgumentsSha256: strings.Repeat("b", 64),
	}); status.Code(err) != codes.NotFound {
		t.Fatalf("denied grant code = %s, want NotFound", status.Code(err))
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

func TestWorkflowRepairExecutionRPCIsOptInAndGatewayBound(t *testing.T) {
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{})
	request := &agentv1.ExecuteWorkflowRepairRequest{
		Context: grpccommon.RequestContext("U-OPS", "dipole-gateway"), ExecutionId: "repair-execution:" + strings.Repeat("a", 64), TaskId: "TASK-1",
	}
	if _, err := server.ExecuteWorkflowRepair(context.Background(), request); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("direct execution code = %s, want PermissionDenied", status.Code(err))
	}
	_, err := invokeAuthenticatedAgentRPC(t, "dipole-gateway", func(ctx context.Context) (any, error) {
		return server.ExecuteWorkflowRepair(ctx, request)
	})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("unconfigured execution code = %s, want Unavailable", status.Code(err))
	}
}

func TestRuntimePromotionControlRPCUsesAuthenticatedGatewayPrincipal(t *testing.T) {
	controls := &runtimePromotionControlServiceStub{proposal: &application.AgentRuntimePromotionProposalV1{ProposalUUID: strings.Repeat("a", 64), TenantID: "dipole", RuntimeID: "dipole-agent", CandidateVersion: "candidate-v1", DefinitionUUID: "DEF-1", DefinitionVersion: 1, EvidenceArtifactUUID: strings.Repeat("1", 64), EvidenceSHA256: strings.Repeat("2", 64), EvalSuiteSHA256: strings.Repeat("3", 64), ProposerUUID: "U-OPS", Status: application.AgentRuntimePromotionProposalProposed, ProposedAt: time.Unix(1, 0), ExpiresAt: time.Unix(2, 0), GrantValidFrom: time.Unix(1, 0), GrantExpiresAt: time.Unix(3, 0)}}
	evidence := &runtimePromotionEvidenceServiceStub{review: &application.AgentRuntimePromotionEvidenceReviewV1{
		Proposal: controls.proposal,
		Artifact: &application.AgentArtifactV1{ArtifactUUID: controls.proposal.EvidenceArtifactUUID, SchemaVersion: application.AgentArtifactSchemaVersionV1, TaskUUID: "TASK-1", RunUUID: "RUN-1", ArtifactType: "promotion_evaluation", Version: 1, Title: "Agent Runtime promotion evaluation", MediaType: "application/json", ContentSHA256: controls.proposal.EvidenceSHA256, SizeBytes: 2, Metadata: []byte(`{}`)},
		Content:  []byte(`{}`),
	}}
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{})
	server, _ = server.WithPromotionControls(controls)
	server, _ = server.WithPromotionEvidence(evidence)
	request := &agentv1.GetRuntimePromotionRequest{Context: grpccommon.RequestContext("U-OPS", "dipole-gateway"), TenantId: "dipole", ProposalId: controls.proposal.ProposalUUID}
	response, err := invokeAuthenticatedAgentRPC(t, "dipole-gateway", func(ctx context.Context) (any, error) { return server.GetRuntimePromotion(ctx, request) })
	if err != nil || response.(*agentv1.RuntimePromotionProposalResponse).GetProposalId() != controls.proposal.ProposalUUID || controls.operator != "U-OPS" {
		t.Fatalf("response=%+v operator=%s err=%v", response, controls.operator, err)
	}
	evidenceResponse, err := invokeAuthenticatedAgentRPC(t, "dipole-gateway", func(ctx context.Context) (any, error) {
		return server.GetRuntimePromotionEvidence(ctx, &agentv1.GetRuntimePromotionEvidenceRequest{Context: request.Context, TenantId: "dipole", ProposalId: controls.proposal.ProposalUUID})
	})
	resolved := evidenceResponse.(*agentv1.RuntimePromotionEvidenceResponse)
	if err != nil || resolved.GetProposal().GetProposalId() != controls.proposal.ProposalUUID || resolved.GetArtifact().GetArtifactId() != controls.proposal.EvidenceArtifactUUID || string(resolved.GetContent()) != `{}` || evidence.operator != "U-OPS" {
		t.Fatalf("evidence response=%+v operator=%s err=%v", resolved, evidence.operator, err)
	}
	request.Context.CallerService = "dipole-agent"
	if _, err := invokeAuthenticatedAgentRPC(t, "dipole-agent", func(ctx context.Context) (any, error) { return server.GetRuntimePromotion(ctx, request) }); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("Agent Runtime control code = %s", status.Code(err))
	}
}

func TestPublishMcpReadinessEvidenceRPCUsesAuthenticatedRuntimeIdentity(t *testing.T) {
	publisher := &mcpReadinessEvidencePublisherStub{created: true}
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{})
	server, _ = server.WithMCPReadinessEvidencePublisher(publisher)
	startedAt := time.Date(2026, 8, 28, 14, 0, 0, 0, time.UTC)
	profileBinding := strings.Repeat("b", 64)
	evidence := application.AgentMCPReadinessEvidenceV1{
		SchemaVersion: application.AgentMCPReadinessEvidenceSchemaVersionV2,
		BindingSHA256: strings.Repeat("a", 64), ProfileBindingSHA256: profileBinding,
		StartedAt: startedAt, PreflightCheckedAt: startedAt.Add(time.Second),
		ConnectivityCheckedAt: startedAt.Add(2 * time.Second), CompletedAt: startedAt.Add(3 * time.Second),
		ProfileCount: 1, CredentialCount: 1, CABundleCount: 1, ToolCount: 2,
	}
	body, _ := json.Marshal(evidence)
	request := &agentv1.PublishMcpReadinessEvidenceRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), TenantId: "dipole",
		ProfileBindingSha256: profileBinding, EvidenceJson: body, ExpiresAtUnixMs: startedAt.Add(30 * time.Minute).UnixMilli(),
	}
	request.Context.RequestId, request.Context.TraceId = "REQ-1", "TRACE-1"
	response, err := invokeAuthenticatedAgentRPC(t, "dipole-agent", func(ctx context.Context) (any, error) {
		return server.PublishMcpReadinessEvidence(ctx, request)
	})
	if err != nil {
		t.Fatal(err)
	}
	result := response.(*agentv1.PublishMcpReadinessEvidenceResponse)
	if publisher.operator != "dipole-agent" || publisher.request.RequestID != "REQ-1" || publisher.request.TraceID != "TRACE-1" ||
		result.GetEvidenceId() != publisher.record.EvidenceUUID || result.GetContentSha256() != publisher.record.ContentSHA256 || !result.GetCreated() {
		t.Fatalf("response=%+v operator=%s request=%+v", result, publisher.operator, publisher.request)
	}
	publisher.created = false
	replay, err := invokeAuthenticatedAgentRPC(t, "dipole-agent", func(ctx context.Context) (any, error) {
		return server.PublishMcpReadinessEvidence(ctx, request)
	})
	if err != nil || replay.(*agentv1.PublishMcpReadinessEvidenceResponse).GetEvidenceId() != result.GetEvidenceId() || replay.(*agentv1.PublishMcpReadinessEvidenceResponse).GetCreated() {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}

	request.Context.PrincipalUserId = "U-OPS"
	if _, err := invokeAuthenticatedAgentRPC(t, "dipole-agent", func(ctx context.Context) (any, error) {
		return server.PublishMcpReadinessEvidence(ctx, request)
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("principal injection code=%s", status.Code(err))
	}
	request.Context = grpccommon.RequestContext("", "dipole-gateway")
	if _, err := invokeAuthenticatedAgentRPC(t, "dipole-gateway", func(ctx context.Context) (any, error) {
		return server.PublishMcpReadinessEvidence(ctx, request)
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("Gateway publish code=%s", status.Code(err))
	}
}

func TestPublishMcpReadinessEvidenceRPCRejectsInvalidEvidenceBeforeStore(t *testing.T) {
	publisher := &mcpReadinessEvidencePublisherStub{}
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{})
	server, _ = server.WithMCPReadinessEvidencePublisher(publisher)
	request := &agentv1.PublishMcpReadinessEvidenceRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), TenantId: "dipole",
		ProfileBindingSha256: strings.Repeat("b", 64), EvidenceJson: []byte(`{"schemaVersion":"dipole.agent.external-mcp-readiness-evidence.v2","token":"secret"}`),
		ExpiresAtUnixMs: time.Now().Add(time.Hour).UnixMilli(),
	}
	if _, err := invokeAuthenticatedAgentRPC(t, "dipole-agent", func(ctx context.Context) (any, error) {
		return server.PublishMcpReadinessEvidence(ctx, request)
	}); status.Code(err) != codes.InvalidArgument || publisher.calls != 0 {
		t.Fatalf("invalid evidence code=%s calls=%d", status.Code(err), publisher.calls)
	}

	server.readinessPublisher = nil
	if _, err := invokeAuthenticatedAgentRPC(t, "dipole-agent", func(ctx context.Context) (any, error) {
		return server.PublishMcpReadinessEvidence(ctx, request)
	}); status.Code(err) != codes.Unavailable {
		t.Fatalf("missing Publisher code=%s", status.Code(err))
	}
}

func TestResolveFreshMcpReadinessEvidenceRPCUsesAuthenticatedRuntimeIdentity(t *testing.T) {
	record, _ := application.NewAgentMCPReadinessEvidenceRecordV1("OPERATOR", readinessEvidencePublishRequestForRPC())
	resolver := &mcpReadinessEvidenceResolverStub{record: &record}
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{})
	server, _ = server.WithMCPReadinessEvidenceResolver(resolver)
	request := &agentv1.ResolveFreshMcpReadinessEvidenceRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), TenantId: "dipole",
		ProfileBindingSha256: strings.Repeat("b", 64), RuntimeBindingSha256: strings.Repeat("a", 64),
	}
	response, err := invokeAuthenticatedAgentRPC(t, "dipole-agent", func(ctx context.Context) (any, error) {
		return server.ResolveFreshMcpReadinessEvidence(ctx, request)
	})
	if err != nil || !response.(*agentv1.ResolveFreshMcpReadinessEvidenceResponse).GetFound() || resolver.tenant != "dipole" {
		t.Fatalf("response=%+v resolver=%+v err=%v", response, resolver, err)
	}
	resolver.record = nil
	empty, err := invokeAuthenticatedAgentRPC(t, "dipole-agent", func(ctx context.Context) (any, error) {
		return server.ResolveFreshMcpReadinessEvidence(ctx, request)
	})
	if err != nil || empty.(*agentv1.ResolveFreshMcpReadinessEvidenceResponse).GetFound() {
		t.Fatalf("empty=%+v err=%v", empty, err)
	}
	request.Context.PrincipalUserId = "U-OPS"
	if _, err := invokeAuthenticatedAgentRPC(t, "dipole-agent", func(ctx context.Context) (any, error) {
		return server.ResolveFreshMcpReadinessEvidence(ctx, request)
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("principal injection code=%s", status.Code(err))
	}
	request.Context = grpccommon.RequestContext("", "dipole-gateway")
	if _, err := invokeAuthenticatedAgentRPC(t, "dipole-gateway", func(ctx context.Context) (any, error) {
		return server.ResolveFreshMcpReadinessEvidence(ctx, request)
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("Gateway resolve code=%s", status.Code(err))
	}
	server.readinessResolver = nil
	request.Context = grpccommon.RequestContext("", "dipole-agent")
	if _, err := invokeAuthenticatedAgentRPC(t, "dipole-agent", func(ctx context.Context) (any, error) {
		return server.ResolveFreshMcpReadinessEvidence(ctx, request)
	}); status.Code(err) != codes.Unavailable {
		t.Fatalf("missing Resolver code=%s", status.Code(err))
	}
}

type mcpReadinessEvidenceResolverStub struct {
	record                                 *application.AgentMCPReadinessEvidenceRecordV1
	tenant, profileBinding, runtimeBinding string
	err                                    error
}

func (resolver *mcpReadinessEvidenceResolverStub) ResolveFreshAgentMCPReadinessEvidence(_ context.Context, tenant, profileBinding, runtimeBinding string) (*application.AgentMCPReadinessEvidenceRecordV1, error) {
	resolver.tenant, resolver.profileBinding, resolver.runtimeBinding = tenant, profileBinding, runtimeBinding
	return resolver.record, resolver.err
}

func readinessEvidencePublishRequestForRPC() application.AgentMCPReadinessEvidenceRequestV1 {
	startedAt := time.Date(2026, 8, 28, 14, 0, 0, 0, time.UTC)
	return application.AgentMCPReadinessEvidenceRequestV1{
		TenantID: "dipole", ProfileBindingSHA256: strings.Repeat("b", 64), ExpiresAt: startedAt.Add(30 * time.Minute),
		Evidence: application.AgentMCPReadinessEvidenceV1{
			SchemaVersion: application.AgentMCPReadinessEvidenceSchemaVersionV2,
			BindingSHA256: strings.Repeat("a", 64), ProfileBindingSHA256: strings.Repeat("b", 64),
			StartedAt: startedAt, PreflightCheckedAt: startedAt.Add(time.Second), ConnectivityCheckedAt: startedAt.Add(2 * time.Second), CompletedAt: startedAt.Add(3 * time.Second),
			ProfileCount: 1, CredentialCount: 1, CABundleCount: 1, ToolCount: 1,
		},
	}
}

func TestPublishMcpReadinessEvidenceRPCRejectsEmptyPublisherResult(t *testing.T) {
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{})
	server.readinessPublisher = emptyMCPReadinessEvidencePublisherStub{}
	startedAt := time.Date(2026, 8, 28, 14, 0, 0, 0, time.UTC)
	evidence := application.AgentMCPReadinessEvidenceV1{
		SchemaVersion: application.AgentMCPReadinessEvidenceSchemaVersionV2,
		BindingSHA256: strings.Repeat("a", 64), ProfileBindingSHA256: strings.Repeat("b", 64),
		StartedAt: startedAt, PreflightCheckedAt: startedAt.Add(time.Second),
		ConnectivityCheckedAt: startedAt.Add(2 * time.Second), CompletedAt: startedAt.Add(3 * time.Second),
		ProfileCount: 1, CredentialCount: 1, CABundleCount: 1, ToolCount: 2,
	}
	body, _ := json.Marshal(evidence)
	request := &agentv1.PublishMcpReadinessEvidenceRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), TenantId: "dipole",
		ProfileBindingSha256: evidence.ProfileBindingSHA256, EvidenceJson: body,
		ExpiresAtUnixMs: startedAt.Add(30 * time.Minute).UnixMilli(),
	}
	if _, err := invokeAuthenticatedAgentRPC(t, "dipole-agent", func(ctx context.Context) (any, error) {
		return server.PublishMcpReadinessEvidence(ctx, request)
	}); status.Code(err) != codes.Internal {
		t.Fatalf("empty Publisher result code=%s", status.Code(err))
	}
}

type mcpReadinessEvidencePublisherStub struct {
	operator string
	request  application.AgentMCPReadinessEvidenceRequestV1
	record   application.AgentMCPReadinessEvidenceRecordV1
	calls    int
	created  bool
}

type emptyMCPReadinessEvidencePublisherStub struct{}

func (emptyMCPReadinessEvidencePublisherStub) PublishAgentMCPReadinessEvidence(context.Context, string, application.AgentMCPReadinessEvidenceRequestV1) (*application.AgentMCPReadinessEvidenceRecordV1, bool, error) {
	return nil, false, nil
}

func (publisher *mcpReadinessEvidencePublisherStub) PublishAgentMCPReadinessEvidence(_ context.Context, operator string, request application.AgentMCPReadinessEvidenceRequestV1) (*application.AgentMCPReadinessEvidenceRecordV1, bool, error) {
	publisher.operator, publisher.request = operator, request
	publisher.calls++
	record, err := application.NewAgentMCPReadinessEvidenceRecordV1(operator, request)
	publisher.record = record
	return &publisher.record, publisher.created, err
}

type runtimePromotionEvidenceServiceStub struct {
	review   *application.AgentRuntimePromotionEvidenceReviewV1
	operator string
}

func (s *runtimePromotionEvidenceServiceStub) Get(_ context.Context, operator, _, _ string) (*application.AgentRuntimePromotionEvidenceReviewV1, error) {
	s.operator = operator
	return s.review, nil
}

type runtimePromotionControlServiceStub struct {
	proposal *application.AgentRuntimePromotionProposalV1
	operator string
}

func (s *runtimePromotionControlServiceStub) Propose(_ context.Context, operator string, _ application.AgentRuntimePromotionProposalRequestV1) (*application.AgentRuntimePromotionProposalV1, error) {
	s.operator = operator
	return s.proposal, nil
}
func (s *runtimePromotionControlServiceStub) Review(_ context.Context, operator, _ string, _ application.AgentRuntimePromotionReviewDecisionV1) (*application.AgentRuntimePromotionProposalV1, error) {
	s.operator = operator
	return s.proposal, nil
}
func (s *runtimePromotionControlServiceStub) Get(_ context.Context, operator, _, _ string) (*application.AgentRuntimePromotionProposalV1, error) {
	s.operator = operator
	return s.proposal, nil
}
func (s *runtimePromotionControlServiceStub) Revoke(context.Context, string, string, string, string) (*application.AgentRuntimePromotionGrantV1, error) {
	return nil, nil
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
