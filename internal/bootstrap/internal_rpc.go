package bootstrap

import (
	"context"
	"errors"
	"fmt"

	agentv1 "github.com/JekYUlll/Dipole/api/gen/go/agent/v1"
	corev1 "github.com/JekYUlll/Dipole/api/gen/go/core/v1"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	platformrpc "github.com/JekYUlll/Dipole/internal/platform/rpc"
	agentgrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/agent"
	grpcauth "github.com/JekYUlll/Dipole/internal/transport/grpc/auth"
	coregrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/core"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

const (
	gatewayServiceName = "dipole-gateway"
	coreServiceName    = "dipole-core"
	messageServiceName = "dipole-message"
	searchServiceName  = "dipole-search"
	syncServiceName    = "dipole-sync"
	agentServiceName   = "dipole-agent"
)

type InternalRPCServer = platformrpc.Server

func NewCoreRPCServer(cfg config.InternalRPC, capability application.CoreCapability) (*InternalRPCServer, error) {
	return newCoreRPCServer(cfg, capability, nil)
}

func NewCoreRPCServerWithAgent(cfg config.InternalRPC, capability application.CoreCapability, agentCapability application.AgentCapabilityV1, resolver application.AgentInvocationResolverV1, admission application.AgentRunAdmissionServiceV1, approvals ...application.AgentApprovalServiceV1) (*InternalRPCServer, error) {
	agentAdapter, err := agentgrpc.NewServer(agentCapability, resolver, admission, approvals...)
	if err != nil {
		return nil, fmt.Errorf("create Agent Capability rpc adapter: %w", err)
	}
	return newCoreRPCServer(cfg, capability, agentAdapter)
}

func NewCoreRPCServerWithAgentControlAndProjection(cfg config.InternalRPC, capability application.CoreCapability, agentCapability application.AgentCapabilityV1, resolver application.AgentInvocationResolverV1, admission application.AgentRunAdmissionServiceV1, approvals application.AgentApprovalServiceV1, controls application.AgentTaskControlAuthorizerV1, projections application.AgentTaskWorkflowProjectionServiceV1, repairs ...application.AgentWorkflowRepairAuditServiceV1) (*InternalRPCServer, error) {
	agentAdapter, err := agentgrpc.NewServerWithControlAndProjection(agentCapability, resolver, admission, approvals, controls, projections, repairs...)
	if err != nil {
		return nil, fmt.Errorf("create Agent Capability rpc adapter: %w", err)
	}
	return newCoreRPCServer(cfg, capability, agentAdapter)
}

func NewCoreRPCServerWithAgentArtifacts(cfg config.InternalRPC, capability application.CoreCapability, agentCapability application.AgentCapabilityV1, resolver application.AgentInvocationResolverV1, admission application.AgentRunAdmissionServiceV1, approvals application.AgentApprovalServiceV1, controls application.AgentTaskControlAuthorizerV1, projections application.AgentTaskWorkflowProjectionServiceV1, repairs application.AgentWorkflowRepairAuditServiceV1, subscriptions application.AgentEventSubscriptionResolverV1, subscriptionControls application.AgentEventSubscriptionControlServiceV1, definitionCatalog application.AgentDefinitionCatalogServiceV1, artifacts application.AgentArtifactServiceV1, toolAudits application.AgentToolInvocationAuditServiceV1, toolRounds application.AgentMCPToolRoundServiceV1, toolTerminals application.AgentMCPToolInvocationTerminalServiceV1, messageCommands application.AgentMessageCommandExecutionV1, approvalGrants application.AgentApprovalGrantResolverV1, promotionControls application.AgentRuntimePromotionControlServiceV1, promotionEvidence application.AgentRuntimePromotionEvidenceReviewServiceV1, readinessPublisher application.AgentMCPReadinessEvidencePublisherV1, readinessResolver application.AgentMCPReadinessEvidenceResolverV1, memoryControls application.AgentMemoryOwnerControlServiceV1, memoryPromotions application.AgentMemoryCandidatePromotionServiceV1, timeline application.AgentTaskTimelineStoreV1, memories ...application.AgentMemoryContextResolverV1) (*InternalRPCServer, error) {
	agentAdapter, err := agentgrpc.NewServerWithControlAndProjection(agentCapability, resolver, admission, approvals, controls, projections, repairs)
	if err != nil {
		return nil, fmt.Errorf("create Agent Capability rpc adapter: %w", err)
	}
	if artifacts != nil {
		if _, err := agentAdapter.WithArtifacts(artifacts); err != nil {
			return nil, fmt.Errorf("configure Agent Artifact rpc adapter: %w", err)
		}
	}
	if timeline != nil {
		if _, err := agentAdapter.WithTaskTimeline(timeline); err != nil {
			return nil, fmt.Errorf("configure Agent Task Timeline rpc adapter: %w", err)
		}
	}
	if toolAudits != nil {
		if _, err := agentAdapter.WithToolAudits(toolAudits); err != nil {
			return nil, fmt.Errorf("configure Agent Tool invocation audit rpc adapter: %w", err)
		}
	}
	if toolRounds != nil {
		if _, err := agentAdapter.WithMCPToolRounds(toolRounds); err != nil {
			return nil, fmt.Errorf("configure Agent MCP Tool round rpc adapter: %w", err)
		}
	}
	if toolTerminals != nil {
		if _, err := agentAdapter.WithMCPToolTerminals(toolTerminals); err != nil {
			return nil, fmt.Errorf("configure Agent MCP Tool terminal rpc adapter: %w", err)
		}
	}
	if messageCommands != nil {
		if _, err := agentAdapter.WithMessageCommands(messageCommands); err != nil {
			return nil, fmt.Errorf("configure Agent Message Command rpc adapter: %w", err)
		}
	}
	if approvalGrants != nil {
		if _, err := agentAdapter.WithApprovalGrants(approvalGrants); err != nil {
			return nil, fmt.Errorf("configure Agent Approval grant rpc adapter: %w", err)
		}
	}
	if promotionControls != nil {
		if _, err := agentAdapter.WithPromotionControls(promotionControls); err != nil {
			return nil, fmt.Errorf("configure Agent Runtime promotion control rpc adapter: %w", err)
		}
	}
	if promotionEvidence != nil {
		if _, err := agentAdapter.WithPromotionEvidence(promotionEvidence); err != nil {
			return nil, fmt.Errorf("configure Agent Runtime promotion evidence review rpc adapter: %w", err)
		}
	}
	if readinessPublisher != nil {
		if _, err := agentAdapter.WithMCPReadinessEvidencePublisher(readinessPublisher); err != nil {
			return nil, fmt.Errorf("configure Agent MCP readiness evidence Publisher rpc adapter: %w", err)
		}
	}
	if readinessResolver != nil {
		if _, err := agentAdapter.WithMCPReadinessEvidenceResolver(readinessResolver); err != nil {
			return nil, fmt.Errorf("configure Agent MCP readiness evidence Resolver rpc adapter: %w", err)
		}
	}
	if subscriptions != nil {
		if _, err := agentAdapter.WithEventSubscriptions(subscriptions); err != nil {
			return nil, fmt.Errorf("configure Agent Event Subscription rpc adapter: %w", err)
		}
	}
	if subscriptionControls != nil {
		if _, err := agentAdapter.WithEventSubscriptionControls(subscriptionControls); err != nil {
			return nil, fmt.Errorf("configure Agent Event Subscription control rpc adapter: %w", err)
		}
	}
	if definitionCatalog != nil {
		if _, err := agentAdapter.WithDefinitionCatalog(definitionCatalog); err != nil {
			return nil, fmt.Errorf("configure Agent Definition catalog rpc adapter: %w", err)
		}
	}
	if len(memories) > 1 {
		return nil, errors.New("at most one Agent Memory resolver may be configured")
	}
	if len(memories) == 1 && memories[0] != nil {
		if _, err := agentAdapter.WithMemories(memories[0]); err != nil {
			return nil, fmt.Errorf("configure Agent Memory rpc adapter: %w", err)
		}
	}
	if memoryControls != nil {
		if _, err := agentAdapter.WithMemoryOwnerControls(memoryControls); err != nil {
			return nil, fmt.Errorf("configure Agent Memory owner control rpc adapter: %w", err)
		}
	}
	if memoryPromotions != nil {
		if _, err := agentAdapter.WithMemoryCandidatePromotions(memoryPromotions); err != nil {
			return nil, fmt.Errorf("configure Agent Memory candidate promotion rpc adapter: %w", err)
		}
	}
	return newCoreRPCServer(cfg, capability, agentAdapter)
}

func newCoreRPCServer(cfg config.InternalRPC, capability application.CoreCapability, agentAdapter *agentgrpc.Server) (*InternalRPCServer, error) {
	adapter, err := coregrpc.NewServer(capability)
	if err != nil {
		return nil, fmt.Errorf("create core rpc adapter: %w", err)
	}
	allowed := []string{messageServiceName, gatewayServiceName, searchServiceName, syncServiceName}
	if agentAdapter != nil {
		allowed = append(allowed, agentServiceName)
	}
	return newInternalRPCServer(cfg, cfg.CoreListenAddress, allowed, func(server *grpc.Server) {
		corev1.RegisterCoreCapabilityServiceServer(server, adapter)
		if agentAdapter != nil {
			agentv1.RegisterAgentCapabilityServiceServer(server, agentAdapter)
		}
	}, restrictCoreServiceMethods)
}

func restrictCoreServiceMethods(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	caller, _ := grpcauth.CallerService(ctx)
	if caller == agentServiceName &&
		info.FullMethod != agentv1.AgentCapabilityService_AdmitRun_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_CompleteRun_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_FinishRun_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_RequestApproval_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_ResolveApproval_FullMethodName &&
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

func newInternalRPCServer(cfg config.InternalRPC, address string, allowedCallers []string, register func(*grpc.Server), additionalInterceptors ...grpc.UnaryServerInterceptor) (*InternalRPCServer, error) {
	return platformrpc.NewServer(cfg, address, allowedCallers, register, additionalInterceptors...)
}

func dialInternalRPC(ctx context.Context, cfg config.InternalRPC, target string, serviceCredentials grpcauth.Credentials) (*grpc.ClientConn, error) {
	return platformrpc.Dial(ctx, cfg, target, serviceCredentials)
}
