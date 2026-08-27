package agentgrpc

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	grpccommon "github.com/JekYUlll/Dipole/internal/transport/grpc/common"
	agentv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/agent/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	agentv1.UnimplementedAgentCapabilityServiceServer
	capability application.AgentCapabilityV1
	resolver   application.AgentInvocationResolverV1
	admission  application.AgentRunAdmissionServiceV1
	approvals  application.AgentApprovalServiceV1
	controls   application.AgentTaskControlAuthorizerV1
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
	return &agentv1.AuthorizeTaskControlResponse{TaskId: authorization.TaskUUID, TaskStatus: string(authorization.Status)}, nil
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
			RequestID: request.GetContext().GetRequestId(), TraceID: request.GetContext().GetTraceId(), EventID: request.GetEventId(),
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
