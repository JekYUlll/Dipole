package bootstrap

import (
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	corerpc "github.com/JekYUlll/Dipole/internal/services/core/rpc"
	agentgrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/agent"
)

const (
	coreServiceName    = "dipole-core"
	agentServiceName   = "dipole-agent"
	gatewayServiceName = "dipole-gateway"
	messageServiceName = "dipole-message"
	searchServiceName  = "dipole-search"
	syncServiceName    = "dipole-sync"
)

// InternalRPCServer remains as an embedded compatibility type alias. Core RPC
// composition is owned by internal/services/core/rpc.
type InternalRPCServer = corerpc.Server

func NewCoreRPCServerWithAgentControlAndProjection(cfg config.InternalRPC, capability application.CoreCapability, agentCapability application.AgentCapabilityV1, resolver application.AgentInvocationResolverV1, admission application.AgentRunAdmissionServiceV1, approvals application.AgentApprovalServiceV1, controls application.AgentTaskControlAuthorizerV1, projections application.AgentTaskWorkflowProjectionServiceV1, repairs ...application.AgentWorkflowRepairAuditServiceV1) (*InternalRPCServer, error) {
	return corerpc.NewWithAgentControlAndProjection(cfg, capability, agentCapability, resolver, admission, approvals, controls, projections, repairs...)
}

func NewCoreRPCServerWithAgentArtifacts(cfg config.InternalRPC, capability application.CoreCapability, agentCapability application.AgentCapabilityV1, resolver application.AgentInvocationResolverV1, admission application.AgentRunAdmissionServiceV1, approvals application.AgentApprovalServiceV1, controls application.AgentTaskControlAuthorizerV1, projections application.AgentTaskWorkflowProjectionServiceV1, repairs application.AgentWorkflowRepairAuditServiceV1, subscriptions application.AgentEventSubscriptionResolverV1, subscriptionControls application.AgentEventSubscriptionControlServiceV1, definitionCatalog application.AgentDefinitionCatalogServiceV1, artifacts application.AgentArtifactServiceV1, toolAudits application.AgentToolInvocationAuditServiceV1, toolRounds application.AgentMCPToolRoundServiceV1, toolTerminals application.AgentMCPToolInvocationTerminalServiceV1, messageCommands application.AgentMessageCommandExecutionV1, approvalGrants application.AgentApprovalGrantResolverV1, promotionControls application.AgentRuntimePromotionControlServiceV1, promotionEvidence application.AgentRuntimePromotionEvidenceReviewServiceV1, readinessPublisher application.AgentMCPReadinessEvidencePublisherV1, readinessResolver application.AgentMCPReadinessEvidenceResolverV1, memoryControls application.AgentMemoryOwnerControlServiceV1, memoryPromotions application.AgentMemoryCandidatePromotionServiceV1, timeline application.AgentTaskTimelineStoreV1, memories ...application.AgentMemoryContextResolverV1) (*InternalRPCServer, error) {
	return corerpc.NewWithAgentArtifacts(cfg, capability, agentCapability, resolver, admission, approvals, controls, projections, repairs, subscriptions, subscriptionControls, definitionCatalog, artifacts, toolAudits, toolRounds, toolTerminals, messageCommands, approvalGrants, promotionControls, promotionEvidence, readinessPublisher, readinessResolver, memoryControls, memoryPromotions, timeline, memories...)
}

func newCoreRPCServer(cfg config.InternalRPC, capability application.CoreCapability, agentAdapter *agentgrpc.Server) (*InternalRPCServer, error) {
	return corerpc.NewServer(cfg, capability, agentAdapter)
}
