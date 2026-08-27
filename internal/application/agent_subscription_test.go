package application

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAgentEventSubscriptionV1ValidatesDeterministicFilters(t *testing.T) {
	t.Parallel()

	base := AgentEventSubscriptionV1{
		SubscriptionUUID: "SUB-1", DefinitionUUID: "DEF-1", DefinitionVersion: 1,
		TenantID: "dipole", AgentUUID: "UAI", Status: AgentSubscriptionStatusActive,
		EventType: "message.direct.created", ResourceType: "conversation", ResourceID: "*",
		FilterKind: AgentSubscriptionFilterAll, FilterJSON: json.RawMessage(`{}`),
	}
	if err := base.Validate(); err != nil {
		t.Fatalf("valid all subscription: %v", err)
	}
	base.FilterKind = AgentSubscriptionFilterMessageContainsAny
	base.FilterJSON = json.RawMessage(`{"terms":["incident","延期"]}`)
	if err := base.Validate(); err != nil {
		t.Fatalf("valid keyword subscription: %v", err)
	}
}

func TestAgentEventSubscriptionV1RejectsUnsafeOrAmbiguousPolicies(t *testing.T) {
	t.Parallel()

	base := AgentEventSubscriptionV1{
		SubscriptionUUID: "SUB-1", DefinitionUUID: "DEF-1", DefinitionVersion: 1,
		TenantID: "dipole", AgentUUID: "UAI", Status: AgentSubscriptionStatusActive,
		EventType: "message.direct.created", ResourceType: "conversation", ResourceID: "*",
		FilterKind: AgentSubscriptionFilterMessageContainsAny,
	}
	for _, filter := range []string{`{}`, `{"terms":[]}`, `{"terms":["ok"],"model":"gpt"}`, `{"terms":["bad\nterm"]}`} {
		candidate := base
		candidate.FilterJSON = json.RawMessage(filter)
		if err := candidate.Validate(); err == nil {
			t.Fatalf("filter %s should be rejected", filter)
		}
	}
	base.FilterJSON = json.RawMessage(`{"terms":["incident"]}`)
	base.ResourceType = ""
	if err := base.Validate(); err == nil {
		t.Fatal("missing resource type should be rejected")
	}
	base.ResourceType = "conversation"
	base.AgentUUID = strings.Repeat("a", 25)
	if err := base.Validate(); err == nil {
		t.Fatal("oversized Agent UUID should be rejected")
	}
}
