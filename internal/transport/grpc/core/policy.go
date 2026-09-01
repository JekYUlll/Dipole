package coregrpc

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

// RestrictServiceMethods applies the least-privilege method policy for the
// Core capability server. It is shared by standalone and compatibility
// bootstrap paths while the latter is being retired.
func RestrictServiceMethods(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	caller, _ := grpcauth.CallerService(ctx)
	if caller == "dipole-agent" &&
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
		info.FullMethod != agentv1.AgentCapabilityService_ResolveMcpContext_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_BeginMcpToolInvocation_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_ResolveMcpToolCommand_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_ClaimMcpToolRound_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_FinishMcpToolRound_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_FinishMcpToolInvocation_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_FinishMcpToolInvocationFromRound_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_ExecuteMcpMessageCommand_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_ProjectTaskWorkflowState_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_ListTaskWorkflowProjectionSnapshots_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_CreateArtifact_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_MatchEventSubscriptions_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_ListContextMemories_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_PublishMcpReadinessEvidence_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_ResolveFreshMcpReadinessEvidence_FullMethodName &&
		info.FullMethod != healthv1.Health_Check_FullMethodName {
		return nil, status.Error(codes.PermissionDenied, "Agent service is not allowed to call this Core capability")
	}
	if caller == "dipole-search" &&
		info.FullMethod != corev1.CoreCapabilityService_ListSearchConversationKeys_FullMethodName &&
		info.FullMethod != healthv1.Health_Check_FullMethodName {
		return nil, status.Error(codes.PermissionDenied, "Search service is not allowed to call this Core capability")
	}
	if caller == "dipole-sync" &&
		info.FullMethod != corev1.CoreCapabilityService_GetGroupMember_FullMethodName &&
		info.FullMethod != healthv1.Health_Check_FullMethodName {
		return nil, status.Error(codes.PermissionDenied, "Sync service is not allowed to call this Core capability")
	}
	return handler(ctx, request)
}
