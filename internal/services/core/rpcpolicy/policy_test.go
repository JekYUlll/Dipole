package rpcpolicy

import (
	"testing"

	agentv1 "github.com/JekYUlll/Dipole/api/gen/go/agent/v1"
)

func TestAgentServiceMethodAllowlistIncludesRuntimeMethods(t *testing.T) {
	if !isAgentServiceMethodAllowed(agentv1.AgentCapabilityService_ListAgentTaskTimeline_FullMethodName) {
		t.Fatal("Agent Task Timeline must be available to the Agent Runtime")
	}
	if !isAgentServiceMethodAllowed(agentv1.AgentCapabilityService_ExecuteMcpMessageCommand_FullMethodName) {
		t.Fatal("Agent Message Command execution must be available to the Agent Runtime")
	}
	if !isAgentServiceMethodAllowed(agentv1.AgentCapabilityService_AuthorizeSubscriptionMessage_FullMethodName) {
		t.Fatal("subscription auto-reply approval must be available to the Agent Runtime")
	}
	if !isAgentServiceMethodAllowed(agentv1.AgentCapabilityService_ClaimOAuthCallbackHandoff_FullMethodName) ||
		!isAgentServiceMethodAllowed(agentv1.AgentCapabilityService_CompleteOAuthCallbackHandoff_FullMethodName) ||
		!isAgentServiceMethodAllowed(agentv1.AgentCapabilityService_ReleaseOAuthCallbackHandoff_FullMethodName) {
		t.Fatal("OAuth callback handoff transitions must be available only to the Agent Runtime")
	}
	if isAgentServiceMethodAllowed("/dipole.agent.v1.AgentCapabilityService/Unknown") {
		t.Fatal("unknown Agent capability must remain denied")
	}
}

// TestAgentServiceMethodAllowlistCoversRuntimeInvocations enumerates every
// AgentCapabilityService RPC that services/agent-runtime/src actually invokes
// against Core in production, ensuring the policy interceptor does not
// silently strand a Runtime call site with PermissionDenied. New RPCs added
// on the Runtime capability client must extend both the client and this
// enumeration; the reverse direction (allowlist without Runtime caller) is
// still permitted so Gateway-only or forward-declared surfaces do not fail
// this guard.
func TestAgentServiceMethodAllowlistCoversRuntimeInvocations(t *testing.T) {
	runtimeInvoked := []string{
		agentv1.AgentCapabilityService_AdmitRun_FullMethodName,
		agentv1.AgentCapabilityService_CompleteRun_FullMethodName,
		agentv1.AgentCapabilityService_FinishRun_FullMethodName,
		agentv1.AgentCapabilityService_RequestApproval_FullMethodName,
		agentv1.AgentCapabilityService_ResolveApproval_FullMethodName,
		agentv1.AgentCapabilityService_ConsumeApproval_FullMethodName,
		agentv1.AgentCapabilityService_ResolveApprovalGrant_FullMethodName,
		agentv1.AgentCapabilityService_AuthorizeSubscriptionMessage_FullMethodName,
		agentv1.AgentCapabilityService_AuthorizeTaskControl_FullMethodName,
		agentv1.AgentCapabilityService_ListConversations_FullMethodName,
		agentv1.AgentCapabilityService_ReadConversation_FullMethodName,
		agentv1.AgentCapabilityService_SearchConversations_FullMethodName,
		agentv1.AgentCapabilityService_ListAgentTaskTimeline_FullMethodName,
		agentv1.AgentCapabilityService_AppendAgentTaskTimelineEvent_FullMethodName,
		agentv1.AgentCapabilityService_ResolveMcpContext_FullMethodName,
		agentv1.AgentCapabilityService_BeginMcpToolInvocation_FullMethodName,
		agentv1.AgentCapabilityService_ResolveMcpToolCommand_FullMethodName,
		agentv1.AgentCapabilityService_ClaimMcpToolRound_FullMethodName,
		agentv1.AgentCapabilityService_FinishMcpToolRound_FullMethodName,
		agentv1.AgentCapabilityService_FinishMcpToolInvocation_FullMethodName,
		agentv1.AgentCapabilityService_FinishMcpToolInvocationFromRound_FullMethodName,
		agentv1.AgentCapabilityService_ExecuteMcpMessageCommand_FullMethodName,
		agentv1.AgentCapabilityService_ProjectTaskWorkflowState_FullMethodName,
		agentv1.AgentCapabilityService_ListTaskWorkflowProjectionSnapshots_FullMethodName,
		agentv1.AgentCapabilityService_CreateArtifact_FullMethodName,
		agentv1.AgentCapabilityService_MatchEventSubscriptions_FullMethodName,
		agentv1.AgentCapabilityService_ListContextMemories_FullMethodName,
		agentv1.AgentCapabilityService_CommitMemoryPromotionReceipt_FullMethodName,
		agentv1.AgentCapabilityService_PublishMcpReadinessEvidence_FullMethodName,
		agentv1.AgentCapabilityService_ResolveFreshMcpReadinessEvidence_FullMethodName,
	}
	for _, method := range runtimeInvoked {
		if !isAgentServiceMethodAllowed(method) {
			t.Fatalf("Runtime invokes %s but the Core allowlist denies dipole-agent — every Runtime capability client method must also be listed in isAgentServiceMethodAllowed", method)
		}
	}
}
