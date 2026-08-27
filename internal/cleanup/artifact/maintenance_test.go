package artifactcleanup

import (
	"context"
	"testing"
	"time"

	artifactreconcile "github.com/JekYUlll/Dipole/internal/reconcile/artifact"
)

func TestDryRunMaintenanceProducesWouldDeleteReceiptWithoutSideEffect(t *testing.T) {
	report, candidate, now := orphanReport(t)
	authorization, err := NewAuthorizationV1(report, candidate.ObjectKey, AuthorizationRolesV1{
		ProposalID: "proposal-1", ProposerID: "operator-1", ApproverIDs: []string{"operator-2", "operator-3"},
		ExecutorID: "operator-4", MaintenanceGrantVersion: "artifact-maintenance/v1",
	}, now, now.Add(10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if authorization.Mode != "dry_run" || authorization.DeleteAdapterAvailable || authorization.AuthorizationSHA256 == "" {
		t.Fatalf("unsafe authorization: %+v", authorization)
	}
	evaluator, err := NewDryRunEvaluatorV1(&metadataStub{}, &objectStub{evidence: ObjectStateV1{
		Found: true, ETag: candidate.ETag, SizeBytes: candidate.SizeBytes, LastModified: candidate.LastModified,
	}}, func() time.Time { return now.Add(time.Minute) })
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := evaluator.Evaluate(context.Background(), authorization)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Outcome != OutcomeWouldDeleteV1 || receipt.DeleteAttempted || receipt.Deleted || receipt.ReceiptSHA256 == "" {
		t.Fatalf("unsafe receipt: %+v", receipt)
	}
	if err := receipt.Verify(); err != nil {
		t.Fatalf("verify receipt: %v", err)
	}
}

func TestDryRunMaintenanceBlocksMetadataAndObjectDrift(t *testing.T) {
	report, candidate, now := orphanReport(t)
	authorization, err := NewAuthorizationV1(report, candidate.ObjectKey, validRoles(), now, now.Add(10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	for name, test := range map[string]struct {
		metadata MetadataRecheckerV1
		object   ObjectInspectorV1
		outcome  string
	}{
		"metadata appeared": {metadata: &metadataStub{exists: true}, object: &objectStub{evidence: matchingObject(candidate)}, outcome: OutcomeBlockedMetadataV1},
		"object missing":    {metadata: &metadataStub{}, object: &objectStub{}, outcome: OutcomeBlockedObjectMissingV1},
		"object drift":      {metadata: &metadataStub{}, object: &objectStub{evidence: ObjectStateV1{Found: true, ETag: "changed", SizeBytes: candidate.SizeBytes, LastModified: candidate.LastModified}}, outcome: OutcomeBlockedEvidenceDriftV1},
	} {
		t.Run(name, func(t *testing.T) {
			evaluator, _ := NewDryRunEvaluatorV1(test.metadata, test.object, func() time.Time { return now.Add(time.Minute) })
			receipt, err := evaluator.Evaluate(context.Background(), authorization)
			if err != nil || receipt.Outcome != test.outcome || receipt.DeleteAttempted || receipt.Deleted {
				t.Fatalf("receipt=%+v err=%v", receipt, err)
			}
		})
	}
}

func TestMaintenanceAuthorizationRejectsRoleEvidenceAndLifetimeDrift(t *testing.T) {
	report, candidate, now := orphanReport(t)
	invalid := []struct {
		name    string
		report  artifactreconcile.ReportV1
		key     string
		roles   AuthorizationRolesV1
		expires time.Time
	}{
		{name: "one approver", report: report, key: candidate.ObjectKey, roles: AuthorizationRolesV1{ProposalID: "p", ProposerID: "u1", ApproverIDs: []string{"u2"}, ExecutorID: "u4", MaintenanceGrantVersion: "v1"}, expires: now.Add(time.Minute)},
		{name: "self execution", report: report, key: candidate.ObjectKey, roles: AuthorizationRolesV1{ProposalID: "p", ProposerID: "u1", ApproverIDs: []string{"u2", "u3"}, ExecutorID: "u1", MaintenanceGrantVersion: "v1"}, expires: now.Add(time.Minute)},
		{name: "long lifetime", report: report, key: candidate.ObjectKey, roles: validRoles(), expires: now.Add(16 * time.Minute)},
		{name: "missing candidate", report: report, key: candidate.ObjectKey + "-other", roles: validRoles(), expires: now.Add(time.Minute)},
	}
	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewAuthorizationV1(test.report, test.key, test.roles, now, test.expires); err == nil {
				t.Fatal("expected unsafe authorization to fail")
			}
		})
	}
}

func TestReceiptVerificationRejectsContradictoryOutcomeState(t *testing.T) {
	report, candidate, now := orphanReport(t)
	authorization, err := NewAuthorizationV1(report, candidate.ObjectKey, validRoles(), now, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	receipt := finalizeReceipt(ReceiptV1{
		SchemaVersion: ReceiptSchemaV1, AuthorizationSHA256: authorization.AuthorizationSHA256,
		Mode: "dry_run", Outcome: OutcomeWouldDeleteV1, Bucket: authorization.Bucket,
		ObjectKey: authorization.ObjectKey, CheckedAt: now, MetadataExists: true,
		ObjectFound: true, ObjectEvidenceMatched: true,
	})
	if err := receipt.Verify(); err == nil {
		t.Fatal("contradictory would-delete receipt was accepted")
	}
}

func orphanReport(t *testing.T) (artifactreconcile.ReportV1, artifactreconcile.ExampleV1, time.Time) {
	t.Helper()
	now := time.Now().UTC()
	key := "agent-artifacts/v1/dipole/task/" + repeat("1") + "/" + repeat("a")
	reconciler, err := artifactreconcile.New(&sourceStub{object: artifactreconcile.ObjectEvidenceV1{
		Key: key, SizeBytes: 12, LastModified: now.Add(-48 * time.Hour), ETag: "etag-1",
	}}, &reconcileMetadataStub{}, artifactreconcile.ConfigV1{
		Bucket: "dipole-agent-artifacts", Prefix: "agent-artifacts/v1/", MinimumAge: 24 * time.Hour, MaxExamples: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	report, err := reconciler.Run(context.Background())
	if err != nil || len(report.Examples) != 1 {
		t.Fatalf("report=%+v err=%v", report, err)
	}
	return report, report.Examples[0], report.CompletedAt
}

func validRoles() AuthorizationRolesV1 {
	return AuthorizationRolesV1{ProposalID: "proposal-1", ProposerID: "operator-1", ApproverIDs: []string{"operator-2", "operator-3"}, ExecutorID: "operator-4", MaintenanceGrantVersion: "artifact-maintenance/v1"}
}

func matchingObject(candidate artifactreconcile.ExampleV1) ObjectStateV1 {
	return ObjectStateV1{Found: true, ETag: candidate.ETag, SizeBytes: candidate.SizeBytes, LastModified: candidate.LastModified}
}

func repeat(value string) string {
	result := ""
	for len(result) < 64 {
		result += value
	}
	return result
}

type sourceStub struct {
	object artifactreconcile.ObjectEvidenceV1
}

func (s *sourceStub) Walk(_ context.Context, _ string, visit func(artifactreconcile.ObjectEvidenceV1) error) error {
	return visit(s.object)
}

type reconcileMetadataStub struct{}

func (*reconcileMetadataStub) ExistsByObjectKey(context.Context, string, string) (bool, error) {
	return false, nil
}

type metadataStub struct{ exists bool }

func (s *metadataStub) ExistsByObjectKey(context.Context, string, string) (bool, error) {
	return s.exists, nil
}

type objectStub struct{ evidence ObjectStateV1 }

func (s *objectStub) Inspect(context.Context, string, string) (ObjectStateV1, error) {
	return s.evidence, nil
}
