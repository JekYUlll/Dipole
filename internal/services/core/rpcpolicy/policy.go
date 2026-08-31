package rpcpolicy

import (
	"context"

	agentv1 "github.com/JekYUlll/Dipole/api/gen/go/agent/v1"
	corev1 "github.com/JekYUlll/Dipole/api/gen/go/core/v1"
	grpcauth "github.com/JekYUlll/Dipole/internal/transport/grpc/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

const (
	agentServiceName  = "dipole-agent"
	searchServiceName = "dipole-search"
	syncServiceName   = "dipole-sync"
)

// RestrictAgentServiceMethods owns the caller-to-method policy for Core's
// Agent capability endpoint without depending on bootstrap lifecycle packages.
func RestrictAgentServiceMethods(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	caller, _ := grpcauth.CallerService(ctx)
	if caller == agentServiceName &&
		info.FullMethod != agentv1.AgentCapabilityService_AdmitRun_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_CompleteRun_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_FinishRun_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_RequestApproval_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_ResolveApproval_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_ConsumeApproval_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_ResolveApprovalGrant_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_ListConversations_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_ReadConversation_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_AuthorizeTaskControl_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_ListAgentTaskTimeline_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_ResolveMcpContext_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_BeginMcpToolInvocation_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_ResolveMcpToolCommand_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_ClaimMcpToolRound_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_FinishMcpToolRound_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_FinishMcpToolInvocation_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_FinishMcpToolInvocationFromRound_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_ProjectTaskWorkflowState_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_ListTaskWorkflowProjectionSnapshots_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_CreateArtifact_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_MatchEventSubscriptions_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_ListContextMemories_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_CommitMemoryPromotionReceipt_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_PublishMcpReadinessEvidence_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_ResolveFreshMcpReadinessEvidence_FullMethodName &&
		info.FullMethod != healthv1.Health_Check_FullMethodName {
		return nil, status.Error(codes.PermissionDenied, "Agent service is not allowed to call this Core capability")
	}
	if caller == searchServiceName &&
		info.FullMethod != corev1.CoreCapabilityService_ListSearchConversationKeys_FullMethodName &&
		info.FullMethod != healthv1.Health_Check_FullMethodName {
		return nil, status.Error(codes.PermissionDenied, "Search service is not allowed to call this Core capability")
	}
	if caller == syncServiceName &&
		info.FullMethod != corev1.CoreCapabilityService_GetGroupMember_FullMethodName &&
		info.FullMethod != healthv1.Health_Check_FullMethodName {
		return nil, status.Error(codes.PermissionDenied, "Sync service is not allowed to call this Core capability")
	}
	return handler(ctx, request)
}
