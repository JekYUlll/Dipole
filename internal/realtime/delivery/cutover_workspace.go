package delivery

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	CutoverAttemptInputsSchemaV1 = "dipole.realtime.cutover-attempt-inputs.v1"
	cutoverAttemptInputsFilename = "inputs.json"
	cutoverAttemptArtifactsDir   = "artifacts"
)

type CutoverAttemptInputs struct {
	SchemaVersion      string                      `json:"schema_version"`
	InitialTransition  FenceTransitionReceipt      `json:"initial_transition"`
	SourceNodes        FenceExpectedNodeManifest   `json:"source_nodes"`
	FrozenNodes        FenceExpectedNodeManifest   `json:"frozen_nodes"`
	TargetNodes        FenceExpectedNodeManifest   `json:"target_nodes"`
	CheckpointManifest DualGroupCheckpointManifest `json:"checkpoint_manifest"`
}

type CutoverAttemptWorkspace struct {
	Directory string
	Inputs    CutoverAttemptInputs
	Journal   *CutoverAttemptJournal
	Artifacts *CutoverActionArtifactStore
}

func CreateCutoverAttemptWorkspace(
	directory, attemptID string,
	sourceAuthority, targetAuthority Authority,
	maxInterruption time.Duration,
	inputs CutoverAttemptInputs,
	createdAt time.Time,
) (*CutoverAttemptWorkspace, error) {
	inputs, manifest, err := buildCutoverAttemptManifest(
		attemptID, sourceAuthority, targetAuthority, maxInterruption, inputs, createdAt,
	)
	if err != nil {
		return nil, err
	}
	directory = filepath.Clean(directory)
	if directory == "." || directory == string(filepath.Separator) {
		return nil, fmt.Errorf("cutover attempt workspace directory is invalid")
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create cutover attempt workspace directory: %w", err)
	}
	inputsPath := filepath.Join(directory, cutoverAttemptInputsFilename)
	if err := publishOrMatchCutoverInputs(inputsPath, inputs); err != nil {
		return nil, err
	}
	journal, err := CreateCutoverAttemptJournal(directory, manifest)
	if err != nil {
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		journal, err = LoadCutoverAttemptJournal(directory)
		if err != nil {
			return nil, err
		}
		_, expectedSHA, hashErr := validateCutoverAttemptManifest(manifest)
		if hashErr != nil || journal.ManifestSHA256 != expectedSHA {
			return nil, fmt.Errorf("cutover attempt workspace manifest conflicts with existing attempt")
		}
	}
	artifacts, err := NewCutoverActionArtifactStore(filepath.Join(directory, cutoverAttemptArtifactsDir))
	if err != nil {
		return nil, err
	}
	return &CutoverAttemptWorkspace{Directory: directory, Inputs: inputs, Journal: journal, Artifacts: artifacts}, nil
}

func LoadCutoverAttemptWorkspace(directory string) (*CutoverAttemptWorkspace, error) {
	directory = filepath.Clean(directory)
	payload, err := os.ReadFile(filepath.Join(directory, cutoverAttemptInputsFilename))
	if err != nil {
		return nil, fmt.Errorf("read cutover attempt workspace inputs: %w", err)
	}
	inputs, err := DecodeStrictJSON[CutoverAttemptInputs](payload)
	if err != nil {
		return nil, fmt.Errorf("decode cutover attempt workspace inputs: %w", err)
	}
	journal, err := LoadCutoverAttemptJournal(directory)
	if err != nil {
		return nil, err
	}
	canonicalInputs, expectedManifest, err := buildCutoverAttemptManifest(
		journal.Manifest.AttemptID, journal.Manifest.SourceAuthority, journal.Manifest.TargetAuthority,
		time.Duration(journal.Manifest.MaxInterruptionMS)*time.Millisecond, inputs,
		time.UnixMilli(journal.Manifest.CreatedAtUnixMS),
	)
	if err != nil {
		return nil, err
	}
	_, expectedSHA, err := validateCutoverAttemptManifest(expectedManifest)
	if err != nil || expectedSHA != journal.ManifestSHA256 {
		return nil, fmt.Errorf("cutover attempt workspace inputs do not match manifest")
	}
	artifacts, err := NewCutoverActionArtifactStore(filepath.Join(directory, cutoverAttemptArtifactsDir))
	if err != nil {
		return nil, err
	}
	return &CutoverAttemptWorkspace{Directory: directory, Inputs: canonicalInputs, Journal: journal, Artifacts: artifacts}, nil
}

func buildCutoverAttemptManifest(
	attemptID string,
	sourceAuthority, targetAuthority Authority,
	maxInterruption time.Duration,
	inputs CutoverAttemptInputs,
	createdAt time.Time,
) (CutoverAttemptInputs, CutoverAttemptManifest, error) {
	inputs, sourceSHA, frozenSHA, targetSHA, checkpointSHA, err := canonicalCutoverAttemptInputs(inputs)
	if err != nil {
		return inputs, CutoverAttemptManifest{}, err
	}
	manifest := CutoverAttemptManifest{
		SchemaVersion: CutoverAttemptManifestSchemaV1, AttemptID: strings.TrimSpace(attemptID),
		SourceAuthority: sourceAuthority, TargetAuthority: targetAuthority,
		InitialEpoch: inputs.InitialTransition.Epoch, InitialLeaseSHA256: inputs.InitialTransition.NextSHA256,
		MaxInterruptionMS: maxInterruption.Milliseconds(), CreatedAtUnixMS: createdAt.UTC().UnixMilli(),
		SourceNodesManifestSHA256: sourceSHA, FrozenNodesManifestSHA256: frozenSHA,
		TargetNodesManifestSHA256: targetSHA, CheckpointManifestSHA256: checkpointSHA,
	}
	manifest, _, err = validateCutoverAttemptManifest(manifest)
	if err != nil {
		return inputs, CutoverAttemptManifest{}, err
	}
	if inputs.InitialTransition.Authority != manifest.SourceAuthority || inputs.InitialTransition.Phase != FencePhaseActive {
		return inputs, CutoverAttemptManifest{}, fmt.Errorf("cutover attempt workspace initial transition is not active source authority")
	}
	return inputs, manifest, nil
}

func canonicalCutoverAttemptInputs(inputs CutoverAttemptInputs) (CutoverAttemptInputs, string, string, string, string, error) {
	if inputs.SchemaVersion != CutoverAttemptInputsSchemaV1 {
		return inputs, "", "", "", "", fmt.Errorf("cutover attempt workspace input schema is invalid")
	}
	if err := validateAggregateTransitionReceipt(inputs.InitialTransition); err != nil {
		return inputs, "", "", "", "", err
	}
	canonicalizeNodes := func(manifest FenceExpectedNodeManifest) (FenceExpectedNodeManifest, string, error) {
		nodes, digest, err := validateExpectedNodeManifest(manifest)
		if err != nil {
			return manifest, "", err
		}
		return FenceExpectedNodeManifest{SchemaVersion: manifest.SchemaVersion, ManifestID: strings.TrimSpace(manifest.ManifestID), Nodes: nodes}, digest, nil
	}
	var err error
	var sourceSHA, frozenSHA, targetSHA, checkpointSHA string
	inputs.SourceNodes, sourceSHA, err = canonicalizeNodes(inputs.SourceNodes)
	if err != nil {
		return inputs, "", "", "", "", err
	}
	inputs.FrozenNodes, frozenSHA, err = canonicalizeNodes(inputs.FrozenNodes)
	if err != nil {
		return inputs, "", "", "", "", err
	}
	inputs.TargetNodes, targetSHA, err = canonicalizeNodes(inputs.TargetNodes)
	if err != nil {
		return inputs, "", "", "", "", err
	}
	inputs.CheckpointManifest, checkpointSHA, err = validateDualGroupCheckpointManifest(inputs.CheckpointManifest)
	if err != nil {
		return inputs, "", "", "", "", err
	}
	return inputs, sourceSHA, frozenSHA, targetSHA, checkpointSHA, nil
}

func publishOrMatchCutoverInputs(path string, inputs CutoverAttemptInputs) error {
	payload, err := json.Marshal(inputs)
	if err != nil {
		return fmt.Errorf("encode cutover attempt workspace inputs: %w", err)
	}
	expectedSHA := hashBytes(payload)
	if err := writeImmutableCutoverJSON(path, inputs); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("publish cutover attempt workspace inputs: %w", err)
	}
	existingPayload, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read existing cutover attempt workspace inputs: %w", err)
	}
	existing, err := DecodeStrictJSON[CutoverAttemptInputs](existingPayload)
	if err != nil {
		return fmt.Errorf("decode existing cutover attempt workspace inputs: %w", err)
	}
	existing, _, _, _, _, err = canonicalCutoverAttemptInputs(existing)
	if err != nil {
		return err
	}
	existingCanonical, err := json.Marshal(existing)
	if err != nil || hashBytes(existingCanonical) != expectedSHA {
		return fmt.Errorf("cutover attempt workspace inputs conflict with existing inputs")
	}
	return nil
}
