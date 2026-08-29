package memorylineage

import (
	"strings"
	"testing"
	"time"
)

func TestDeploymentEvidenceBindsReviewAndRecordsReadOnlyResult(t *testing.T) {
	review := validRolloutReview(t)
	evidence := DeploymentEvidence{
		SchemaVersion: DeploymentEvidenceSchemaVersion, ReviewReceiptSHA256: review.ReceiptSHA256,
		DeploymentID: "deploy-20260828-01", RuntimeRevision: strings.Repeat("a", 40),
		ConfigurationSHA256: strings.Repeat("b", 64), ObservedMigration: 43,
		HealthCheckPassed: true, RollbackDrillID: "rollback-20260828-01", RollbackDrillPassed: true,
		ObservedAt: time.Date(2026, 8, 28, 11, 0, 0, 0, time.UTC), SharedEnvironment: true,
	}
	receipt, err := BuildDeploymentEvidenceReceipt(review, evidence)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Outcome != "recorded" || receipt.ExecutionAuthority || receipt.ContentRead || receipt.DeletionAuthority || receipt.RuntimeAuthority {
		t.Fatalf("receipt = %+v", receipt)
	}
	if _, err := ParseDeploymentEvidenceReceipt(mustJSON(receipt)); err != nil {
		t.Fatal(err)
	}
}

func TestDeploymentEvidenceRejectsUnboundOrFailedDrill(t *testing.T) {
	review := validRolloutReview(t)
	evidence := DeploymentEvidence{SchemaVersion: DeploymentEvidenceSchemaVersion, ReviewReceiptSHA256: "wrong", DeploymentID: "deploy", RuntimeRevision: strings.Repeat("a", 40), ConfigurationSHA256: strings.Repeat("b", 64), ObservedMigration: 43, HealthCheckPassed: true, RollbackDrillID: "rollback", RollbackDrillPassed: false, ObservedAt: time.Now().UTC(), SharedEnvironment: true}
	if _, err := BuildDeploymentEvidenceReceipt(review, evidence); err == nil {
		t.Fatal("expected unbound or failed evidence rejection")
	}
}

func validRolloutReview(t *testing.T) RolloutReviewReceipt {
	t.Helper()
	manifest, err := NewManifest(42, 100)
	if err != nil {
		t.Fatal(err)
	}
	approval, err := NewApproval(manifest, "operator-a", "approver-a", "review-a")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	input := RolloutReviewInput{SchemaVersion: RolloutReviewSchemaVersion, PolicyVersion: PolicyVersion, ManifestSHA256: manifest.ManifestSHA256, ApprovalSHA256: approval.ApprovalSHA256, ExpectedMigration: 43, ObservedMigration: 43, RuntimeRevision: strings.Repeat("a", 40), ConfigurationSHA256: strings.Repeat("b", 64), MaintenanceWindowStart: now.Add(time.Minute), MaintenanceWindowEnd: now.Add(time.Hour), RollbackVerified: true, BackupVerified: true, ReviewerCount: 2}
	review, err := BuildRolloutReviewReceipt(manifest, approval, input, now)
	if err != nil {
		t.Fatal(err)
	}
	return review
}
