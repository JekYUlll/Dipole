package rpcpolicy

import (
	"testing"

	agentv1 "github.com/JekYUlll/Dipole/api/gen/go/agent/v1"
)

func TestAgentServiceMethodAllowlistIncludesTaskTimeline(t *testing.T) {
	if !isAgentServiceMethodAllowed(agentv1.AgentCapabilityService_ListAgentTaskTimeline_FullMethodName) {
		t.Fatal("Agent Task Timeline must be available to the Agent Runtime")
	}
	if isAgentServiceMethodAllowed("/dipole.agent.v1.AgentCapabilityService/Unknown") {
		t.Fatal("unknown Agent capability must remain denied")
	}
}
