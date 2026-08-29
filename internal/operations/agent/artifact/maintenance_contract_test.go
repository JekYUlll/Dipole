package artifactcleanup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArtifactMaintenanceSchemasRemainDryRunOnly(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..", "contracts", "agent-artifact-maintenance", "v1")
	for _, file := range []string{"authorization.schema.json", "receipt.schema.json", filepath.Join("examples", "dry-run-authorization.json"), filepath.Join("examples", "would-delete-receipt.json")} {
		body, err := os.ReadFile(filepath.Join(root, file))
		if err != nil {
			t.Fatal(err)
		}
		var document map[string]any
		if err := json.Unmarshal(body, &document); err != nil {
			t.Fatalf("decode %s: %v", file, err)
		}
	}
	authorization := readContractJSON(t, filepath.Join(root, "authorization.schema.json"))
	receipt := readContractJSON(t, filepath.Join(root, "receipt.schema.json"))
	for name, schema := range map[string]map[string]any{"authorization": authorization, "receipt": receipt} {
		if schema["additionalProperties"] != false {
			t.Fatalf("%s schema permits additive execution fields", name)
		}
		properties := schema["properties"].(map[string]any)
		if properties["mode"].(map[string]any)["const"] != "dry_run" {
			t.Fatalf("%s mode can widen", name)
		}
	}
	authorizationProperties := authorization["properties"].(map[string]any)
	if authorizationProperties["delete_adapter_available"].(map[string]any)["const"] != false || authorizationProperties["metadata_recheck_required"].(map[string]any)["const"] != true {
		t.Fatal("authorization does not fix delete absence and metadata recheck")
	}
	receiptProperties := receipt["properties"].(map[string]any)
	if receiptProperties["delete_attempted"].(map[string]any)["const"] != false || receiptProperties["deleted"].(map[string]any)["const"] != false {
		t.Fatal("receipt permits a deletion side effect")
	}
}

func TestArtifactMaintenanceExamplesMatchGoEvidenceContract(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..", "contracts", "agent-artifact-maintenance", "v1", "examples")
	var authorization AuthorizationV1
	readStrictContract(t, filepath.Join(root, "dry-run-authorization.json"), &authorization)
	if err := authorization.Verify(); err != nil {
		t.Fatalf("authorization example: %v", err)
	}
	var receipt ReceiptV1
	readStrictContract(t, filepath.Join(root, "would-delete-receipt.json"), &receipt)
	if err := receipt.Verify(); err != nil {
		t.Fatalf("receipt example: %v", err)
	}
	if receipt.AuthorizationSHA256 != authorization.AuthorizationSHA256 {
		t.Fatal("receipt example is not bound to the authorization example")
	}
}

func TestProductionAPIRemainsWithoutArtifactDeleteMethod(t *testing.T) {
	proto, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "api", "proto", "dipole", "agent", "v1", "agent.proto"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"DeleteArtifact", "ExecuteArtifactMaintenance", "ApplyArtifactMaintenance"} {
		if strings.Contains(string(proto), forbidden) {
			t.Fatalf("production API exposes %s", forbidden)
		}
	}
}

func readContractJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(body, &document); err != nil {
		t.Fatal(err)
	}
	return document
}

func readStrictContract(t *testing.T, path string, target any) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		t.Fatal(err)
	}
}
