package application

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentWorkflowRepairExecutionPlanV1RemainsDryRunOnly(t *testing.T) {
	root := filepath.Join("..", "..", "contracts", "agent-workflow-repair", "v1")
	payload, err := os.ReadFile(filepath.Join(root, "execution-plan.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(payload, &schema); err != nil {
		t.Fatal(err)
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("properties are required")
	}
	mode, ok := properties["mode"].(map[string]any)
	if !ok || mode["const"] != "dry_run" {
		t.Fatalf("mode = %+v", mode)
	}
	if schema["additionalProperties"] != false {
		t.Fatal("execution plan must reject additive command fields")
	}
	examplePayload, err := os.ReadFile(filepath.Join(root, "examples", "dry-run.json"))
	if err != nil {
		t.Fatal(err)
	}
	var example map[string]any
	if err := json.Unmarshal(examplePayload, &example); err != nil {
		t.Fatal(err)
	}
	if example["mode"] != "dry_run" || len(example["approverIds"].([]any)) != 2 {
		t.Fatalf("example = %+v", example)
	}
	if example["proposalStatus"] != "approved" || example["proposerId"] == example["executorId"] {
		t.Fatalf("execution role snapshot = %+v", example)
	}
	proto, err := os.ReadFile(filepath.Join("..", "..", "api", "proto", "dipole", "agent", "v1", "agent.proto"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"ApplyWorkflowRepair", "ExecuteWorkflowRepair", "RollbackWorkflowRepair"} {
		if strings.Contains(string(proto), forbidden) {
			t.Fatalf("production RPC %s must remain absent", forbidden)
		}
	}
}
