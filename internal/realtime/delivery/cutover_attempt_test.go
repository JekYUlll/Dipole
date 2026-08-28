package delivery

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCutoverAttemptReducerCompletesHappyPath(t *testing.T) {
	projection := newCutoverAttemptProjection("attempt-a")
	for index, eventType := range []CutoverAttemptEventType{
		CutoverEventSourceCheckpointed,
		CutoverEventFreezeApplied,
		CutoverEventFrozenConfirmed,
		CutoverEventTargetActivated,
		CutoverEventTargetCheckpointed,
		CutoverEventCompleted,
	} {
		event := validCutoverAttemptEvent("attempt-a", uint64(index+1), eventType)
		if err := projection.Apply(event); err != nil {
			t.Fatalf("Apply(%s): %v", eventType, err)
		}
	}
	if projection.State != CutoverAttemptCompleted || projection.RollbackNeedsFreeze {
		t.Fatalf("unexpected projection: %+v", projection)
	}
}

func TestCutoverAttemptReducerRollsBackFromFrozenWithoutSecondFreeze(t *testing.T) {
	projection := newCutoverAttemptProjection("attempt-a")
	for index, eventType := range []CutoverAttemptEventType{
		CutoverEventSourceCheckpointed,
		CutoverEventFreezeApplied,
		CutoverEventFrozenConfirmed,
		CutoverEventRollbackRequested,
		CutoverEventSourceReactivated,
		CutoverEventRollbackCheckpointed,
		CutoverEventRolledBack,
	} {
		if err := projection.Apply(validCutoverAttemptEvent("attempt-a", uint64(index+1), eventType)); err != nil {
			t.Fatalf("Apply(%s): %v", eventType, err)
		}
	}
	if projection.State != CutoverAttemptRolledBack || projection.RollbackNeedsFreeze {
		t.Fatalf("unexpected frozen rollback projection: %+v", projection)
	}
}

func TestCutoverAttemptReducerRequiresFreezeAfterTargetActivation(t *testing.T) {
	projection := newCutoverAttemptProjection("attempt-a")
	for index, eventType := range []CutoverAttemptEventType{
		CutoverEventSourceCheckpointed,
		CutoverEventFreezeApplied,
		CutoverEventFrozenConfirmed,
		CutoverEventTargetActivated,
		CutoverEventRollbackRequested,
	} {
		if err := projection.Apply(validCutoverAttemptEvent("attempt-a", uint64(index+1), eventType)); err != nil {
			t.Fatalf("Apply(%s): %v", eventType, err)
		}
	}
	if !projection.RollbackNeedsFreeze {
		t.Fatal("active target rollback must require a new freeze")
	}
	if err := projection.Apply(validCutoverAttemptEvent("attempt-a", 6, CutoverEventSourceReactivated)); err == nil {
		t.Fatal("source cannot reactivate before rollback freeze")
	}
	for index, eventType := range []CutoverAttemptEventType{
		CutoverEventRollbackFreezeApplied,
		CutoverEventRollbackFrozenConfirmed,
		CutoverEventSourceReactivated,
		CutoverEventRollbackCheckpointed,
		CutoverEventRolledBack,
	} {
		if err := projection.Apply(validCutoverAttemptEvent("attempt-a", uint64(index+6), eventType)); err != nil {
			t.Fatalf("Apply(%s): %v", eventType, err)
		}
	}
	if projection.State != CutoverAttemptRolledBack || projection.RollbackNeedsFreeze {
		t.Fatalf("unexpected active rollback projection: %+v", projection)
	}
}

func TestCutoverAttemptReducerRejectsInvalidSequenceAndBinding(t *testing.T) {
	projection := newCutoverAttemptProjection("attempt-a")
	if err := projection.Apply(validCutoverAttemptEvent("attempt-b", 1, CutoverEventSourceCheckpointed)); err == nil {
		t.Fatal("attempt drift must fail")
	}
	if err := projection.Apply(validCutoverAttemptEvent("attempt-a", 2, CutoverEventSourceCheckpointed)); err == nil {
		t.Fatal("sequence gap must fail")
	}
	if err := projection.Apply(validCutoverAttemptEvent("attempt-a", 1, CutoverEventTargetActivated)); err == nil {
		t.Fatal("state jump must fail")
	}
}

func TestCutoverAttemptJournalPersistsAndDetectsTampering(t *testing.T) {
	now := time.Date(2026, 8, 28, 8, 0, 0, 0, time.UTC)
	manifest := validCutoverAttemptManifest(now)
	directory := t.TempDir()
	journal, err := CreateCutoverAttemptJournal(directory, manifest)
	if err != nil {
		t.Fatal(err)
	}
	for _, eventType := range []CutoverAttemptEventType{CutoverEventSourceCheckpointed, CutoverEventFreezeApplied} {
		if _, err := journal.Append(eventType, strings.Repeat("a", 64), now); err != nil {
			t.Fatalf("Append(%s): %v", eventType, err)
		}
	}
	loaded, err := LoadCutoverAttemptJournal(directory)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Projection.State != CutoverAttemptFreezeApplied || len(loaded.Events) != 2 || !validSHA256(loaded.HeadSHA256) {
		t.Fatalf("unexpected loaded journal: %+v", loaded)
	}
	if _, err := CreateCutoverAttemptJournal(directory, manifest); err == nil {
		t.Fatal("attempt journal manifest must be immutable")
	}

	loaded.Events[1].PreviousEventSHA256 = strings.Repeat("b", 64)
	if _, err := reduceCutoverAttemptEvents(manifest, loaded.Events); err == nil {
		t.Fatal("tampered hash chain must fail")
	}
}

func TestCutoverAttemptJournalPersistsRollbackAfterTargetActivation(t *testing.T) {
	now := time.Date(2026, 8, 28, 8, 0, 0, 0, time.UTC)
	directory := t.TempDir()
	journal, err := CreateCutoverAttemptJournal(directory, validCutoverAttemptManifest(now))
	if err != nil {
		t.Fatal(err)
	}
	path := []CutoverAttemptEventType{
		CutoverEventSourceCheckpointed,
		CutoverEventFreezeApplied,
		CutoverEventFrozenConfirmed,
		CutoverEventTargetActivated,
		CutoverEventRollbackRequested,
		CutoverEventRollbackFreezeApplied,
		CutoverEventRollbackFrozenConfirmed,
		CutoverEventSourceReactivated,
		CutoverEventRollbackCheckpointed,
		CutoverEventRolledBack,
	}
	for index, eventType := range path {
		if _, err := journal.Append(eventType, strings.Repeat("a", 64), now.Add(time.Duration(index)*time.Millisecond)); err != nil {
			t.Fatalf("Append(%s): %v", eventType, err)
		}
	}
	loaded, err := LoadCutoverAttemptJournal(directory)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Projection.State != CutoverAttemptRolledBack || loaded.Projection.RollbackNeedsFreeze || len(loaded.Events) != len(path) {
		t.Fatalf("unexpected persisted rollback: %+v", loaded.Projection)
	}
	for _, name := range append([]string{cutoverAttemptManifestFilename}, cutoverAttemptEventFilename(1), cutoverAttemptEventFilename(uint64(len(path)))) {
		info, err := os.Stat(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode = %o, want 600", name, info.Mode().Perm())
		}
	}
}

func TestCutoverAttemptJournalRejectsStrictJSONAndBackwardsTime(t *testing.T) {
	now := time.Date(2026, 8, 28, 8, 0, 0, 0, time.UTC)
	directory := t.TempDir()
	journal, err := CreateCutoverAttemptJournal(directory, validCutoverAttemptManifest(now))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := journal.Append(CutoverEventSourceCheckpointed, strings.Repeat("a", 64), now.Add(-time.Millisecond)); err == nil {
		t.Fatal("event before manifest creation must fail")
	}
	payload, err := os.ReadFile(filepath.Join(directory, cutoverAttemptManifestFilename))
	if err != nil {
		t.Fatal(err)
	}
	payload = []byte(strings.Replace(string(payload), "\n}", ",\n  \"unexpected\": true\n}", 1))
	if err := os.WriteFile(filepath.Join(directory, cutoverAttemptManifestFilename), payload, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCutoverAttemptJournal(directory); err == nil {
		t.Fatal("unknown manifest field must fail closed")
	}
}

func TestCutoverAttemptManifestRequiresCanonicalDistinctAuthorities(t *testing.T) {
	now := time.Date(2026, 8, 28, 8, 0, 0, 0, time.UTC)
	manifest := validCutoverAttemptManifest(now)
	manifest.SourceAuthority = Authority("GO")
	if _, _, err := validateCutoverAttemptManifest(manifest); err == nil {
		t.Fatal("non-canonical authority must fail")
	}
	manifest.SourceAuthority = AuthorityGo
	manifest.TargetAuthority = AuthorityGo
	if _, _, err := validateCutoverAttemptManifest(manifest); err == nil {
		t.Fatal("identical source and target authorities must fail")
	}
	manifest.TargetAuthority = AuthorityCPP
	manifest.InitialEpoch = ^uint64(0) - 1
	if _, _, err := validateCutoverAttemptManifest(manifest); err == nil {
		t.Fatal("epoch without two safe transition increments must fail")
	}
}

func validCutoverAttemptManifest(now time.Time) CutoverAttemptManifest {
	return CutoverAttemptManifest{
		SchemaVersion: CutoverAttemptManifestSchemaV1,
		AttemptID:     "attempt-a", SourceAuthority: AuthorityGo, TargetAuthority: AuthorityCPP,
		InitialEpoch: 1, MaxInterruptionMS: 60000, CreatedAtUnixMS: now.UnixMilli(),
		SourceNodesManifestSHA256: strings.Repeat("1", 64),
		FrozenNodesManifestSHA256: strings.Repeat("2", 64),
		TargetNodesManifestSHA256: strings.Repeat("3", 64),
		CheckpointManifestSHA256:  strings.Repeat("4", 64),
	}
}

func validCutoverAttemptEvent(attemptID string, sequence uint64, eventType CutoverAttemptEventType) CutoverAttemptEvent {
	return CutoverAttemptEvent{
		SchemaVersion: CutoverAttemptEventSchemaV1,
		AttemptID:     attemptID, Sequence: sequence, EventType: eventType,
		PreviousEventSHA256: strings.Repeat("0", 64), ArtifactSHA256: strings.Repeat("a", 64),
		RecordedAtUnixMS: time.Date(2026, 8, 28, 8, 0, int(sequence), 0, time.UTC).UnixMilli(),
	}
}
