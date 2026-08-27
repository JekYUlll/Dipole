package application_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/JekYUlll/Dipole/internal/application"
)

func TestAgentPolicyPersistenceV1HasLanguageNeutralContract(t *testing.T) {
	t.Parallel()
	path := filepath.Join("..", "..", "contracts", "agent-policy", "v1", "schema.json")
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Agent Policy persistence contract: %v", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(payload, &schema); err != nil {
		t.Fatalf("decode Agent Policy persistence contract: %v", err)
	}
	if schema["$id"] != "https://dipole.local/contracts/agent-policy/v1/schema.json" {
		t.Fatalf("unexpected Agent Policy schema ID %q", schema["$id"])
	}
	if schema["x-dipole-version"] != application.AgentPolicyPersistenceVersionV1 {
		t.Fatalf("schema version = %q, want %q", schema["x-dipole-version"], application.AgentPolicyPersistenceVersionV1)
	}
	definitions, ok := schema["$defs"].(map[string]any)
	if !ok {
		t.Fatal("Agent Policy schema is missing definitions")
	}
	for _, name := range []string{"resource_scope", "definition_version", "task", "approval"} {
		if definitions[name] == nil {
			t.Fatalf("Agent Policy schema is missing %s", name)
		}
	}
}
