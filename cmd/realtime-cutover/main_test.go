package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	realtimeDelivery "github.com/JekYUlll/Dipole/internal/realtime/delivery"
)

type commandExecutor struct {
	workspace *realtimeDelivery.CutoverAttemptWorkspace
	now       time.Time
}

func (e commandExecutor) Execute(_ context.Context, action realtimeDelivery.CutoverAttemptAction) (realtimeDelivery.CutoverAttemptActionResult, error) {
	payload := struct {
		Status string `json:"status"`
	}{Status: "captured"}
	artifact, digest, err := e.workspace.Artifacts.Publish(action, payload, e.now)
	if err != nil {
		return realtimeDelivery.CutoverAttemptActionResult{}, err
	}
	return realtimeDelivery.CutoverAttemptActionResult{ArtifactSHA256: digest, RecordedAtUnixMS: artifact.RecordedAtUnixMS}, nil
}

func TestRunCreatesAndReadsCutoverWorkspace(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	inputsPath := filepath.Join(t.TempDir(), "inputs.json")
	inputs := commandInputs(now)
	payload, err := json.Marshal(inputs)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inputsPath, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(t.TempDir(), "attempt")
	var output bytes.Buffer
	if err := run(context.Background(), []string{
		"-operation", "create", "-attempt-dir", directory, "-inputs", inputsPath,
		"-attempt-id", "cli-a", "-source", "go", "-target", "cpp", "-confirm",
	}, &output, func() time.Time { return now }, nil); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	if err := run(context.Background(), []string{
		"-operation", "status", "-attempt-dir", directory,
	}, &output, func() time.Time { return now }, nil); err != nil {
		t.Fatal(err)
	}
	var status statusOutput
	if err := json.Unmarshal(output.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.AttemptID != "cli-a" || status.State != realtimeDelivery.CutoverAttemptCreated || status.LastSequence != 0 {
		t.Fatalf("status=%+v", status)
	}
}

func TestRunRequiresConfirmationBeforeMutation(t *testing.T) {
	if err := run(context.Background(), []string{
		"-operation", "create", "-attempt-dir", t.TempDir(),
	}, &bytes.Buffer{}, time.Now, nil); err == nil {
		t.Fatal("create without confirmation must fail")
	}
}

func TestRunAdvancesOneDurableStep(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	inputsPath := filepath.Join(t.TempDir(), "inputs.json")
	payload, err := json.Marshal(commandInputs(now))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inputsPath, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(t.TempDir(), "attempt")
	if err := run(context.Background(), []string{
		"-operation", "create", "-attempt-dir", directory, "-inputs", inputsPath,
		"-attempt-id", "cli-advance-a", "-source", "go", "-target", "cpp", "-confirm",
	}, &bytes.Buffer{}, func() time.Time { return now }, nil); err != nil {
		t.Fatal(err)
	}
	factory := func(workspace *realtimeDelivery.CutoverAttemptWorkspace, _ string, _ time.Duration) (realtimeDelivery.CutoverAttemptActionExecutor, func(), error) {
		return commandExecutor{workspace: workspace, now: now.Add(time.Second)}, func() {}, nil
	}
	var output bytes.Buffer
	if err := run(context.Background(), []string{
		"-operation", "advance", "-attempt-dir", directory, "-operator", "operator-a", "-confirm",
	}, &output, func() time.Time { return now.Add(time.Second) }, factory); err != nil {
		t.Fatal(err)
	}
	var result realtimeDelivery.CutoverAttemptAdvance
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.State != realtimeDelivery.CutoverAttemptSourceCheckpointed || result.Sequence != 1 {
		t.Fatalf("advance result=%+v", result)
	}
	workspace, err := realtimeDelivery.LoadCutoverAttemptWorkspace(directory)
	if err != nil {
		t.Fatal(err)
	}
	if workspace.Journal.Projection.State != realtimeDelivery.CutoverAttemptSourceCheckpointed {
		t.Fatalf("persisted projection=%+v", workspace.Journal.Projection)
	}
}

func TestRunRenewsOneDurableLeaseStep(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	directory := createCommandWorkspace(t, now, "cli-renew-a")
	factory := func(workspace *realtimeDelivery.CutoverAttemptWorkspace, _ string, _ time.Duration) (realtimeDelivery.CutoverAttemptActionExecutor, func(), error) {
		return commandExecutor{workspace: workspace, now: now.Add(time.Second)}, func() {}, nil
	}
	var output bytes.Buffer
	if err := run(context.Background(), []string{
		"-operation", "renew", "-attempt-dir", directory, "-operator", "operator-a", "-confirm",
	}, &output, func() time.Time { return now.Add(time.Second) }, factory); err != nil {
		t.Fatal(err)
	}
	var result realtimeDelivery.CutoverAttemptAdvance
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.State != realtimeDelivery.CutoverAttemptCreated || result.EventType != realtimeDelivery.CutoverEventLeaseRenewed || result.Sequence != 1 {
		t.Fatalf("renew result=%+v", result)
	}
}

func createCommandWorkspace(t *testing.T, now time.Time, attemptID string) string {
	t.Helper()
	inputsPath := filepath.Join(t.TempDir(), "inputs.json")
	payload, err := json.Marshal(commandInputs(now))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inputsPath, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(t.TempDir(), "attempt")
	if err := run(context.Background(), []string{
		"-operation", "create", "-attempt-dir", directory, "-inputs", inputsPath,
		"-attempt-id", attemptID, "-source", "go", "-target", "cpp", "-confirm",
	}, &bytes.Buffer{}, func() time.Time { return now }, nil); err != nil {
		t.Fatal(err)
	}
	return directory
}

func commandInputs(now time.Time) realtimeDelivery.CutoverAttemptInputs {
	transition := realtimeDelivery.FenceTransitionReceipt{
		SchemaVersion: realtimeDelivery.FenceTransitionReceiptSchemaV1,
		TransitionID:  "initial-a", RequestSHA256: repeat("1"), Action: realtimeDelivery.FenceTransitionBootstrap,
		OperatorID: "operator-a", ReasonSHA256: repeat("2"), NextSHA256: repeat("3"),
		Authority: realtimeDelivery.AuthorityGo, Phase: realtimeDelivery.FencePhaseActive, Epoch: 1,
		LeaseUntilUnixMS: now.Add(10 * time.Minute).UnixMilli(), AppliedAtUnixMS: now.Add(-time.Second).UnixMilli(),
	}
	nodes := func(id string, authority realtimeDelivery.Authority) realtimeDelivery.FenceExpectedNodeManifest {
		return realtimeDelivery.FenceExpectedNodeManifest{
			SchemaVersion: realtimeDelivery.FenceExpectedNodeManifestSchemaV1, ManifestID: id,
			Nodes: []realtimeDelivery.FenceExpectedNode{{Component: "gateway", ObserverID: "gateway-a", ExpectedAuthority: authority}},
		}
	}
	return realtimeDelivery.CutoverAttemptInputs{
		SchemaVersion: realtimeDelivery.CutoverAttemptInputsSchemaV1, InitialTransition: transition,
		SourceNodes: nodes("source-a", realtimeDelivery.AuthorityGo),
		FrozenNodes: nodes("frozen-a", realtimeDelivery.AuthorityCPP),
		TargetNodes: nodes("target-a", realtimeDelivery.AuthorityCPP),
		CheckpointManifest: realtimeDelivery.DualGroupCheckpointManifest{
			SchemaVersion: realtimeDelivery.DualGroupCheckpointManifestSchemaV1, ManifestID: "checkpoint-a",
			Topics: []string{"dipole.message.direct.created"},
			Groups: []realtimeDelivery.KafkaCheckpointGroupSpec{
				{Role: realtimeDelivery.KafkaCheckpointRoleCompatibility, GroupID: "gateway-a"},
				{Role: realtimeDelivery.KafkaCheckpointRolePrimary, GroupID: "primary-a"},
			},
		},
	}
}

func repeat(value string) string {
	result := ""
	for range 64 {
		result += value
	}
	return result
}
