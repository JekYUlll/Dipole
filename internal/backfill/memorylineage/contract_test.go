package memorylineage

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestManifestAndReceiptRoundTrip(t *testing.T) {
	manifest, err := NewManifest(42, 100)
	if err != nil {
		t.Fatalf("NewManifest() error = %v", err)
	}
	if _, err := ParseManifest(mustJSON(manifest)); err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}
	receipt, err := BuildReceipt(manifest, Result{HighWatermarkID: 42, LastProcessedID: 42, Processed: 4, References: 8, Inserted: 6, Duplicates: 2})
	if err != nil {
		t.Fatalf("BuildReceipt() error = %v", err)
	}
	parsed, err := ParseReceipt(mustJSON(receipt))
	if err != nil {
		t.Fatalf("ParseReceipt() error = %v", err)
	}
	if !parsed.Complete || parsed.ContentRead || parsed.DeletionAuthority || parsed.RuntimeAuthority {
		t.Fatalf("receipt = %+v", parsed)
	}
}

func TestContractsRejectDriftAndSensitiveExpansion(t *testing.T) {
	manifest, err := NewManifest(42, 100)
	if err != nil {
		t.Fatal(err)
	}
	encoded := mustJSON(manifest)
	encoded = []byte(strings.Replace(string(encoded), `"batchSize":100`, `"batchSize":101`, 1))
	if _, err := ParseManifest(encoded); err == nil {
		t.Fatal("expected manifest hash drift")
	}
	receipt, err := BuildReceipt(manifest, Result{HighWatermarkID: 42, LastProcessedID: 41, Processed: 1, References: 1, Inserted: 1})
	if err != nil {
		t.Fatal(err)
	}
	receipt.ContentRead = true
	if _, err := ParseReceipt(mustJSON(receipt)); err == nil {
		t.Fatal("expected content authority rejection")
	}
	var unknown map[string]any
	if err := json.Unmarshal(mustJSON(manifest), &unknown); err != nil {
		t.Fatal(err)
	}
	unknown["owner"] = "principal"
	if _, err := ParseManifest(mustJSON(unknown)); err == nil {
		t.Fatal("expected unknown field rejection")
	}
}

func TestBuildReceiptRejectsCounterDrift(t *testing.T) {
	manifest, err := NewManifest(42, 100)
	if err != nil {
		t.Fatal(err)
	}
	_, err = BuildReceipt(manifest, Result{HighWatermarkID: 42, LastProcessedID: 42, References: 1, Inserted: 2})
	if err == nil {
		t.Fatal("expected counter drift rejection")
	}
}

func TestContractsRejectTrailingJSON(t *testing.T) {
	manifest, err := NewManifest(42, 100)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseManifest(append(mustJSON(manifest), []byte(`{}`)...)); err == nil {
		t.Fatal("expected trailing JSON rejection")
	}
}

func TestApprovalBindsManifestAndOperator(t *testing.T) {
	manifest, err := NewManifest(42, 100)
	if err != nil {
		t.Fatal(err)
	}
	approval, err := NewApproval(manifest, "memory-lineage-v1", "ops-a", "review-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseApproval(mustJSON(approval), manifest, "memory-lineage-v1"); err != nil {
		t.Fatalf("ParseApproval() error = %v", err)
	}
	approval.JobName = "other-job"
	if _, err := ParseApproval(mustJSON(approval), manifest, "memory-lineage-v1"); err == nil {
		t.Fatal("expected job binding rejection")
	}
}

func TestContractFilesAreBounded(t *testing.T) {
	manifest, err := NewManifest(42, 100)
	if err != nil {
		t.Fatal(err)
	}
	path := t.TempDir() + "/manifest.json"
	if err := os.WriteFile(path, mustJSON(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseManifestFile(path); err != nil {
		t.Fatalf("ParseManifestFile() error = %v", err)
	}
	if err := os.WriteFile(path, make([]byte, 64*1024+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseManifestFile(path); err == nil {
		t.Fatal("expected oversized contract rejection")
	}
}
