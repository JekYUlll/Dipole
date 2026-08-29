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

func TestRunWritesEvidenceReceipt(t *testing.T) {
	dir := t.TempDir()
	review := validReview(t)
	reviewPath := writeJSON(t, dir, "review.json", review)
	evidence := memorylineage.DeploymentEvidence{SchemaVersion: memorylineage.DeploymentEvidenceSchemaVersion, ReviewReceiptSHA256: review.ReceiptSHA256, DeploymentID: "deploy-1", RuntimeRevision: strings.Repeat("a", 40), ConfigurationSHA256: strings.Repeat("b", 64), ObservedMigration: 43, HealthCheckPassed: true, RollbackDrillID: "rollback-1", RollbackDrillPassed: true, ObservedAt: time.Date(2026, 8, 28, 11, 0, 0, 0, time.UTC), SharedEnvironment: true}
	evidencePath := writeJSON(t, dir, "evidence.json", evidence)
	out := filepath.Join(dir, "receipt.json")
	if err := run(reviewPath, evidencePath, out); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := memorylineage.ParseDeploymentEvidenceReceipt(data); err != nil {
		t.Fatal(err)
	}
}

func TestRunRequiresInputs(t *testing.T) {
	if err := run("", "", ""); err == nil {
		t.Fatal("expected required input error")
	}
}

func validReview(t *testing.T) memorylineage.RolloutReviewReceipt {
	t.Helper()
	manifest, err := memorylineage.NewManifest(42, 100)
	if err != nil {
		t.Fatal(err)
	}
	approval, err := memorylineage.NewApproval(manifest, "operator-a", "approver-a", "review-a")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	input := memorylineage.RolloutReviewInput{SchemaVersion: memorylineage.RolloutReviewSchemaVersion, PolicyVersion: memorylineage.PolicyVersion, ManifestSHA256: manifest.ManifestSHA256, ApprovalSHA256: approval.ApprovalSHA256, ExpectedMigration: 43, ObservedMigration: 43, RuntimeRevision: strings.Repeat("a", 40), ConfigurationSHA256: strings.Repeat("b", 64), MaintenanceWindowStart: now.Add(time.Minute), MaintenanceWindowEnd: now.Add(time.Hour), RollbackVerified: true, BackupVerified: true, ReviewerCount: 2}
	review, err := memorylineage.BuildRolloutReviewReceipt(manifest, approval, input, now)
	if err != nil {
		t.Fatal(err)
	}
	return review
}

func writeJSON(t *testing.T, dir, name string, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
