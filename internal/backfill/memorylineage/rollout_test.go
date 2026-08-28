package memorylineage

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestRolloutReviewBindsEvidenceAndKeepsAuthorityClosed(t *testing.T) {
	manifest, err := NewManifest(42, 100)
	if err != nil {
		t.Fatal(err)
	}
	approval, err := NewApproval(manifest, "operator-a", "approver-a", "review-a")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	input := RolloutReviewInput{
		SchemaVersion: RolloutReviewSchemaVersion, PolicyVersion: PolicyVersion,
		ManifestSHA256: manifest.ManifestSHA256, ApprovalSHA256: approval.ApprovalSHA256,
		ExpectedMigration: 43, ObservedMigration: 43,
		RuntimeRevision: strings.Repeat("a", 64), ConfigurationSHA256: strings.Repeat("b", 64),
		MaintenanceWindowStart: now.Add(time.Minute), MaintenanceWindowEnd: now.Add(time.Hour),
		RollbackVerified: true, BackupVerified: true, ReviewerCount: 2,
	}
	receipt, err := BuildRolloutReviewReceipt(manifest, approval, input, now)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Outcome != "eligible" || receipt.ExecutionAuthority || receipt.ContentRead || receipt.DeletionAuthority || receipt.RuntimeAuthority {
		t.Fatalf("receipt = %+v", receipt)
	}
	if _, err := ParseRolloutReviewReceipt(mustJSON(receipt)); err != nil {
		t.Fatalf("ParseRolloutReviewReceipt() error = %v", err)
	}
}

func TestRolloutReviewRejectsDriftAndIncompleteChecks(t *testing.T) {
	manifest, _ := NewManifest(42, 100)
	approval, _ := NewApproval(manifest, "operator-a", "approver-a", "review-a")
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	input := RolloutReviewInput{SchemaVersion: RolloutReviewSchemaVersion, PolicyVersion: PolicyVersion, ManifestSHA256: manifest.ManifestSHA256, ApprovalSHA256: approval.ApprovalSHA256, ExpectedMigration: 42, ObservedMigration: 43, RuntimeRevision: strings.Repeat("a", 64), ConfigurationSHA256: strings.Repeat("b", 64), MaintenanceWindowStart: now.Add(time.Minute), MaintenanceWindowEnd: now.Add(time.Hour), RollbackVerified: true, BackupVerified: true, ReviewerCount: 2}
	if _, err := BuildRolloutReviewReceipt(manifest, approval, input, now); err == nil {
		t.Fatal("expected migration drift rejection")
	}
	input.ExpectedMigration = 43
	input.BackupVerified = false
	if _, err := BuildRolloutReviewReceipt(manifest, approval, input, now); err == nil {
		t.Fatal("expected backup check rejection")
	}
	var unknown map[string]any
	if err := json.Unmarshal(mustJSON(input), &unknown); err != nil {
		t.Fatal(err)
	}
	unknown["principal"] = "sensitive"
	if _, err := ParseRolloutReviewInput(mustJSON(unknown)); err == nil {
		t.Fatal("expected unknown field rejection")
	}
}
