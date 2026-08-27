package agentgrpc

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	grpcauth "github.com/JekYUlll/Dipole/internal/transport/grpc/auth"
	grpccommon "github.com/JekYUlll/Dipole/internal/transport/grpc/common"
	agentv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/agent/v1"
	commonv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/common/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	agentv1.UnimplementedAgentCapabilityServiceServer
	capability    application.AgentCapabilityV1
	resolver      application.AgentInvocationResolverV1
	admission     application.AgentRunAdmissionServiceV1
	approvals     application.AgentApprovalServiceV1
	controls      application.AgentTaskControlAuthorizerV1
	projections   application.AgentTaskWorkflowProjectionServiceV1
	repairs       application.AgentWorkflowRepairAuditServiceV1
	artifacts     application.AgentArtifactServiceV1
	subscriptions application.AgentEventSubscriptionResolverV1
	memories      application.AgentMemoryContextResolverV1
}

func (s *Server) WithMemories(memories application.AgentMemoryContextResolverV1) (*Server, error) {
	if s == nil || memories == nil {
		return nil, errors.New("Agent Memory resolver is required")
	}
	s.memories = memories
	return s, nil
}

func (s *Server) ListContextMemories(ctx context.Context, request *agentv1.ListContextMemoriesRequest) (*agentv1.ListContextMemoriesResponse, error) {
	caller, err := authenticatedAgentArtifactCallerV1(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	if caller != "dipole-agent" || strings.TrimSpace(request.GetContext().GetPrincipalUserId()) != "" {
		return nil, status.Error(codes.PermissionDenied, "only the authenticated Agent runtime may list Context Memories")
	}
	if s.memories == nil {
		return nil, status.Error(codes.Unavailable, "Agent Memory resolver is unavailable")
	}
	items, err := s.memories.ResolveContextMemories(grpccommon.Correlation(ctx, request.GetContext()), request.GetTaskId(), request.GetRunId(), request.GetResourceType(), request.GetResourceId(), int(request.GetLimit()))
	if err != nil {
		switch {
		case errors.Is(err, application.ErrAgentMemoryDenied):
			return nil, status.Error(codes.PermissionDenied, "Agent Memory scope denied")
		case errors.Is(err, application.ErrAgentMemoryInvalid), errors.Is(err, application.ErrAgentExecutionPolicyDenied):
			return nil, status.Error(codes.FailedPrecondition, "Agent Memory request is invalid")
		default:
			return nil, status.Error(codes.Internal, "Agent Memory lookup failed")
		}
	}
	response := &agentv1.ListContextMemoriesResponse{Memories: make([]*agentv1.AgentContextMemory, 0, len(items))}
	for _, item := range items {
		response.Memories = append(response.Memories, &agentv1.AgentContextMemory{
			MemoryId: item.MemoryUUID, MemoryType: string(item.MemoryType), Content: item.Content,
			CompactContent: item.CompactContent, Priority: item.Priority,
			Provenance: &agentv1.AgentMemoryProvenance{SourceType: item.Provenance.SourceType, SourceId: item.Provenance.SourceID, Uri: item.Provenance.URI, Sequence: item.Provenance.Sequence},
		})
	}
	return response, nil
}

func (s *Server) WithEventSubscriptions(resolver application.AgentEventSubscriptionResolverV1) (*Server, error) {
	if s == nil || resolver == nil {
		return nil, errors.New("Agent Event Subscription resolver is required")
	}
	s.subscriptions = resolver
	return s, nil
}

func (s *Server) MatchEventSubscriptions(ctx context.Context, request *agentv1.MatchEventSubscriptionsRequest) (*agentv1.MatchEventSubscriptionsResponse, error) {
	caller, err := authenticatedAgentArtifactCallerV1(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	if caller != "dipole-agent" || strings.TrimSpace(request.GetContext().GetPrincipalUserId()) != "" {
		return nil, status.Error(codes.PermissionDenied, "only the authenticated Agent runtime may match Event Subscriptions")
	}
	if s.subscriptions == nil {
		return nil, status.Error(codes.Unavailable, "Agent Event Subscription resolver is unavailable")
	}
	items, err := s.subscriptions.MatchEventSubscriptions(grpccommon.Correlation(ctx, request.GetContext()), application.AgentEventSubscriptionMatchRequestV1{
		TenantID: request.GetTenantId(), AgentUUID: request.GetAgentId(), EventType: request.GetEventType(),
		ResourceType: request.GetResourceType(), ResourceID: request.GetResourceId(),
	})
	if err != nil {
		if errors.Is(err, application.ErrAgentSubscriptionInvalid) {
			return nil, status.Error(codes.FailedPrecondition, "Agent Event Subscription policy is invalid")
		}
		return nil, status.Error(codes.Internal, "Agent Event Subscription lookup failed")
	}
	response := &agentv1.MatchEventSubscriptionsResponse{Subscriptions: make([]*agentv1.AgentEventSubscription, 0, len(items))}
	for _, item := range items {
		response.Subscriptions = append(response.Subscriptions, &agentv1.AgentEventSubscription{
			SubscriptionId: item.SubscriptionUUID, DefinitionId: item.DefinitionUUID, DefinitionVersion: item.DefinitionVersion,
			TenantId: item.TenantID, AgentId: item.AgentUUID, EventType: item.EventType,
			ResourceType: item.ResourceType, ResourceId: item.ResourceID, FilterKind: string(item.FilterKind), FilterJson: item.FilterJSON,
		})
	}
	return response, nil
}

func (s *Server) WithArtifacts(artifacts application.AgentArtifactServiceV1) (*Server, error) {
	if s == nil || artifacts == nil {
		return nil, errors.New("Agent Artifact service is required")
	}
	s.artifacts = artifacts
	return s, nil
}

func (s *Server) CreateArtifact(ctx context.Context, request *agentv1.CreateArtifactRequest) (*agentv1.CreateArtifactResponse, error) {
	caller, err := authenticatedAgentArtifactCallerV1(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	if caller != "dipole-agent" || strings.TrimSpace(request.GetContext().GetPrincipalUserId()) != "" {
		return nil, status.Error(codes.PermissionDenied, "only the authenticated Agent runtime may create Artifacts")
	}
	if s.artifacts == nil {
		return nil, status.Error(codes.Unavailable, "Agent Artifact storage is unavailable")
	}
	artifact, err := s.artifacts.Create(grpccommon.Correlation(ctx, request.GetContext()), application.AgentArtifactCreateV1{
		TenantID: request.GetTenantId(), TaskUUID: request.GetTaskId(), RunUUID: request.GetRunId(),
		ArtifactType: request.GetArtifactType(), Version: request.GetVersion(), Title: request.GetTitle(),
		MediaType: request.GetMediaType(), Content: request.GetContent(), Metadata: request.GetMetadataJson(),
	})
	if err != nil {
		return nil, mapAgentArtifactErrorV1(err)
	}
	return &agentv1.CreateArtifactResponse{Artifact: agentArtifactResponseV1(artifact)}, nil
}

func (s *Server) GetArtifact(ctx context.Context, request *agentv1.GetArtifactRequest) (*agentv1.GetArtifactResponse, error) {
	caller, err := authenticatedAgentArtifactCallerV1(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	if caller != "dipole-gateway" {
		return nil, status.Error(codes.PermissionDenied, "only the authenticated Gateway may retrieve Artifacts")
	}
	principal, err := grpccommon.Principal(request.GetContext())
	if err != nil {
		return nil, err
	}
	if s.artifacts == nil {
		return nil, status.Error(codes.Unavailable, "Agent Artifact storage is unavailable")
	}
	artifact, body, err := s.artifacts.GetForPrincipal(grpccommon.Correlation(ctx, request.GetContext()), principal, request.GetArtifactId())
	if err != nil {
		return nil, mapAgentArtifactErrorV1(err)
	}
	return &agentv1.GetArtifactResponse{Artifact: agentArtifactResponseV1(artifact), Content: body}, nil
}

func authenticatedAgentArtifactCallerV1(ctx context.Context, requestContext *commonv1.RequestContext) (string, error) {
	authenticated, ok := grpcauth.CallerService(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "authenticated service identity is required")
	}
	claimed, err := grpccommon.Caller(ctx, requestContext)
	if err != nil {
		return "", err
	}
	if authenticated != claimed {
		return "", status.Error(codes.PermissionDenied, "caller service does not match authenticated service")
	}
	return authenticated, nil
}

func agentArtifactResponseV1(value *application.AgentArtifactV1) *agentv1.AgentArtifact {
	if value == nil {
		return nil
	}
	return &agentv1.AgentArtifact{
		SchemaVersion: value.SchemaVersion, ArtifactId: value.ArtifactUUID, TaskId: value.TaskUUID, RunId: value.RunUUID,
		ArtifactType: value.ArtifactType, Version: value.Version, Title: value.Title, MediaType: value.MediaType,
		ContentSha256: value.ContentSHA256, SizeBytes: value.SizeBytes, MetadataJson: value.Metadata,
		CreatedAtUnixMs: value.CreatedAt.UnixMilli(),
	}
}

func mapAgentArtifactErrorV1(err error) error {
	if errors.Is(err, application.ErrAgentArtifactDenied) {
		return status.Error(codes.NotFound, "Agent Artifact unavailable")
	}
	if errors.Is(err, application.ErrAgentArtifactConflict) {
		return status.Error(codes.FailedPrecondition, "Agent Artifact evidence conflicts")
	}
	if errors.Is(err, application.ErrAgentArtifactInvalid) {
		return status.Error(codes.InvalidArgument, "Agent Artifact request is invalid")
	}
	return status.Error(codes.Internal, "Agent Artifact operation failed")
}

func NewServer(capability application.AgentCapabilityV1, resolver application.AgentInvocationResolverV1, admission application.AgentRunAdmissionServiceV1, approvals ...application.AgentApprovalServiceV1) (*Server, error) {
	if capability == nil || resolver == nil || admission == nil {
		return nil, errors.New("Agent Capability, Invocation resolver, and Run admission are required")
	}
	var approvalService application.AgentApprovalServiceV1
	if len(approvals) > 0 {
		approvalService = approvals[0]
	}
	return &Server{capability: capability, resolver: resolver, admission: admission, approvals: approvalService}, nil
}

func NewServerWithControl(capability application.AgentCapabilityV1, resolver application.AgentInvocationResolverV1, admission application.AgentRunAdmissionServiceV1, approvals application.AgentApprovalServiceV1, controls application.AgentTaskControlAuthorizerV1) (*Server, error) {
	server, err := NewServer(capability, resolver, admission, approvals)
	if err != nil {
		return nil, err
	}
	if controls == nil {
		return nil, errors.New("Agent Task control authorizer is required")
	}
	server.controls = controls
	return server, nil
}

func NewServerWithControlAndProjection(capability application.AgentCapabilityV1, resolver application.AgentInvocationResolverV1, admission application.AgentRunAdmissionServiceV1, approvals application.AgentApprovalServiceV1, controls application.AgentTaskControlAuthorizerV1, projections application.AgentTaskWorkflowProjectionServiceV1, repairs ...application.AgentWorkflowRepairAuditServiceV1) (*Server, error) {
	server, err := NewServerWithControl(capability, resolver, admission, approvals, controls)
	if err != nil {
		return nil, err
	}
	if projections == nil {
		return nil, errors.New("Agent Task Workflow projection service is required")
	}
	server.projections = projections
	if len(repairs) > 0 {
		if repairs[0] == nil {
			return nil, errors.New("Agent Workflow repair audit service is required")
		}
		server.repairs = repairs[0]
	}
	return server, nil
}

func (s *Server) ProposeWorkflowRepair(ctx context.Context, request *agentv1.ProposeWorkflowRepairRequest) (*agentv1.WorkflowRepairProposalResponse, error) {
	principal, err := workflowRepairOperatorV1(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	if s.repairs == nil {
		return nil, status.Error(codes.Unavailable, "Agent Workflow repair audit is unavailable")
	}
	proposal, err := s.repairs.Propose(ctx, principal, application.AgentWorkflowRepairProposalRequestV1{
		TaskUUID: request.GetTaskId(), Outcome: application.AgentWorkflowRepairOutcomeV1(request.GetOutcome()), TicketRef: request.GetTicketRef(), Reason: request.GetReason(),
		Projected: workflowRepairEvidenceFromRPCV1(request.GetProjected()), Temporal: workflowRepairEvidenceValueFromRPCV1(request.GetTemporal()),
		ProposedAt: time.UnixMilli(request.GetProposedAtUnixMs()), ExpiresAt: time.UnixMilli(request.GetExpiresAtUnixMs()),
	})
	if err != nil {
		return nil, workflowRepairErrorV1(err)
	}
	return workflowRepairProposalResponseV1(proposal), nil
}

func (s *Server) DecideWorkflowRepair(ctx context.Context, request *agentv1.DecideWorkflowRepairRequest) (*agentv1.WorkflowRepairProposalResponse, error) {
	principal, err := workflowRepairOperatorV1(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	if s.repairs == nil {
		return nil, status.Error(codes.Unavailable, "Agent Workflow repair audit is unavailable")
	}
	proposal, err := s.repairs.Decide(ctx, principal, request.GetProposalId(), application.AgentWorkflowRepairDecisionV1(request.GetDecision()))
	if err != nil {
		return nil, workflowRepairErrorV1(err)
	}
	return workflowRepairProposalResponseV1(proposal), nil
}

func (s *Server) GetWorkflowRepair(ctx context.Context, request *agentv1.GetWorkflowRepairRequest) (*agentv1.WorkflowRepairProposalResponse, error) {
	principal, err := workflowRepairOperatorV1(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	if s.repairs == nil {
		return nil, status.Error(codes.Unavailable, "Agent Workflow repair audit is unavailable")
	}
	proposal, err := s.repairs.Get(ctx, principal, request.GetProposalId())
	if err != nil {
		return nil, workflowRepairErrorV1(err)
	}
	return workflowRepairProposalResponseV1(proposal), nil
}

func workflowRepairOperatorV1(ctx context.Context, requestContext *commonv1.RequestContext) (string, error) {
	authenticated, ok := grpcauth.CallerService(ctx)
	if !ok || authenticated != "dipole-gateway" || strings.TrimSpace(requestContext.GetCallerService()) != authenticated {
		return "", status.Error(codes.PermissionDenied, "only the authenticated Gateway may submit Workflow repair audit requests")
	}
	// Caller still verifies the mTLS/shared-secret identity against the claimed service.
	if _, err := grpccommon.Caller(ctx, requestContext); err != nil {
		return "", err
	}
	return grpccommon.Principal(requestContext)
}

func workflowRepairEvidenceFromRPCV1(value *agentv1.WorkflowRepairEvidence) *application.AgentWorkflowEvidenceV1 {
	if value == nil {
		return nil
	}
	result := workflowRepairEvidenceValueFromRPCV1(value)
	return &result
}
func workflowRepairEvidenceValueFromRPCV1(value *agentv1.WorkflowRepairEvidence) application.AgentWorkflowEvidenceV1 {
	if value == nil {
		return application.AgentWorkflowEvidenceV1{}
	}
	return application.AgentWorkflowEvidenceV1{WorkflowID: value.GetWorkflowId(), WorkflowRunID: value.GetWorkflowRunId(), Status: value.GetStatus(), Revision: value.GetRevision()}
}
func workflowRepairEvidenceToRPCV1(value *application.AgentWorkflowEvidenceV1) *agentv1.WorkflowRepairEvidence {
	if value == nil {
		return nil
	}
	return &agentv1.WorkflowRepairEvidence{WorkflowId: value.WorkflowID, WorkflowRunId: value.WorkflowRunID, Status: value.Status, Revision: value.Revision}
}
func workflowRepairProposalResponseV1(value *application.AgentWorkflowRepairProposalV1) *agentv1.WorkflowRepairProposalResponse {
	if value == nil {
		return nil
	}
	temporal := value.Temporal
	return &agentv1.WorkflowRepairProposalResponse{ProposalId: value.ProposalUUID, TaskId: value.TaskUUID, Outcome: string(value.Outcome), Action: value.Action,
		ProposerId: value.ProposerUUID, TicketRef: value.TicketRef, Reason: value.Reason, Projected: workflowRepairEvidenceToRPCV1(value.Projected), Temporal: workflowRepairEvidenceToRPCV1(&temporal),
		EvidenceSha256: value.EvidenceSHA256, Status: string(value.Status), RequiredApprovals: uint32(value.RequiredApprovals), ProposedAtUnixMs: value.ProposedAt.UnixMilli(), ExpiresAtUnixMs: value.ExpiresAt.UnixMilli()}
}
func workflowRepairErrorV1(err error) error {
	if errors.Is(err, application.ErrAgentWorkflowRepairDenied) {
		return status.Error(codes.PermissionDenied, "Agent Workflow repair access denied")
	}
	if errors.Is(err, application.ErrAgentWorkflowRepairConflict) {
		return status.Error(codes.FailedPrecondition, "Agent Workflow repair evidence conflicts")
	}
	return status.Error(codes.Internal, "Agent Workflow repair audit failed")
}

func (s *Server) AuthorizeTaskControl(ctx context.Context, request *agentv1.AuthorizeTaskControlRequest) (*agentv1.AuthorizeTaskControlResponse, error) {
	if _, err := grpccommon.Caller(ctx, request.GetContext()); err != nil {
		return nil, err
	}
	if s.controls == nil || strings.TrimSpace(request.GetContext().GetPrincipalUserId()) != "" {
		return nil, status.Error(codes.InvalidArgument, "Agent Task control authorization is invalid")
	}
	authorization, err := s.controls.AuthorizeTaskControl(ctx, request.GetTaskId(), request.GetPrincipalUserId())
	if err != nil {
		if errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
			return nil, status.Error(codes.NotFound, "Agent Task unavailable")
		}
		return nil, status.Error(codes.Internal, "Agent Task control authorization failed")
	}
	response := &agentv1.AuthorizeTaskControlResponse{TaskId: authorization.TaskUUID, TaskStatus: string(authorization.Status)}
	if authorization.Workflow != nil {
		response.WorkflowId = authorization.Workflow.WorkflowID
		response.WorkflowRunId = authorization.Workflow.RunID
		response.WorkflowStatus = string(authorization.Workflow.Status)
		response.WorkflowRevision = authorization.Workflow.Revision
	}
	return response, nil
}

func (s *Server) ResolveMcpContext(ctx context.Context, request *agentv1.ResolveMcpContextRequest) (*agentv1.ResolveMcpContextResponse, error) {
	caller, err := grpccommon.Caller(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	if caller != "dipole-agent" || strings.TrimSpace(request.GetContext().GetPrincipalUserId()) != "" {
		return nil, status.Error(codes.PermissionDenied, "only the authenticated Agent runtime may resolve MCP context")
	}
	invocation, err := s.resolver.Resolve(grpccommon.Correlation(ctx, request.GetContext()), request.GetTaskId(), request.GetRunId())
	if err != nil {
		if errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
			return nil, status.Error(codes.NotFound, "Agent MCP context unavailable")
		}
		return nil, status.Error(codes.Internal, "Agent MCP context lookup failed")
	}
	if strings.TrimSpace(request.GetPrincipalUserId()) == "" || request.GetPrincipalUserId() != invocation.PrincipalUUID {
		return nil, status.Error(codes.NotFound, "Agent MCP context unavailable")
	}
	response := &agentv1.ResolveMcpContextResponse{
		TenantId: invocation.TenantID, PrincipalUserId: invocation.PrincipalUUID, AgentId: invocation.AgentUUID,
		DelegatedByUserId: invocation.DelegatedByUUID, Permissions: append([]string(nil), invocation.Permissions...),
		ApprovedCapabilities: append([]string(nil), invocation.ApprovedCapabilities...),
		ResourceScopes:       make([]*agentv1.AgentResourceScope, 0, len(invocation.ResourceScopes)),
	}
	for _, scope := range invocation.ResourceScopes {
		response.ResourceScopes = append(response.ResourceScopes, &agentv1.AgentResourceScope{
			ResourceType: scope.ResourceType, ResourceId: scope.ResourceID, Actions: append([]string(nil), scope.Actions...),
		})
	}
	return response, nil
}

func (s *Server) ProjectTaskWorkflowState(ctx context.Context, request *agentv1.ProjectTaskWorkflowStateRequest) (*agentv1.ProjectTaskWorkflowStateResponse, error) {
	if _, err := grpccommon.Caller(ctx, request.GetContext()); err != nil {
		return nil, err
	}
	if s.projections == nil || strings.TrimSpace(request.GetContext().GetPrincipalUserId()) != "" {
		return nil, status.Error(codes.InvalidArgument, "Agent Task Workflow projection is invalid")
	}
	projection, err := s.projections.Project(ctx, application.AgentTaskWorkflowProjectionRequestV1{
		Projection: application.AgentTaskWorkflowProjectionV1{
			TaskUUID: request.GetTaskId(), WorkflowID: request.GetWorkflowId(), RunID: request.GetWorkflowRunId(),
			Status: application.AgentTaskWorkflowStatusV1(request.GetWorkflowStatus()), Revision: request.GetWorkflowRevision(),
		},
		RunUUID: request.GetRunId(), RuntimeID: "dipole-agent", Mode: "shadow",
	})
	if err != nil {
		if errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
			return nil, status.Error(codes.PermissionDenied, "Agent Task Workflow projection denied")
		}
		if errors.Is(err, application.ErrAgentWorkflowProjectionConflict) {
			return nil, status.Error(codes.FailedPrecondition, "Agent Task Workflow projection conflicts")
		}
		return nil, status.Error(codes.Internal, "Agent Task Workflow projection failed")
	}
	return &agentv1.ProjectTaskWorkflowStateResponse{
		TaskId: projection.TaskUUID, WorkflowId: projection.WorkflowID, WorkflowRunId: projection.RunID,
		WorkflowStatus: string(projection.Status), WorkflowRevision: projection.Revision,
	}, nil
}

func (s *Server) ListTaskWorkflowProjectionSnapshots(ctx context.Context, request *agentv1.ListTaskWorkflowProjectionSnapshotsRequest) (*agentv1.ListTaskWorkflowProjectionSnapshotsResponse, error) {
	if _, err := grpccommon.Caller(ctx, request.GetContext()); err != nil {
		return nil, err
	}
	if s.projections == nil || strings.TrimSpace(request.GetContext().GetPrincipalUserId()) != "" {
		return nil, status.Error(codes.InvalidArgument, "Agent Task Workflow projection page is invalid")
	}
	page, err := s.projections.ListProjectionSnapshots(ctx, request.GetAfterTaskId(), int(request.GetPageSize()))
	if err != nil {
		if errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
			return nil, status.Error(codes.InvalidArgument, "Agent Task Workflow projection page is invalid")
		}
		return nil, status.Error(codes.Internal, "Agent Task Workflow projection page failed")
	}
	response := &agentv1.ListTaskWorkflowProjectionSnapshotsResponse{NextCursor: page.NextCursor}
	for _, task := range page.Tasks {
		snapshot := &agentv1.TaskWorkflowProjectionSnapshot{TaskId: task.TaskUUID}
		if task.Workflow != nil {
			snapshot.HasWorkflow = true
			snapshot.WorkflowId = task.Workflow.WorkflowID
			snapshot.WorkflowRunId = task.Workflow.RunID
			snapshot.WorkflowStatus = string(task.Workflow.Status)
			snapshot.WorkflowRevision = task.Workflow.Revision
		}
		response.Tasks = append(response.Tasks, snapshot)
	}
	return response, nil
}

func (s *Server) RequestApproval(ctx context.Context, request *agentv1.RequestApprovalRequest) (*agentv1.ApprovalResponse, error) {
	if _, err := grpccommon.Caller(ctx, request.GetContext()); err != nil {
		return nil, err
	}
	if s.approvals == nil || strings.TrimSpace(request.GetContext().GetPrincipalUserId()) != "" || request.GetResourceScope() == nil {
		return nil, status.Error(codes.InvalidArgument, "Agent Approval request is invalid")
	}
	approval, err := s.approvals.Request(ctx, application.AgentApprovalRequestV1{
		TaskUUID: request.GetTaskId(), RunUUID: request.GetRunId(), RuntimeID: "dipole-agent", Mode: "shadow",
		Approval: application.AgentApprovalV1{
			ApprovalUUID: request.GetApprovalId(), TaskUUID: request.GetTaskId(), CapabilityID: request.GetCapabilityId(),
			ResourceScope: application.AgentResourceScopeV1{ResourceType: request.GetResourceScope().GetResourceType(), ResourceID: request.GetResourceScope().GetResourceId(), Actions: request.GetResourceScope().GetActions()},
			ScopeSHA256:   request.GetScopeSha256(), ArgumentsSHA256: request.GetArgumentsSha256(), NonceSHA256: request.GetNonceSha256(),
			Status: application.AgentApprovalStatusPending, ExpiresAt: time.UnixMilli(request.GetExpiresAtUnixMs()).UTC(),
		},
	})
	if err != nil {
		return nil, mapApprovalError(err)
	}
	return approvalResponse(approval), nil
}

func (s *Server) ResolveApproval(ctx context.Context, request *agentv1.ResolveApprovalRequest) (*agentv1.ApprovalResponse, error) {
	if _, err := grpccommon.Caller(ctx, request.GetContext()); err != nil {
		return nil, err
	}
	if s.approvals == nil || strings.TrimSpace(request.GetContext().GetPrincipalUserId()) != "" {
		return nil, status.Error(codes.InvalidArgument, "Agent Approval resolution is invalid")
	}
	approval, err := s.approvals.Resolve(ctx, application.AgentApprovalResolutionV1{
		TaskUUID: request.GetTaskId(), RunUUID: request.GetRunId(), RuntimeID: "dipole-agent", Mode: "shadow",
		ApprovalUUID: request.GetApprovalId(), ActorUUID: request.GetActorUserId(), Decision: application.AgentApprovalDecisionV1(request.GetDecision()),
	})
	if err != nil {
		return nil, mapApprovalError(err)
	}
	return approvalResponse(approval), nil
}

func approvalResponse(approval *application.AgentApprovalV1) *agentv1.ApprovalResponse {
	return &agentv1.ApprovalResponse{ApprovalId: approval.ApprovalUUID, Status: string(approval.Status), ApprovedByUserId: approval.ApprovedByUUID}
}

func mapApprovalError(err error) error {
	if errors.Is(err, application.ErrAgentApprovalDenied) {
		return status.Error(codes.PermissionDenied, "Agent Approval denied")
	}
	return status.Error(codes.Internal, "Agent Approval transition failed")
}

func (s *Server) AdmitRun(ctx context.Context, request *agentv1.AdmitRunRequest) (*agentv1.AdmitRunResponse, error) {
	if _, err := grpccommon.Caller(ctx, request.GetContext()); err != nil {
		return nil, err
	}
	if strings.TrimSpace(request.GetContext().GetPrincipalUserId()) != "" {
		return nil, status.Error(codes.InvalidArgument, "admission principal belongs to the trusted event payload")
	}
	if strings.TrimSpace(request.GetRuntimeId()) != "dipole-agent" || strings.TrimSpace(request.GetMode()) != "shadow" {
		return nil, status.Error(codes.InvalidArgument, "Agent Run identity is fixed by the authenticated endpoint")
	}
	execution, err := s.admission.Admit(ctx, application.AgentRunAdmissionRequestV1{
		AgentExecutionPolicyStartV1: application.AgentExecutionPolicyStartV1{
			TenantID: request.GetTenantId(), PrincipalUUID: request.GetPrincipalUserId(), AgentUUID: request.GetAgentId(),
			DelegatedByUUID: request.GetPrincipalUserId(), TriggerType: request.GetTriggerType(), TriggerRef: request.GetTriggerRef(),
			SubscriptionUUID: request.GetSubscriptionId(),
			RequestID:        request.GetContext().GetRequestId(), TraceID: request.GetContext().GetTraceId(), EventID: request.GetEventId(),
		}, RuntimeID: "dipole-agent", Mode: "shadow",
	})
	if err != nil {
		if errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
			return nil, status.Error(codes.PermissionDenied, "Agent Run admission denied")
		}
		return nil, status.Error(codes.Internal, "Agent Run admission failed")
	}
	return &agentv1.AdmitRunResponse{TaskId: execution.TaskUUID, RunId: execution.RunUUID, RunStatus: string(execution.RunStatus)}, nil
}

func (s *Server) CompleteRun(ctx context.Context, request *agentv1.CompleteRunRequest) (*agentv1.CompleteRunResponse, error) {
	if _, err := grpccommon.Caller(ctx, request.GetContext()); err != nil {
		return nil, err
	}
	if strings.TrimSpace(request.GetContext().GetPrincipalUserId()) != "" {
		return nil, status.Error(codes.InvalidArgument, "Agent principal must be resolved from Task")
	}
	if err := s.admission.Finish(ctx, request.GetTaskId(), request.GetRunId(), "dipole-agent", "shadow", application.AgentRunStatusCompleted, ""); err != nil {
		if errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
			return nil, status.Error(codes.PermissionDenied, "Agent Run completion denied")
		}
		return nil, status.Error(codes.Internal, "Agent Run completion failed")
	}
	return &agentv1.CompleteRunResponse{RunStatus: string(application.AgentRunStatusCompleted)}, nil
}

func (s *Server) FinishRun(ctx context.Context, request *agentv1.FinishRunRequest) (*agentv1.FinishRunResponse, error) {
	if _, err := grpccommon.Caller(ctx, request.GetContext()); err != nil {
		return nil, err
	}
	if strings.TrimSpace(request.GetContext().GetPrincipalUserId()) != "" {
		return nil, status.Error(codes.InvalidArgument, "Agent principal must be resolved from Task")
	}
	runStatus := application.AgentRunStatusV1(strings.TrimSpace(request.GetRunStatus()))
	lastError := strings.TrimSpace(request.GetLastError())
	if err := application.ValidateAgentRunTerminalV1(runStatus, lastError); err != nil {
		return nil, status.Error(codes.InvalidArgument, "Agent Run terminal evidence is invalid")
	}
	if err := s.admission.Finish(ctx, request.GetTaskId(), request.GetRunId(), "dipole-agent", "shadow", runStatus, lastError); err != nil {
		if errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
			return nil, status.Error(codes.PermissionDenied, "Agent Run terminal transition denied")
		}
		return nil, status.Error(codes.Internal, "Agent Run terminal transition failed")
	}
	return &agentv1.FinishRunResponse{RunStatus: string(runStatus)}, nil
}

func (s *Server) ListConversations(ctx context.Context, request *agentv1.ListConversationsRequest) (*agentv1.ListConversationsResponse, error) {
	if _, err := grpccommon.Caller(ctx, request.GetContext()); err != nil {
		return nil, err
	}
	if strings.TrimSpace(request.GetContext().GetPrincipalUserId()) != "" {
		return nil, status.Error(codes.InvalidArgument, "Agent principal must be resolved from Task")
	}
	limit := int(request.GetLimit())
	if limit < 1 || limit > 100 {
		return nil, status.Error(codes.InvalidArgument, "limit must be between 1 and 100")
	}
	invocation, err := s.resolver.Resolve(ctx, request.GetTaskId(), request.GetRunId())
	if err != nil {
		if errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
			return nil, status.Error(codes.PermissionDenied, "Agent Task policy denied")
		}
		return nil, status.Error(codes.Internal, "Agent Task policy lookup failed")
	}
	items, err := s.capability.ListConversations(ctx, invocation, limit)
	if err != nil {
		if errors.Is(err, application.ErrAgentCapabilityDenied) {
			return nil, status.Error(codes.PermissionDenied, "Agent Capability denied")
		}
		return nil, status.Error(codes.Internal, "Agent conversation list failed")
	}
	response := &agentv1.ListConversationsResponse{Conversations: make([]*agentv1.ConversationSnapshot, 0, len(items))}
	for _, item := range items {
		if item != nil {
			response.Conversations = append(response.Conversations, conversationToProto(item))
		}
	}
	return response, nil
}

func conversationToProto(item *model.Conversation) *agentv1.ConversationSnapshot {
	return &agentv1.ConversationSnapshot{
		ConversationKey: item.ConversationKey, TargetId: item.TargetUUID, TargetType: int32(item.TargetType),
		LastMessageId: item.LastMessageUUID, LastMessageSeq: item.LastMessageSeq,
		LastMessagePreview: item.LastMessagePreview, LastMessageAtUnixMs: item.LastMessageAt.UnixMilli(),
		ReadSeq: item.ReadSeq, UnreadCount: int32(item.UnreadCount),
	}
}
