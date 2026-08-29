package memorylineage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const RolloutReviewSchemaVersion = "dipole.agent.memory-lineage-backfill-rollout-review.v1"
const DeploymentEvidenceSchemaVersion = "dipole.agent.memory-lineage-backfill-deployment-evidence.v1"

type RolloutReviewInput struct {
	SchemaVersion            string    `json:"schemaVersion"`
	PolicyVersion            string    `json:"policyVersion"`
	ManifestSHA256           string    `json:"manifestSha256"`
	ApprovalSHA256           string    `json:"approvalSha256"`
	ExpectedMigration        uint64    `json:"expectedMigration"`
	ObservedMigration        uint64    `json:"observedMigration"`
	RuntimeRevision          string    `json:"runtimeRevision"`
	ConfigurationSHA256      string    `json:"configurationSha256"`
	MaintenanceWindowStart   time.Time `json:"maintenanceWindowStart"`
	MaintenanceWindowEnd     time.Time `json:"maintenanceWindowEnd"`
	RollbackVerified         bool      `json:"rollbackVerified"`
	BackupVerified           bool      `json:"backupVerified"`
	ReviewerCount            uint32    `json:"reviewerCount"`
	SharedExecutionRequested bool      `json:"sharedExecutionRequested"`
}

type RolloutReviewReceipt struct {
	SchemaVersion       string `json:"schemaVersion"`
	PolicyVersion       string `json:"policyVersion"`
	ManifestSHA256      string `json:"manifestSha256"`
	ApprovalSHA256      string `json:"approvalSha256"`
	InputSHA256         string `json:"inputSha256"`
	ExpectedMigration   uint64 `json:"expectedMigration"`
	ObservedMigration   uint64 `json:"observedMigration"`
	ReviewerCount       uint32 `json:"reviewerCount"`
	MaintenanceWindowOK bool   `json:"maintenanceWindowOk"`
	RollbackVerified    bool   `json:"rollbackVerified"`
	BackupVerified      bool   `json:"backupVerified"`
	Outcome             string `json:"outcome"`
	ExecutionAuthority  bool   `json:"executionAuthority"`
	ContentRead         bool   `json:"contentRead"`
	DeletionAuthority   bool   `json:"deletionAuthority"`
	RuntimeAuthority    bool   `json:"runtimeAuthority"`
	ReceiptSHA256       string `json:"receiptSha256"`
}

type DeploymentEvidence struct {
	SchemaVersion       string    `json:"schemaVersion"`
	ReviewReceiptSHA256 string    `json:"reviewReceiptSha256"`
	DeploymentID        string    `json:"deploymentId"`
	RuntimeRevision     string    `json:"runtimeRevision"`
	ConfigurationSHA256 string    `json:"configurationSha256"`
	ObservedMigration   uint64    `json:"observedMigration"`
	HealthCheckPassed   bool      `json:"healthCheckPassed"`
	RollbackDrillID     string    `json:"rollbackDrillId"`
	RollbackDrillPassed bool      `json:"rollbackDrillPassed"`
	ObservedAt          time.Time `json:"observedAt"`
	SharedEnvironment   bool      `json:"sharedEnvironment"`
}

type DeploymentEvidenceReceipt struct {
	SchemaVersion       string `json:"schemaVersion"`
	ReviewReceiptSHA256 string `json:"reviewReceiptSha256"`
	DeploymentIDSHA256  string `json:"deploymentIdSha256"`
	RuntimeRevision     string `json:"runtimeRevision"`
	ConfigurationSHA256 string `json:"configurationSha256"`
	ObservedMigration   uint64 `json:"observedMigration"`
	HealthCheckPassed   bool   `json:"healthCheckPassed"`
	RollbackDrillPassed bool   `json:"rollbackDrillPassed"`
	SharedEnvironment   bool   `json:"sharedEnvironment"`
	Outcome             string `json:"outcome"`
	ExecutionAuthority  bool   `json:"executionAuthority"`
	ContentRead         bool   `json:"contentRead"`
	DeletionAuthority   bool   `json:"deletionAuthority"`
	RuntimeAuthority    bool   `json:"runtimeAuthority"`
	ReceiptSHA256       string `json:"receiptSha256"`
}

func ParseDeploymentEvidence(data []byte) (DeploymentEvidence, error) {
	var evidence DeploymentEvidence
	if err := decodeStrict(data, &evidence); err != nil {
		return DeploymentEvidence{}, err
	}
	return evidence, nil
}

func BuildDeploymentEvidenceReceipt(review RolloutReviewReceipt, evidence DeploymentEvidence) (DeploymentEvidenceReceipt, error) {
	if _, err := ParseRolloutReviewReceipt(mustJSON(review)); err != nil {
		return DeploymentEvidenceReceipt{}, err
	}
	if evidence.SchemaVersion != DeploymentEvidenceSchemaVersion ||
		evidence.ReviewReceiptSHA256 != review.ReceiptSHA256 ||
		strings.TrimSpace(evidence.DeploymentID) == "" ||
		!validRevision(evidence.RuntimeRevision) ||
		!validSHA256(evidence.ConfigurationSHA256) ||
		evidence.ObservedMigration != 43 ||
		!evidence.HealthCheckPassed || strings.TrimSpace(evidence.RollbackDrillID) == "" ||
		!evidence.RollbackDrillPassed || evidence.ObservedAt.IsZero() || !evidence.SharedEnvironment {
		return DeploymentEvidenceReceipt{}, errors.New("Memory lineage deployment evidence is incomplete")
	}
	receipt := DeploymentEvidenceReceipt{
		SchemaVersion: evidence.SchemaVersion, ReviewReceiptSHA256: review.ReceiptSHA256,
		DeploymentIDSHA256: sha256String(evidence.DeploymentID), RuntimeRevision: evidence.RuntimeRevision,
		ConfigurationSHA256: evidence.ConfigurationSHA256, ObservedMigration: evidence.ObservedMigration,
		HealthCheckPassed: true, RollbackDrillPassed: true, SharedEnvironment: true, Outcome: "recorded",
		ExecutionAuthority: false, ContentRead: false, DeletionAuthority: false, RuntimeAuthority: false,
	}
	receipt.ReceiptSHA256 = deploymentEvidenceReceiptDigest(receipt)
	return receipt, nil
}

func ParseDeploymentEvidenceReceipt(data []byte) (DeploymentEvidenceReceipt, error) {
	var receipt DeploymentEvidenceReceipt
	if err := decodeStrict(data, &receipt); err != nil {
		return DeploymentEvidenceReceipt{}, err
	}
	if receipt.SchemaVersion != DeploymentEvidenceSchemaVersion || receipt.Outcome != "recorded" ||
		!validSHA256(receipt.ReviewReceiptSHA256) || !validSHA256(receipt.DeploymentIDSHA256) ||
		!validRevision(receipt.RuntimeRevision) || !validSHA256(receipt.ConfigurationSHA256) ||
		receipt.ObservedMigration != 43 || !receipt.HealthCheckPassed || !receipt.RollbackDrillPassed ||
		!receipt.SharedEnvironment || receipt.ExecutionAuthority || receipt.ContentRead || receipt.DeletionAuthority || receipt.RuntimeAuthority ||
		!validSHA256(receipt.ReceiptSHA256) || receipt.ReceiptSHA256 != deploymentEvidenceReceiptDigest(receipt) {
		return DeploymentEvidenceReceipt{}, errors.New("Memory lineage deployment evidence receipt is invalid")
	}
	return receipt, nil
}

func ParseRolloutReviewInput(data []byte) (RolloutReviewInput, error) {
	var input RolloutReviewInput
	if err := decodeStrict(data, &input); err != nil {
		return RolloutReviewInput{}, err
	}
	return input, nil
}

func BuildRolloutReviewReceipt(manifest Manifest, approval Approval, input RolloutReviewInput, now time.Time) (RolloutReviewReceipt, error) {
	if _, err := ParseManifest(mustJSON(manifest)); err != nil {
		return RolloutReviewReceipt{}, err
	}
	if approval.ManifestSHA256 != manifest.ManifestSHA256 || !validSHA256(approval.ApprovalSHA256) ||
		input.SchemaVersion != RolloutReviewSchemaVersion || input.PolicyVersion != PolicyVersion ||
		input.ManifestSHA256 != manifest.ManifestSHA256 || input.ApprovalSHA256 != approval.ApprovalSHA256 ||
		input.ExpectedMigration != 43 || input.ObservedMigration != input.ExpectedMigration ||
		!validRevision(input.RuntimeRevision) || !validSHA256(input.ConfigurationSHA256) ||
		input.ReviewerCount != 2 || input.SharedExecutionRequested || !input.RollbackVerified || !input.BackupVerified {
		return RolloutReviewReceipt{}, errors.New("Memory lineage backfill rollout review is incomplete")
	}
	start, end := input.MaintenanceWindowStart.UTC(), input.MaintenanceWindowEnd.UTC()
	now = now.UTC()
	if start.IsZero() || end.IsZero() || !end.After(start) || start.Before(now) || end.Sub(start) > 24*time.Hour {
		return RolloutReviewReceipt{}, errors.New("Memory lineage backfill maintenance window is invalid")
	}
	receipt := RolloutReviewReceipt{
		SchemaVersion: RolloutReviewSchemaVersion, PolicyVersion: PolicyVersion,
		ManifestSHA256: manifest.ManifestSHA256, ApprovalSHA256: approval.ApprovalSHA256,
		InputSHA256: rolloutInputDigest(input), ExpectedMigration: input.ExpectedMigration,
		ObservedMigration: input.ObservedMigration, ReviewerCount: input.ReviewerCount,
		MaintenanceWindowOK: true, RollbackVerified: true, BackupVerified: true,
		Outcome: "eligible", ExecutionAuthority: false, ContentRead: false,
		DeletionAuthority: false, RuntimeAuthority: false,
	}
	receipt.ReceiptSHA256 = rolloutReceiptDigest(receipt)
	return receipt, nil
}

func validRevision(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func sha256String(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func ParseRolloutReviewReceipt(data []byte) (RolloutReviewReceipt, error) {
	var receipt RolloutReviewReceipt
	if err := decodeStrict(data, &receipt); err != nil {
		return RolloutReviewReceipt{}, err
	}
	if receipt.SchemaVersion != RolloutReviewSchemaVersion || receipt.PolicyVersion != PolicyVersion ||
		receipt.Outcome != "eligible" || receipt.ExpectedMigration != 43 || receipt.ObservedMigration != 43 ||
		receipt.ReviewerCount != 2 || !receipt.MaintenanceWindowOK || !receipt.RollbackVerified || !receipt.BackupVerified ||
		receipt.ExecutionAuthority || receipt.ContentRead || receipt.DeletionAuthority || receipt.RuntimeAuthority ||
		!validSHA256(receipt.ManifestSHA256) || !validSHA256(receipt.ApprovalSHA256) || !validSHA256(receipt.InputSHA256) ||
		!validSHA256(receipt.ReceiptSHA256) || receipt.ReceiptSHA256 != rolloutReceiptDigest(receipt) {
		return RolloutReviewReceipt{}, errors.New("Memory lineage backfill rollout receipt is invalid")
	}
	return receipt, nil
}

func rolloutInputDigest(input RolloutReviewInput) string {
	input.SchemaVersion = strings.TrimSpace(input.SchemaVersion)
	input.PolicyVersion = strings.TrimSpace(input.PolicyVersion)
	input.MaintenanceWindowStart = input.MaintenanceWindowStart.UTC()
	input.MaintenanceWindowEnd = input.MaintenanceWindowEnd.UTC()
	encoded, _ := json.Marshal(input)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func rolloutReceiptDigest(receipt RolloutReviewReceipt) string {
	receipt.ReceiptSHA256 = ""
	encoded, _ := json.Marshal(receipt)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func deploymentEvidenceReceiptDigest(receipt DeploymentEvidenceReceipt) string {
	receipt.ReceiptSHA256 = ""
	encoded, _ := json.Marshal(receipt)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}
