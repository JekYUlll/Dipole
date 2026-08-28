package application

import (
	"encoding/json"
	"os"
	"testing"
)

func TestAgentMemoryRetentionLanguageNeutralExamples(t *testing.T) {
	t.Parallel()
	for _, path := range []string{
		"../../contracts/agent-memory-retention/v1/policy.schema.json",
		"../../contracts/agent-memory-retention/v1/policy.example.json",
		"../../contracts/agent-memory-retention/v1/receipt.schema.json",
		"../../contracts/agent-memory-retention/v1/receipt.example.json",
	} {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var value map[string]any
		if err = json.Unmarshal(source, &value); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
	}
	var policy struct {
		Tombstone                   string   `json:"tombstone"`
		ReasonCodes                 []string `json:"reasonCodes"`
		AutomaticExecutionAuthority bool     `json:"automaticExecutionAuthority"`
		PublicAPIAuthority          bool     `json:"publicApiAuthority"`
	}
	source, _ := os.ReadFile("../../contracts/agent-memory-retention/v1/policy.example.json")
	if json.Unmarshal(source, &policy) != nil || policy.Tombstone != AgentMemoryErasedContentV1 || len(policy.ReasonCodes) != 3 || policy.AutomaticExecutionAuthority || policy.PublicAPIAuthority {
		t.Fatalf("retention policy drift: %+v", policy)
	}
}
