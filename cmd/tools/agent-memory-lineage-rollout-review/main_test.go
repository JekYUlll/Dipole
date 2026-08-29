package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/operations/agent/memorylineage"
)

func TestRunRequiresReviewInputs(t *testing.T) {
	if err := run("", "", "", "", "", ""); err == nil {
		t.Fatal("expected rollout review input requirement")
	}
}

func TestRunWritesEligibleReadOnlyReceipt(t *testing.T) {
	dir := t.TempDir()
	manifest, err := memorylineage.NewManifest(42, 100)
	if err != nil {
		t.Fatal(err)
	}
	approval, err := memorylineage.NewApproval(manifest, "job", "operator-a", "approver-a")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	input := memorylineage.RolloutReviewInput{
		SchemaVersion: memorylineage.RolloutReviewSchemaVersion, PolicyVersion: memorylineage.PolicyVersion,
		ManifestSHA256: manifest.ManifestSHA256, ApprovalSHA256: approval.ApprovalSHA256,
		ExpectedMigration: 43, ObservedMigration: 43, RuntimeRevision: strings.Repeat("a", 64), ConfigurationSHA256: strings.Repeat("b", 64),
		MaintenanceWindowStart: now.Add(time.Minute), MaintenanceWindowEnd: now.Add(time.Hour), RollbackVerified: true, BackupVerified: true, ReviewerCount: 2,
	}
	write := func(name string, value any) string {
		t.Helper()
		path := filepath.Join(dir, name)
		data, marshalErr := json.Marshal(value)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if writeErr := os.WriteFile(path, data, 0o600); writeErr != nil {
			t.Fatal(writeErr)
		}
		return path
	}
	manifestPath := write("manifest.json", manifest)
	approvalPath := write("approval.json", approval)
	inputPath := write("input.json", input)
	receiptPath := filepath.Join(dir, "receipt.json")
	if err := run("job", manifestPath, approvalPath, inputPath, receiptPath, now.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := memorylineage.ParseRolloutReviewReceipt(data)
	if err != nil || receipt.Outcome != "eligible" || receipt.ExecutionAuthority {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
}
