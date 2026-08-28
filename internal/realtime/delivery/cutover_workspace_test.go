package delivery

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCutoverAttemptWorkspaceCreatesLoadsAndReplays(t *testing.T) {
	f := newProductionExecutorFixture(t)
	directory := filepath.Join(t.TempDir(), "attempt")
	inputs := productionWorkspaceInputs(f.config)
	workspace, err := CreateCutoverAttemptWorkspace(
		directory, f.config.Manifest.AttemptID, f.config.Manifest.SourceAuthority,
		f.config.Manifest.TargetAuthority, time.Duration(f.config.Manifest.MaxInterruptionMS)*time.Millisecond,
		inputs, time.UnixMilli(f.config.Manifest.CreatedAtUnixMS),
	)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := CreateCutoverAttemptWorkspace(
		directory, f.config.Manifest.AttemptID, f.config.Manifest.SourceAuthority,
		f.config.Manifest.TargetAuthority, time.Duration(f.config.Manifest.MaxInterruptionMS)*time.Millisecond,
		inputs, time.UnixMilli(f.config.Manifest.CreatedAtUnixMS),
	)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadCutoverAttemptWorkspace(directory)
	if err != nil {
		t.Fatal(err)
	}
	if workspace.Journal.Manifest != f.config.Manifest || replayed.Journal.ManifestSHA256 != workspace.Journal.ManifestSHA256 ||
		loaded.Journal.ManifestSHA256 != workspace.Journal.ManifestSHA256 || loaded.Inputs.InitialTransition != inputs.InitialTransition {
		t.Fatalf("workspace mismatch: created=%+v replayed=%+v loaded=%+v", workspace.Journal.Manifest, replayed.Journal.Manifest, loaded.Journal.Manifest)
	}
	for _, name := range []string{cutoverAttemptInputsFilename, cutoverAttemptManifestFilename} {
		info, err := os.Stat(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode=%o, want 600", name, info.Mode().Perm())
		}
	}
}

func TestCutoverAttemptWorkspaceRejectsInputDrift(t *testing.T) {
	f := newProductionExecutorFixture(t)
	directory := filepath.Join(t.TempDir(), "attempt")
	inputs := productionWorkspaceInputs(f.config)
	if _, err := CreateCutoverAttemptWorkspace(
		directory, "workspace-a", AuthorityGo, AuthorityCPP, time.Minute, inputs, f.now,
	); err != nil {
		t.Fatal(err)
	}
	drifted := inputs
	drifted.TargetNodes.ManifestID = "different-target"
	if _, err := CreateCutoverAttemptWorkspace(
		directory, "workspace-a", AuthorityGo, AuthorityCPP, time.Minute, drifted, f.now,
	); err == nil {
		t.Fatal("workspace input drift must fail")
	}
}

func TestCutoverAttemptWorkspaceRejectsTamperedInputs(t *testing.T) {
	f := newProductionExecutorFixture(t)
	directory := filepath.Join(t.TempDir(), "attempt")
	if _, err := CreateCutoverAttemptWorkspace(
		directory, "workspace-a", AuthorityGo, AuthorityCPP, time.Minute, productionWorkspaceInputs(f.config), f.now,
	); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, cutoverAttemptInputsFilename)
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tampered := strings.Replace(string(payload), "\n}", ",\n  \"unknown\": true\n}", 1)
	if err := os.WriteFile(path, []byte(tampered), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCutoverAttemptWorkspace(directory); err == nil {
		t.Fatal("tampered workspace inputs must fail")
	}
}

func TestCutoverAttemptWorkspaceRejectsExpiredInitialLease(t *testing.T) {
	f := newProductionExecutorFixture(t)
	inputs := productionWorkspaceInputs(f.config)
	inputs.InitialTransition.LeaseUntilUnixMS = f.now.Add(-time.Second).UnixMilli()
	if _, err := CreateCutoverAttemptWorkspace(
		filepath.Join(t.TempDir(), "attempt"), "workspace-expired-a", AuthorityGo, AuthorityCPP,
		time.Minute, inputs, f.now,
	); err == nil {
		t.Fatal("expired initial lease must fail")
	}
}

func productionWorkspaceInputs(config ProductionCutoverExecutorConfig) CutoverAttemptInputs {
	return CutoverAttemptInputs{
		SchemaVersion:     CutoverAttemptInputsSchemaV1,
		InitialTransition: config.InitialTransition,
		SourceNodes:       config.SourceNodes, FrozenNodes: config.FrozenNodes, TargetNodes: config.TargetNodes,
		CheckpointManifest: config.CheckpointManifest,
	}
}
