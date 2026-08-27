package artifactcleanup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	artifactreconcile "github.com/JekYUlll/Dipole/internal/reconcile/artifact"
)

const (
	AuthorizationSchemaV1 = "dipole.agent.artifact-maintenance-authorization.v1"
	ReceiptSchemaV1       = "dipole.agent.artifact-maintenance-receipt.v1"
	MaximumLifetimeV1     = 15 * time.Minute

	OutcomeWouldDeleteV1          = "would_delete"
	OutcomeBlockedMetadataV1      = "blocked_metadata_present"
	OutcomeBlockedObjectMissingV1 = "blocked_object_missing"
	OutcomeBlockedEvidenceDriftV1 = "blocked_evidence_drift"
	OutcomeBlockedExpiredV1       = "blocked_expired"
)

type AuthorizationRolesV1 struct {
	ProposalID              string   `json:"proposal_id"`
	ProposerID              string   `json:"proposer_id"`
	ApproverIDs             []string `json:"approver_ids"`
	ExecutorID              string   `json:"executor_id"`
	MaintenanceGrantVersion string   `json:"maintenance_grant_version"`
}

type AuthorizationV1 struct {
	SchemaVersion           string    `json:"schema_version"`
	Mode                    string    `json:"mode"`
	Action                  string    `json:"action"`
	ReportEvidenceSHA256    string    `json:"report_evidence_sha256"`
	Bucket                  string    `json:"bucket"`
	Prefix                  string    `json:"prefix"`
	ObjectKey               string    `json:"object_key"`
	ContentSHA256           string    `json:"content_sha256"`
	SizeBytes               int64     `json:"size_bytes"`
	ETag                    string    `json:"etag"`
	LastModified            time.Time `json:"last_modified"`
	EligibleAfter           time.Time `json:"eligible_after"`
	CandidateObservedAt     time.Time `json:"candidate_observed_at"`
	ProposalID              string    `json:"proposal_id"`
	ProposerID              string    `json:"proposer_id"`
	ApproverIDs             []string  `json:"approver_ids"`
	ExecutorID              string    `json:"executor_id"`
	MaintenanceGrantVersion string    `json:"maintenance_grant_version"`
	GeneratedAt             time.Time `json:"generated_at"`
	ExpiresAt               time.Time `json:"expires_at"`
	MetadataRecheckRequired bool      `json:"metadata_recheck_required"`
	DeleteAdapterAvailable  bool      `json:"delete_adapter_available"`
	AuthorizationSHA256     string    `json:"authorization_sha256"`
}

type ObjectStateV1 struct {
	Found        bool
	ETag         string
	SizeBytes    int64
	LastModified time.Time
}

type MetadataRecheckerV1 interface {
	ExistsByObjectKey(context.Context, string, string) (bool, error)
}

type ObjectInspectorV1 interface {
	Inspect(context.Context, string, string) (ObjectStateV1, error)
}

type ReceiptV1 struct {
	SchemaVersion         string    `json:"schema_version"`
	AuthorizationSHA256   string    `json:"authorization_sha256"`
	Mode                  string    `json:"mode"`
	Outcome               string    `json:"outcome"`
	Bucket                string    `json:"bucket"`
	ObjectKey             string    `json:"object_key"`
	CheckedAt             time.Time `json:"checked_at"`
	MetadataExists        bool      `json:"metadata_exists"`
	ObjectFound           bool      `json:"object_found"`
	ObjectEvidenceMatched bool      `json:"object_evidence_matched"`
	DeleteAttempted       bool      `json:"delete_attempted"`
	Deleted               bool      `json:"deleted"`
	ReceiptSHA256         string    `json:"receipt_sha256"`
}

type DryRunEvaluatorV1 struct {
	metadata MetadataRecheckerV1
	objects  ObjectInspectorV1
	now      func() time.Time
}

func NewAuthorizationV1(report artifactreconcile.ReportV1, objectKey string, roles AuthorizationRolesV1, generatedAt, expiresAt time.Time) (AuthorizationV1, error) {
	if err := report.VerifyEvidence(); err != nil {
		return AuthorizationV1{}, fmt.Errorf("verify Artifact reconcile evidence: %w", err)
	}
	if report.DeleteAuthorized || report.Mode != "dry_run" || report.MinimumAgeSeconds < int64(artifactreconcile.MinimumSafeAgeV1/time.Second) || report.OrphanCandidateCount == 0 || report.Consistent {
		return AuthorizationV1{}, errors.New("Artifact reconcile report cannot authorize maintenance review")
	}
	roles, err := normalizeRoles(roles)
	if err != nil {
		return AuthorizationV1{}, err
	}
	generatedAt, expiresAt = generatedAt.UTC(), expiresAt.UTC()
	if generatedAt.IsZero() || !expiresAt.After(generatedAt) || expiresAt.Sub(generatedAt) > MaximumLifetimeV1 {
		return AuthorizationV1{}, errors.New("Artifact maintenance authorization lifetime is invalid")
	}
	objectKey = strings.TrimSpace(objectKey)
	var candidate *artifactreconcile.ExampleV1
	for index := range report.Examples {
		example := &report.Examples[index]
		if example.ObjectKey == objectKey && example.Kind == "orphan_candidate" && example.EligibleForCleanup {
			candidate = example
			break
		}
	}
	if candidate == nil || candidate.ContentSHA256 == "" || candidate.EligibleAfter.After(generatedAt) ||
		!candidate.LastModified.Add(time.Duration(report.MinimumAgeSeconds)*time.Second).Equal(candidate.EligibleAfter) ||
		report.CompletedAt.Before(candidate.EligibleAfter) {
		return AuthorizationV1{}, errors.New("Artifact maintenance candidate is absent or ineligible")
	}
	authorization := AuthorizationV1{
		SchemaVersion: AuthorizationSchemaV1, Mode: "dry_run", Action: "delete_candidate",
		ReportEvidenceSHA256: report.EvidenceSHA256, Bucket: report.Bucket, Prefix: report.Prefix,
		ObjectKey: candidate.ObjectKey, ContentSHA256: candidate.ContentSHA256,
		SizeBytes: candidate.SizeBytes, ETag: candidate.ETag, LastModified: candidate.LastModified.UTC(),
		EligibleAfter: candidate.EligibleAfter.UTC(), CandidateObservedAt: report.CompletedAt.UTC(),
		ProposalID: roles.ProposalID, ProposerID: roles.ProposerID, ApproverIDs: roles.ApproverIDs,
		ExecutorID: roles.ExecutorID, MaintenanceGrantVersion: roles.MaintenanceGrantVersion,
		GeneratedAt: generatedAt, ExpiresAt: expiresAt, MetadataRecheckRequired: true, DeleteAdapterAvailable: false,
	}
	authorization.AuthorizationSHA256 = authorization.hash()
	if err := authorization.Verify(); err != nil {
		return AuthorizationV1{}, err
	}
	return authorization, nil
}

func (a AuthorizationV1) Verify() error {
	if a.SchemaVersion != AuthorizationSchemaV1 || a.Mode != "dry_run" || a.Action != "delete_candidate" ||
		!a.MetadataRecheckRequired || a.DeleteAdapterAvailable || a.AuthorizationSHA256 == "" ||
		a.Bucket != "dipole-agent-artifacts" || a.Prefix != "agent-artifacts/v1/" ||
		!strings.HasPrefix(a.ObjectKey, a.Prefix) || !strings.HasSuffix(a.ObjectKey, "/"+a.ContentSHA256) || !isSHA256(a.ReportEvidenceSHA256) || !isSHA256(a.ContentSHA256) ||
		a.SizeBytes <= 0 || a.SizeBytes > 1<<20 || strings.TrimSpace(a.ETag) == "" || a.LastModified.IsZero() || a.EligibleAfter.IsZero() || a.CandidateObservedAt.IsZero() {
		return errors.New("invalid Artifact maintenance authorization contract")
	}
	normalizedRoles, err := normalizeRoles(AuthorizationRolesV1{
		ProposalID: a.ProposalID, ProposerID: a.ProposerID, ApproverIDs: a.ApproverIDs,
		ExecutorID: a.ExecutorID, MaintenanceGrantVersion: a.MaintenanceGrantVersion,
	})
	if err != nil {
		return err
	}
	if !slices.Equal(normalizedRoles.ApproverIDs, a.ApproverIDs) || a.EligibleAfter.Sub(a.LastModified) < artifactreconcile.MinimumSafeAgeV1 ||
		a.CandidateObservedAt.Before(a.EligibleAfter) || a.GeneratedAt.Before(a.CandidateObservedAt) ||
		!a.ExpiresAt.After(a.GeneratedAt) || a.ExpiresAt.Sub(a.GeneratedAt) > MaximumLifetimeV1 || a.hash() != a.AuthorizationSHA256 {
		return errors.New("Artifact maintenance authorization evidence is invalid")
	}
	return nil
}

func NewDryRunEvaluatorV1(metadata MetadataRecheckerV1, objects ObjectInspectorV1, now func() time.Time) (*DryRunEvaluatorV1, error) {
	if metadata == nil || objects == nil || now == nil {
		return nil, errors.New("Artifact maintenance dry-run dependencies are required")
	}
	return &DryRunEvaluatorV1{metadata: metadata, objects: objects, now: now}, nil
}

func (e *DryRunEvaluatorV1) Evaluate(ctx context.Context, authorization AuthorizationV1) (ReceiptV1, error) {
	if err := authorization.Verify(); err != nil {
		return ReceiptV1{}, err
	}
	checkedAt := e.now().UTC()
	receipt := ReceiptV1{
		SchemaVersion: ReceiptSchemaV1, AuthorizationSHA256: authorization.AuthorizationSHA256,
		Mode: "dry_run", Bucket: authorization.Bucket, ObjectKey: authorization.ObjectKey, CheckedAt: checkedAt,
		DeleteAttempted: false, Deleted: false,
	}
	if checkedAt.After(authorization.ExpiresAt) {
		receipt.Outcome = OutcomeBlockedExpiredV1
		return finalizeReceipt(receipt), nil
	}
	object, err := e.objects.Inspect(ctx, authorization.Bucket, authorization.ObjectKey)
	if err != nil {
		return ReceiptV1{}, fmt.Errorf("inspect Artifact maintenance object: %w", err)
	}
	receipt.ObjectFound = object.Found
	if !object.Found {
		receipt.Outcome = OutcomeBlockedObjectMissingV1
		return finalizeReceipt(receipt), nil
	}
	receipt.ObjectEvidenceMatched = object.ETag == authorization.ETag && object.SizeBytes == authorization.SizeBytes && object.LastModified.UTC().Equal(authorization.LastModified)
	if !receipt.ObjectEvidenceMatched {
		receipt.Outcome = OutcomeBlockedEvidenceDriftV1
		return finalizeReceipt(receipt), nil
	}
	exists, err := e.metadata.ExistsByObjectKey(ctx, authorization.Bucket, authorization.ObjectKey)
	if err != nil {
		return ReceiptV1{}, fmt.Errorf("recheck Artifact maintenance metadata: %w", err)
	}
	receipt.MetadataExists = exists
	if exists {
		receipt.Outcome = OutcomeBlockedMetadataV1
		return finalizeReceipt(receipt), nil
	}
	receipt.Outcome = OutcomeWouldDeleteV1
	return finalizeReceipt(receipt), nil
}

func (r ReceiptV1) Verify() error {
	allowed := []string{OutcomeWouldDeleteV1, OutcomeBlockedMetadataV1, OutcomeBlockedObjectMissingV1, OutcomeBlockedEvidenceDriftV1, OutcomeBlockedExpiredV1}
	if r.SchemaVersion != ReceiptSchemaV1 || r.Mode != "dry_run" || r.DeleteAttempted || r.Deleted ||
		!slices.Contains(allowed, r.Outcome) || !isSHA256(r.AuthorizationSHA256) || r.ReceiptSHA256 == "" ||
		!receiptStateValid(r) || r.hash() != r.ReceiptSHA256 {
		return errors.New("invalid Artifact maintenance receipt")
	}
	return nil
}

func receiptStateValid(receipt ReceiptV1) bool {
	switch receipt.Outcome {
	case OutcomeWouldDeleteV1:
		return receipt.ObjectFound && receipt.ObjectEvidenceMatched && !receipt.MetadataExists
	case OutcomeBlockedMetadataV1:
		return receipt.ObjectFound && receipt.ObjectEvidenceMatched && receipt.MetadataExists
	case OutcomeBlockedObjectMissingV1:
		return !receipt.ObjectFound && !receipt.ObjectEvidenceMatched && !receipt.MetadataExists
	case OutcomeBlockedEvidenceDriftV1:
		return receipt.ObjectFound && !receipt.ObjectEvidenceMatched && !receipt.MetadataExists
	case OutcomeBlockedExpiredV1:
		return !receipt.ObjectFound && !receipt.ObjectEvidenceMatched && !receipt.MetadataExists
	default:
		return false
	}
}

func normalizeRoles(roles AuthorizationRolesV1) (AuthorizationRolesV1, error) {
	roles.ProposalID = strings.TrimSpace(roles.ProposalID)
	roles.ProposerID = strings.TrimSpace(roles.ProposerID)
	roles.ExecutorID = strings.TrimSpace(roles.ExecutorID)
	roles.MaintenanceGrantVersion = strings.TrimSpace(roles.MaintenanceGrantVersion)
	if len(roles.ApproverIDs) != 2 || roles.ProposalID == "" || len(roles.ProposalID) > 64 || roles.ProposerID == "" || roles.ExecutorID == "" || roles.MaintenanceGrantVersion == "" || len(roles.MaintenanceGrantVersion) > 64 {
		return roles, errors.New("Artifact maintenance role snapshot is incomplete")
	}
	roles.ApproverIDs = []string{strings.TrimSpace(roles.ApproverIDs[0]), strings.TrimSpace(roles.ApproverIDs[1])}
	sort.Strings(roles.ApproverIDs)
	identities := []string{roles.ProposerID, roles.ExecutorID, roles.ApproverIDs[0], roles.ApproverIDs[1]}
	for _, identity := range identities {
		if identity == "" || len(identity) > 64 {
			return roles, errors.New("Artifact maintenance identity is invalid")
		}
	}
	seen := make(map[string]struct{}, len(identities))
	for _, identity := range identities {
		if _, exists := seen[identity]; exists {
			return roles, errors.New("Artifact maintenance duties must remain separated")
		}
		seen[identity] = struct{}{}
	}
	return roles, nil
}

func (a AuthorizationV1) hash() string {
	a.AuthorizationSHA256 = ""
	return jsonHash(a)
}

func (r ReceiptV1) hash() string {
	r.ReceiptSHA256 = ""
	return jsonHash(r)
}

func finalizeReceipt(receipt ReceiptV1) ReceiptV1 {
	receipt.ReceiptSHA256 = receipt.hash()
	return receipt
}

func jsonHash(value any) string {
	body, _ := json.Marshal(value)
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:])
}

func isSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
