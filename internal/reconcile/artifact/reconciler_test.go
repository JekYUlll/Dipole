package artifactreconcile

import (
	"context"
	"testing"
	"time"
)

func TestReconcilerProducesBoundedDryRunEvidence(t *testing.T) {
	now := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	prefix := "agent-artifacts/v1/"
	referenced := prefix + "dipole/task-a/" + hash("1") + "/" + hash("a")
	orphan := prefix + "dipole/task-b/" + hash("2") + "/" + hash("b")
	young := prefix + "dipole/task-c/" + hash("3") + "/" + hash("c")
	unsafe := prefix + "unexpected"
	reconciler := mustReconciler(t, &objectSourceStub{objects: []ObjectEvidenceV1{
		{Key: referenced, SizeBytes: 10, LastModified: now.Add(-48 * time.Hour), ETag: "etag-a"},
		{Key: orphan, SizeBytes: 20, LastModified: now.Add(-48 * time.Hour), ETag: "etag-b"},
		{Key: young, SizeBytes: 30, LastModified: now.Add(-time.Hour), ETag: "etag-c"},
		{Key: unsafe, SizeBytes: 40, LastModified: now.Add(-48 * time.Hour), ETag: "etag-d"},
	}}, &metadataStoreStub{keys: map[string]bool{referenced: true}}, ConfigV1{
		Bucket: "dipole-agent-artifacts", Prefix: prefix, MinimumAge: 24 * time.Hour, MaxExamples: 10,
	})
	reconciler.now = func() time.Time { return now }

	report, err := reconciler.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.SchemaVersion != SchemaVersionV1 || report.Mode != "dry_run" || report.DeleteAuthorized {
		t.Fatalf("unsafe report contract: %+v", report)
	}
	if report.ScannedCount != 4 || report.ReferencedCount != 1 || report.YoungCount != 1 || report.OrphanCandidateCount != 1 || report.UnsafeCount != 1 {
		t.Fatalf("unexpected counters: %+v", report)
	}
	if report.Consistent || len(report.Examples) != 2 || report.EvidenceSHA256 == "" {
		t.Fatalf("missing bounded evidence: %+v", report)
	}
	if report.Examples[0].Kind != "orphan_candidate" || report.Examples[0].ContentSHA256 != hash("b") {
		t.Fatalf("unexpected orphan evidence: %+v", report.Examples[0])
	}
	if report.Examples[1].Kind != "invalid_object_key" || report.Examples[1].EligibleForCleanup {
		t.Fatalf("unsafe object became cleanup eligible: %+v", report.Examples[1])
	}
	if err := report.VerifyEvidence(); err != nil {
		t.Fatalf("verify report evidence: %v", err)
	}
}

func TestReconcilerRejectsUnsafeConfiguration(t *testing.T) {
	for name, cfg := range map[string]ConfigV1{
		"short age":   {Bucket: "dipole-agent-artifacts", Prefix: "agent-artifacts/v1/", MinimumAge: time.Hour, MaxExamples: 1},
		"wide prefix": {Bucket: "dipole-agent-artifacts", Prefix: "", MinimumAge: 24 * time.Hour, MaxExamples: 1},
		"no examples": {Bucket: "dipole-agent-artifacts", Prefix: "agent-artifacts/v1/", MinimumAge: 24 * time.Hour},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := New(&objectSourceStub{}, &metadataStoreStub{}, cfg); err == nil {
				t.Fatal("expected unsafe reconcile configuration to fail")
			}
		})
	}
}

func mustReconciler(t *testing.T, source ObjectSourceV1, metadata MetadataStoreV1, cfg ConfigV1) *ReconcilerV1 {
	t.Helper()
	reconciler, err := New(source, metadata, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return reconciler
}

func hash(char string) string {
	value := ""
	for len(value) < 64 {
		value += char
	}
	return value
}

type objectSourceStub struct{ objects []ObjectEvidenceV1 }

func (s *objectSourceStub) Walk(_ context.Context, _ string, visit func(ObjectEvidenceV1) error) error {
	for _, object := range s.objects {
		if err := visit(object); err != nil {
			return err
		}
	}
	return nil
}

type metadataStoreStub struct{ keys map[string]bool }

func (s *metadataStoreStub) ExistsByObjectKey(_ context.Context, _, key string) (bool, error) {
	return s.keys[key], nil
}
