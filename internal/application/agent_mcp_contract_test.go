package application

import (
	"errors"
	"testing"
)

func TestAgentMCPContractAcceptsHTTPResource(t *testing.T) {
	for _, resource := range []string{AgentMCPResource, "http://agent.local/mcp"} {
		if err := ValidateAgentMCPResource(resource); err != nil {
			t.Fatalf("resource %q should be accepted: %v", resource, err)
		}
	}
}

func TestAgentMCPResourceIdentifierUsesConfiguredValue(t *testing.T) {
	if got := AgentMCPResourceIdentifier("  https://agent.local/mcp "); got != "https://agent.local/mcp" {
		t.Fatalf("configured resource was not normalized: %q", got)
	}
	if got := AgentMCPResourceIdentifier(" "); got != AgentMCPResource {
		t.Fatalf("empty resource should use default: %q", got)
	}
}

func TestAgentTokenSessionCarriesVerifierResult(t *testing.T) {
	session := AgentTokenSession{UserUUID: "U1", TokenID: "T1"}
	if session.UserUUID != "U1" || session.TokenID != "T1" {
		t.Fatalf("unexpected token session: %+v", session)
	}
}

func TestAgentMCPContractRejectsUnsafeResource(t *testing.T) {
	for _, resource := range []string{"", "ftp://agent.local/mcp", "https://agent.local/mcp?token=secret", "https://user:pass@agent.local/mcp", "https://agent.local/mcp#fragment"} {
		if err := ValidateAgentMCPResource(resource); !errors.Is(err, ErrInvalidAgentMCPResource) {
			t.Fatalf("resource %q should be rejected with contract error, got %v", resource, err)
		}
	}
}
