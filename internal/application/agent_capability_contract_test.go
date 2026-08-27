package application_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
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
}
