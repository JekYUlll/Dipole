package application_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/JekYUlll/Dipole/internal/application"
)

func TestAgentCapabilityV1HasLanguageNeutralContract(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "contracts", "agent-capabilities", "v1", "schema.json")
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Agent Capability v1 contract: %v", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(payload, &schema); err != nil {
		t.Fatalf("decode Agent Capability v1 contract: %v", err)
	}
	if schema["$id"] != "https://dipole.local/contracts/agent-capabilities/v1/schema.json" {
		t.Fatalf("unexpected Agent Capability schema ID %q", schema["$id"])
	}
	operations, ok := schema["x-dipole-operations"].([]any)
	if !ok || len(operations) != 5 {
		t.Fatalf("Agent Capability contract must declare five operations, got %#v", schema["x-dipole-operations"])
	}
	want := map[string]bool{
		"get_user_profile": true, "list_direct_messages": true, "list_conversations": true,
		"read_conversation": true, "send_system_message": true,
	}
	for _, operation := range operations {
		name, ok := operation.(string)
		if !ok || !want[name] {
			t.Fatalf("unexpected Agent Capability operation %#v", operation)
		}
		delete(want, name)
	}
	if len(want) != 0 {
		t.Fatalf("missing Agent Capability operations: %#v", want)
	}
	descriptors, ok := schema["x-dipole-capabilities"].([]any)
	if !ok || len(descriptors) != 5 {
		t.Fatalf("Agent Capability contract must publish five descriptors, got %#v", schema["x-dipole-capabilities"])
	}
	wantDescriptors := map[string]application.AgentCapabilityDescriptorV1{}
	for _, id := range []string{
		application.AgentCapabilityUserProfileRead,
		application.AgentCapabilityDirectMessagesRead,
		application.AgentCapabilityConversationsList,
		application.AgentCapabilityConversationRead,
		application.AgentCapabilitySystemMessageSend,
	} {
		descriptor, ok := application.AgentCapabilityDescriptorByIDV1(id)
		if !ok {
			t.Fatalf("Go registry is missing descriptor %s", id)
		}
		wantDescriptors[id] = descriptor
	}
	for _, raw := range descriptors {
		descriptor, ok := raw.(map[string]any)
		if !ok || descriptor["id"] == "" || descriptor["required_permission"] == "" {
			t.Fatalf("invalid Agent Capability descriptor %#v", raw)
		}
		risk, _ := descriptor["risk"].(string)
		if risk != string(application.AgentCapabilityRiskRead) && risk != string(application.AgentCapabilityRiskWrite) && risk != string(application.AgentCapabilityRiskDestructive) {
			t.Fatalf("invalid Agent Capability risk %#v", descriptor)
		}
		id, _ := descriptor["id"].(string)
		want, ok := wantDescriptors[id]
		if !ok {
			t.Fatalf("schema has unknown Agent Capability descriptor %q", id)
		}
		permission, _ := descriptor["required_permission"].(string)
		approval, _ := descriptor["approval_required"].(bool)
		if risk != string(want.Risk) || permission != want.RequiredPermission || approval != want.ApprovalRequired {
			t.Fatalf("schema descriptor drift for %s: schema=%#v Go=%+v", id, descriptor, want)
		}
		delete(wantDescriptors, id)
	}
	if len(wantDescriptors) != 0 {
		t.Fatalf("schema is missing Agent Capability descriptors: %#v", wantDescriptors)
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok || properties["resource_scopes"] == nil {
		t.Fatalf("Agent Capability contract must declare trusted resource_scopes: %#v", properties)
	}
}
