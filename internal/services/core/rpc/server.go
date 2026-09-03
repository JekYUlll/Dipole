// Package rpc owns the Core service RPC composition, including the optional
// Agent capability surface used by the embedded compatibility runtime.
package rpc

import (
	"errors"
	"fmt"

	agentv1 "github.com/JekYUlll/Dipole/api/gen/go/agent/v1"
	corev1 "github.com/JekYUlll/Dipole/api/gen/go/core/v1"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	platformrpc "github.com/JekYUlll/Dipole/internal/platform/rpc"
	corepolicy "github.com/JekYUlll/Dipole/internal/services/core/rpcpolicy"
	agentgrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/agent"
	coregrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/core"
	"google.golang.org/grpc"
)

const (
	gatewayServiceName = "dipole-gateway"
	messageServiceName = "dipole-message"
	searchServiceName  = "dipole-search"
	syncServiceName    = "dipole-sync"
	agentServiceName   = "dipole-agent"
)

type Server = platformrpc.Server

// NewServer creates the Core RPC surface with an optional Agent adapter.
func NewServer(cfg config.InternalRPC, capability application.CoreCapability, agentAdapter agentv1.AgentCapabilityServiceServer) (*Server, error) {
	adapter, err := coregrpc.NewServer(capability)
	if err != nil {
		return nil, fmt.Errorf("create core rpc adapter: %w", err)
	}
	allowed := []string{messageServiceName, gatewayServiceName, searchServiceName, syncServiceName}
	if agentAdapter != nil {
		allowed = append(allowed, agentServiceName)
	}
	return platformrpc.NewServer(cfg, cfg.CoreListenAddress, allowed, func(server *grpc.Server) {
		corev1.RegisterCoreCapabilityServiceServer(server, adapter)
		if agentAdapter != nil {
			agentv1.RegisterAgentCapabilityServiceServer(server, agentAdapter)
		}
	}, corepolicy.RestrictAgentServiceMethods)
}

// NewWithAgentControlAndProjection creates the Core RPC surface and the
// Agent adapter used by the embedded compatibility runtime.
func NewWithAgentControlAndProjection(cfg config.InternalRPC, capability application.CoreCapability, agentCapability application.AgentCapabilityV1, resolver application.AgentInvocationResolverV1, admission application.AgentRunAdmissionServiceV1, approvals application.AgentApprovalServiceV1, controls application.AgentTaskControlAuthorizerV1, projections application.AgentTaskWorkflowProjectionServiceV1, repairs ...application.AgentWorkflowRepairAuditServiceV1) (*Server, error) {
	agentAdapter, err := agentgrpc.NewServerWithControlAndProjection(agentCapability, resolver, admission, approvals, controls, projections, repairs...)
	if err != nil {
		return nil, fmt.Errorf("create Agent Capability rpc adapter: %w", err)
	}
	return NewServer(cfg, capability, agentAdapter)
}

// NewWithAgentArtifacts creates the complete optional Agent RPC surface.
func NewWithAgentArtifacts(cfg config.InternalRPC, capability application.CoreCapability, agentCapability application.AgentCapabilityV1, resolver application.AgentInvocationResolverV1, admission application.AgentRunAdmissionServiceV1, approvals application.AgentApprovalServiceV1, controls application.AgentTaskControlAuthorizerV1, projections application.AgentTaskWorkflowProjectionServiceV1, repairs application.AgentWorkflowRepairAuditServiceV1, subscriptions application.AgentEventSubscriptionResolverV1, subscriptionControls application.AgentEventSubscriptionControlServiceV1, definitionCatalog application.AgentDefinitionCatalogServiceV1, artifacts application.AgentArtifactServiceV1, toolAudits application.AgentToolInvocationAuditServiceV1, toolRounds application.AgentMCPToolRoundServiceV1, toolTerminals application.AgentMCPToolInvocationTerminalServiceV1, messageCommands application.AgentMessageCommandExecutionV1, approvalGrants application.AgentApprovalGrantResolverV1, promotionControls application.AgentRuntimePromotionControlServiceV1, promotionEvidence application.AgentRuntimePromotionEvidenceReviewServiceV1, readinessPublisher application.AgentMCPReadinessEvidencePublisherV1, readinessResolver application.AgentMCPReadinessEvidenceResolverV1, memoryControls application.AgentMemoryOwnerControlServiceV1, memoryPromotions application.AgentMemoryCandidatePromotionServiceV1, timeline application.AgentTaskTimelineStoreV1, memoryPromotionCommits application.AgentMemoryPromotionReceiptCommitServiceV1, repairExecutor application.AgentWorkflowRepairExecutorV1, memories ...application.AgentMemoryContextResolverV1) (*Server, error) {
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
	if memoryPromotionCommits != nil {
		if _, err := agentAdapter.WithMemoryPromotionReceiptCommits(memoryPromotionCommits); err != nil {
			return nil, fmt.Errorf("configure Agent Memory promotion receipt commit rpc adapter: %w", err)
		}
	}
	if repairExecutor != nil {
		if _, err := agentAdapter.WithWorkflowRepairExecutor(repairExecutor); err != nil {
			return nil, fmt.Errorf("configure Agent Workflow repair executor rpc adapter: %w", err)
		}
	}
	return NewServer(cfg, capability, agentAdapter)
}
