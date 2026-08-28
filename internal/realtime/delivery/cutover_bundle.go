package delivery

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const CheckpointBundleSchemaV1 = "dipole.realtime.delivery-checkpoint-bundle.v1"

type CheckpointBundle struct {
	SchemaVersion        string                           `json:"schema_version"`
	ObservationAggregate FenceObservationAggregateReceipt `json:"observation_aggregate"`
	Checkpoint           DualGroupCheckpointReceipt       `json:"checkpoint"`
}

func NewCheckpointBundle(proof FenceObservationAggregateReceipt, checkpoint DualGroupCheckpointReceipt) (CheckpointBundle, error) {
	payload, err := json.Marshal(proof)
	if err != nil {
		return CheckpointBundle{}, fmt.Errorf("encode delivery checkpoint observation aggregate: %w", err)
	}
	if checkpoint.SchemaVersion != DualGroupCheckpointReceiptSchemaV1 || checkpoint.Decision != DualGroupCheckpointEligible ||
		checkpoint.ObservationAggregateSHA256 != hashBytes(payload) || checkpoint.TransitionID != proof.TransitionID ||
		checkpoint.LeaseSHA256 != proof.LeaseSHA256 || checkpoint.Authority != proof.Authority ||
		checkpoint.Phase != proof.Phase || checkpoint.Epoch != proof.Epoch {
		return CheckpointBundle{}, fmt.Errorf("delivery checkpoint does not match observation aggregate")
	}
	return CheckpointBundle{SchemaVersion: CheckpointBundleSchemaV1, ObservationAggregate: proof, Checkpoint: checkpoint}, nil
}

func WriteImmutableCheckpointBundle(path string, bundle CheckpointBundle) error {
	path = filepath.Clean(path)
	if path == "." || filepath.Base(path) == "." {
		return fmt.Errorf("delivery checkpoint output path is invalid")
	}
	payload, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return fmt.Errorf("encode delivery checkpoint bundle: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".dipole-delivery-checkpoint-*.json")
	if err != nil {
		return fmt.Errorf("create delivery checkpoint temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err == nil {
		_, err = temporary.Write(append(payload, '\n'))
	}
	if err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("write delivery checkpoint temporary file: %w", err)
	}
	if err := os.Link(temporaryPath, path); err != nil {
		return fmt.Errorf("publish immutable delivery checkpoint bundle: %w", err)
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("open delivery checkpoint directory: %w", err)
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil {
		return fmt.Errorf("sync delivery checkpoint directory: %w", syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close delivery checkpoint directory: %w", closeErr)
	}
	return nil
}
