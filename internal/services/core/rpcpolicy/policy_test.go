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
	if isAgentServiceMethodAllowed("/dipole.agent.v1.AgentCapabilityService/Unknown") {
		t.Fatal("unknown Agent capability must remain denied")
	}
}
