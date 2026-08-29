package memorylineage

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	ManifestSchemaVersion = "dipole.agent.memory-lineage-backfill-manifest.v1"
	ReceiptSchemaVersion  = "dipole.agent.memory-lineage-backfill-receipt.v1"
	ApprovalSchemaVersion = "dipole.agent.memory-lineage-backfill-approval.v1"
	PolicyVersion         = "memory-lineage-backfill-v1"
)

type Manifest struct {
	SchemaVersion         string `json:"schemaVersion"`
	PolicyVersion         string `json:"policyVersion"`
	SourceHighWatermarkID uint64 `json:"sourceHighWatermarkId"`
	BatchSize             uint32 `json:"batchSize"`
	ManifestSHA256        string `json:"manifestSha256"`
}

type Receipt struct {
	SchemaVersion         string `json:"schemaVersion"`
	PolicyVersion         string `json:"policyVersion"`
	ManifestSHA256        string `json:"manifestSha256"`
	SourceHighWatermarkID uint64 `json:"sourceHighWatermarkId"`
	LastProcessedID       uint64 `json:"lastProcessedId"`
	ProcessedPlans        uint64 `json:"processedPlans"`
	References            uint64 `json:"references"`
	Inserted              uint64 `json:"inserted"`
	Duplicates            uint64 `json:"duplicates"`
	Complete              bool   `json:"complete"`
	ContentRead           bool   `json:"contentRead"`
	DeletionAuthority     bool   `json:"deletionAuthority"`
	RuntimeAuthority      bool   `json:"runtimeAuthority"`
	ReceiptSHA256         string `json:"receiptSha256"`
}

type Approval struct {
	SchemaVersion         string `json:"schemaVersion"`
	PolicyVersion         string `json:"policyVersion"`
	JobName               string `json:"jobName"`
	OperatorID            string `json:"operatorId"`
	ApproverID            string `json:"approverId"`
	ManifestSHA256        string `json:"manifestSha256"`
	SourceHighWatermarkID uint64 `json:"sourceHighWatermarkId"`
	Approved              bool   `json:"approved"`
	ApprovalSHA256        string `json:"approvalSha256"`
}

func NewManifest(highWatermark uint64, batchSize uint32) (Manifest, error) {
	manifest := Manifest{SchemaVersion: ManifestSchemaVersion, PolicyVersion: PolicyVersion, SourceHighWatermarkID: highWatermark, BatchSize: batchSize}
	if err := validateManifest(manifest); err != nil {
		return Manifest{}, err
	}
	manifest.ManifestSHA256 = manifestDigest(manifest)
	return manifest, nil
}

func ParseManifest(data []byte) (Manifest, error) {
	var manifest Manifest
	if err := decodeStrict(data, &manifest); err != nil {
		return Manifest{}, err
	}
	if err := validateManifest(manifest); err != nil {
		return Manifest{}, err
	}
	want := manifestDigest(manifest)
	if manifest.ManifestSHA256 != want {
		return Manifest{}, errors.New("Memory lineage backfill manifest hash is invalid")
	}
	return manifest, nil
}

func ParseManifestFile(path string) (Manifest, error) {
	data, err := readContractFile(path)
	if err != nil {
		return Manifest{}, err
	}
	return ParseManifest(data)
}

func BuildReceipt(manifest Manifest, result Result) (Receipt, error) {
	if _, err := ParseManifest(mustJSON(manifest)); err != nil {
		return Receipt{}, err
	}
	if result.LastProcessedID > result.HighWatermarkID || result.HighWatermarkID != manifest.SourceHighWatermarkID || result.Inserted+result.Duplicates > result.References {
		return Receipt{}, errors.New("Memory lineage backfill result is inconsistent")
	}
	receipt := Receipt{
		SchemaVersion: ReceiptSchemaVersion, PolicyVersion: PolicyVersion, ManifestSHA256: manifest.ManifestSHA256,
		SourceHighWatermarkID: result.HighWatermarkID, LastProcessedID: result.LastProcessedID,
		ProcessedPlans: result.Processed, References: result.References, Inserted: result.Inserted, Duplicates: result.Duplicates,
		Complete: result.LastProcessedID == result.HighWatermarkID, ContentRead: false, DeletionAuthority: false, RuntimeAuthority: false,
	}
	receipt.ReceiptSHA256 = receiptDigest(receipt)
	return receipt, nil
}

func ParseReceipt(data []byte) (Receipt, error) {
	var receipt Receipt
	if err := decodeStrict(data, &receipt); err != nil {
		return Receipt{}, err
	}
	if receipt.SchemaVersion != ReceiptSchemaVersion || receipt.PolicyVersion != PolicyVersion || !validSHA256(receipt.ManifestSHA256) || !validSHA256(receipt.ReceiptSHA256) || receipt.ContentRead || receipt.DeletionAuthority || receipt.RuntimeAuthority {
		return Receipt{}, errors.New("Memory lineage backfill receipt is invalid")
	}
	if receipt.LastProcessedID > receipt.SourceHighWatermarkID || receipt.Complete != (receipt.LastProcessedID == receipt.SourceHighWatermarkID) || receipt.Inserted+receipt.Duplicates > receipt.References {
		return Receipt{}, errors.New("Memory lineage backfill receipt counters are inconsistent")
	}
	want := receiptDigest(receipt)
	if receipt.ReceiptSHA256 != want {
		return Receipt{}, errors.New("Memory lineage backfill receipt hash is invalid")
	}
	return receipt, nil
}

func NewApproval(manifest Manifest, jobName, operatorID, approverID string) (Approval, error) {
	if _, err := ParseManifest(mustJSON(manifest)); err != nil {
		return Approval{}, err
	}
	approval := Approval{
		SchemaVersion: ApprovalSchemaVersion, PolicyVersion: PolicyVersion,
		JobName: strings.TrimSpace(jobName), OperatorID: strings.TrimSpace(operatorID), ApproverID: strings.TrimSpace(approverID),
		ManifestSHA256: manifest.ManifestSHA256, SourceHighWatermarkID: manifest.SourceHighWatermarkID,
		Approved: true,
	}
	if approval.JobName == "" || approval.OperatorID == "" || approval.ApproverID == "" || approval.OperatorID == approval.ApproverID {
		return Approval{}, errors.New("Memory lineage backfill approval identity is required")
	}
	approval.ApprovalSHA256 = approvalDigest(approval)
	return approval, nil
}

func ParseApproval(data []byte, manifest Manifest, jobName string) (Approval, error) {
	if _, err := ParseManifest(mustJSON(manifest)); err != nil {
		return Approval{}, err
	}
	var approval Approval
	if err := decodeStrict(data, &approval); err != nil {
		return Approval{}, err
	}
	if approval.SchemaVersion != ApprovalSchemaVersion || approval.PolicyVersion != PolicyVersion ||
		!approval.Approved || strings.TrimSpace(approval.JobName) != strings.TrimSpace(jobName) ||
		!validSHA256(approval.ManifestSHA256) || approval.ManifestSHA256 != manifest.ManifestSHA256 ||
		approval.SourceHighWatermarkID != manifest.SourceHighWatermarkID || strings.TrimSpace(approval.OperatorID) == "" ||
		strings.TrimSpace(approval.ApproverID) == "" || strings.TrimSpace(approval.OperatorID) == strings.TrimSpace(approval.ApproverID) ||
		!validSHA256(approval.ApprovalSHA256) {
		return Approval{}, errors.New("Memory lineage backfill approval is invalid")
	}
	if approval.ApprovalSHA256 != approvalDigest(approval) {
		return Approval{}, errors.New("Memory lineage backfill approval hash is invalid")
	}
	return approval, nil
}

func ParseApprovalFile(path string, manifest Manifest, jobName string) (Approval, error) {
	data, err := readContractFile(path)
	if err != nil {
		return Approval{}, err
	}
	return ParseApproval(data, manifest, jobName)
}

func validateManifest(manifest Manifest) error {
	if manifest.SchemaVersion != ManifestSchemaVersion || manifest.PolicyVersion != PolicyVersion || manifest.BatchSize == 0 || manifest.BatchSize > MaxBatchSize {
		return errors.New("Memory lineage backfill manifest is invalid")
	}
	if manifest.ManifestSHA256 != "" && !validSHA256(manifest.ManifestSHA256) {
		return errors.New("Memory lineage backfill manifest hash is invalid")
	}
	return nil
}

func decodeStrict(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode Memory lineage backfill contract: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return errors.New("decode Memory lineage backfill contract: trailing JSON")
		}
		return fmt.Errorf("decode Memory lineage backfill contract: trailing data: %w", err)
	}
	return nil
}

func manifestDigest(manifest Manifest) string {
	manifest.ManifestSHA256 = ""
	encoded, _ := json.Marshal(manifest)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func receiptDigest(receipt Receipt) string {
	receipt.ReceiptSHA256 = ""
	encoded, _ := json.Marshal(receipt)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func approvalDigest(approval Approval) string {
	approval.ApprovalSHA256 = ""
	encoded, _ := json.Marshal(approval)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func mustJSON(value any) []byte { encoded, _ := json.Marshal(value); return encoded }

func readContractFile(path string) ([]byte, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("Memory lineage backfill contract path is required")
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() > 64*1024 {
		return nil, errors.New("Memory lineage backfill contract file is invalid")
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) > 64*1024 {
		return nil, errors.New("Memory lineage backfill contract file is invalid")
	}
	return data, nil
}
