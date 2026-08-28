package delivery

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewCheckpointBundleAndWriteImmutable(t *testing.T) {
	now := time.Date(2026, 8, 28, 6, 0, 0, 0, time.UTC)
	proof := validObservationAggregateProof(now)
	proofPayload, err := json.Marshal(proof)
	if err != nil {
		t.Fatal(err)
	}
	receipt := DualGroupCheckpointReceipt{
		SchemaVersion: DualGroupCheckpointReceiptSchemaV1, Decision: DualGroupCheckpointEligible,
		ObservationAggregateSHA256: hashBytes(proofPayload), TransitionID: proof.TransitionID,
		LeaseSHA256: proof.LeaseSHA256, Authority: proof.Authority, Phase: proof.Phase, Epoch: proof.Epoch,
	}
	bundle, err := NewCheckpointBundle(proof, receipt)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "checkpoint.json")
	if err := WriteImmutableCheckpointBundle(path, bundle); err != nil {
		t.Fatalf("WriteImmutableCheckpointBundle(): %v", err)
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var decoded CheckpointBundle
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SchemaVersion != CheckpointBundleSchemaV1 || decoded.Checkpoint.LeaseSHA256 != proof.LeaseSHA256 {
		t.Fatalf("unexpected bundle: %+v", decoded)
	}
	if err := WriteImmutableCheckpointBundle(path, bundle); err == nil || !strings.Contains(err.Error(), "exists") {
		t.Fatalf("second immutable write error = %v", err)
	}
}

func TestNewCheckpointBundleRejectsProofHashDrift(t *testing.T) {
	now := time.Date(2026, 8, 28, 6, 0, 0, 0, time.UTC)
	proof := validObservationAggregateProof(now)
	receipt := DualGroupCheckpointReceipt{
		SchemaVersion: DualGroupCheckpointReceiptSchemaV1, Decision: DualGroupCheckpointEligible,
		ObservationAggregateSHA256: strings.Repeat("a", 64), TransitionID: proof.TransitionID,
		LeaseSHA256: proof.LeaseSHA256, Authority: proof.Authority, Phase: proof.Phase, Epoch: proof.Epoch,
	}
	if _, err := NewCheckpointBundle(proof, receipt); err == nil {
		t.Fatal("proof hash drift must fail")
	}
}
