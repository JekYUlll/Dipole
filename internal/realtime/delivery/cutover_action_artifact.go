package delivery

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	CutoverActionArtifactSchemaV1 = "dipole.realtime.cutover-action-artifact.v1"
	cutoverActionArtifactSuffix   = ".artifact.json"
	maxCutoverActionPayloadBytes  = 16 << 20
)

type CutoverActionArtifact struct {
	SchemaVersion    string                  `json:"schema_version"`
	ActionID         string                  `json:"action_id"`
	ActionSHA256     string                  `json:"action_sha256"`
	Action           CutoverAttemptAction    `json:"action"`
	AttemptID        string                  `json:"attempt_id"`
	Sequence         uint64                  `json:"sequence"`
	EventType        CutoverAttemptEventType `json:"event_type"`
	PayloadSHA256    string                  `json:"payload_sha256"`
	Payload          json.RawMessage         `json:"payload"`
	RecordedAtUnixMS int64                   `json:"recorded_at_unix_ms"`
}

type CutoverActionArtifactStore struct {
	directory string
}

func NewCutoverActionArtifactStore(directory string) (*CutoverActionArtifactStore, error) {
	directory = filepath.Clean(directory)
	if directory == "." || directory == string(filepath.Separator) {
		return nil, fmt.Errorf("cutover action artifact directory is invalid")
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create cutover action artifact directory: %w", err)
	}
	return &CutoverActionArtifactStore{directory: directory}, nil
}

func (s *CutoverActionArtifactStore) Publish(
	action CutoverAttemptAction,
	payload any,
	recordedAt time.Time,
) (CutoverActionArtifact, string, error) {
	actionSHA256, err := validateAndHashCutoverAction(action)
	if err != nil {
		return CutoverActionArtifact{}, "", err
	}
	canonicalPayload, err := json.Marshal(payload)
	if err != nil {
		return CutoverActionArtifact{}, "", fmt.Errorf("encode cutover action artifact payload: %w", err)
	}
	canonicalPayload, err = validateCutoverActionPayload(canonicalPayload)
	if err != nil {
		return CutoverActionArtifact{}, "", err
	}
	payloadSHA256 := hashBytes(canonicalPayload)
	path := s.path(action.ActionID)
	if _, err := os.Stat(path); err == nil {
		existing, digest, err := s.Load(action)
		if err != nil {
			return CutoverActionArtifact{}, "", err
		}
		if existing.PayloadSHA256 != payloadSHA256 {
			return CutoverActionArtifact{}, "", fmt.Errorf("cutover action artifact payload conflicts with existing action")
		}
		return existing, digest, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return CutoverActionArtifact{}, "", fmt.Errorf("inspect cutover action artifact: %w", err)
	}
	artifact := CutoverActionArtifact{
		SchemaVersion: CutoverActionArtifactSchemaV1,
		ActionID:      action.ActionID, ActionSHA256: actionSHA256, Action: action,
		AttemptID: action.AttemptID, Sequence: action.Sequence, EventType: action.EventType,
		PayloadSHA256: payloadSHA256, Payload: canonicalPayload, RecordedAtUnixMS: recordedAt.UTC().UnixMilli(),
	}
	digest, err := validateAndHashCutoverActionArtifact(artifact, action)
	if err != nil {
		return CutoverActionArtifact{}, "", err
	}
	if err := writeImmutableCutoverJSON(path, artifact); err != nil {
		if errors.Is(err, os.ErrExist) {
			existing, existingDigest, loadErr := s.Load(action)
			if loadErr == nil && existing.PayloadSHA256 == payloadSHA256 {
				return existing, existingDigest, nil
			}
		}
		return CutoverActionArtifact{}, "", fmt.Errorf("publish cutover action artifact: %w", err)
	}
	return artifact, digest, nil
}

func (s *CutoverActionArtifactStore) Load(action CutoverAttemptAction) (CutoverActionArtifact, string, error) {
	actionSHA256, err := validateAndHashCutoverAction(action)
	if err != nil {
		return CutoverActionArtifact{}, "", err
	}
	artifact, digest, err := s.LoadByActionID(action.ActionID)
	if err != nil {
		return CutoverActionArtifact{}, "", err
	}
	if artifact.ActionSHA256 != actionSHA256 {
		return CutoverActionArtifact{}, "", fmt.Errorf("cutover action artifact action conflicts with expected action")
	}
	return artifact, digest, nil
}

func (s *CutoverActionArtifactStore) LoadByActionID(actionID string) (CutoverActionArtifact, string, error) {
	actionID = strings.TrimSpace(actionID)
	if !fenceTransitionIDPattern.MatchString(actionID) {
		return CutoverActionArtifact{}, "", fmt.Errorf("cutover action artifact action ID is invalid")
	}
	payload, err := os.ReadFile(s.path(actionID))
	if err != nil {
		return CutoverActionArtifact{}, "", fmt.Errorf("read cutover action artifact: %w", err)
	}
	artifact, err := DecodeStrictJSON[CutoverActionArtifact](payload)
	if err != nil {
		return CutoverActionArtifact{}, "", fmt.Errorf("decode cutover action artifact: %w", err)
	}
	if artifact.ActionID != actionID {
		return CutoverActionArtifact{}, "", fmt.Errorf("cutover action artifact filename binding is invalid")
	}
	digest, err := validateAndHashCutoverActionArtifact(artifact, artifact.Action)
	if err != nil {
		return CutoverActionArtifact{}, "", err
	}
	artifact.Payload, err = validateCutoverActionPayload(artifact.Payload)
	if err != nil {
		return CutoverActionArtifact{}, "", err
	}
	return artifact, digest, nil
}

func DecodeCutoverActionArtifactPayload[T any](artifact CutoverActionArtifact) (T, error) {
	canonical, err := validateCutoverActionPayload(artifact.Payload)
	if err != nil {
		var value T
		return value, err
	}
	if hashBytes(canonical) != artifact.PayloadSHA256 {
		var value T
		return value, fmt.Errorf("cutover action artifact payload SHA-256 is invalid")
	}
	value, err := DecodeStrictJSON[T](canonical)
	if err != nil {
		return value, fmt.Errorf("decode cutover action artifact payload: %w", err)
	}
	return value, nil
}

func validateAndHashCutoverAction(action CutoverAttemptAction) (string, error) {
	if !fenceTransitionIDPattern.MatchString(action.ActionID) || !fenceTransitionIDPattern.MatchString(action.AttemptID) ||
		action.Sequence == 0 || action.ExpectedEpoch == 0 || action.MaxInterruptionMS <= 0 ||
		action.MaxInterruptionMS > int64((10*time.Minute)/time.Millisecond) {
		return "", fmt.Errorf("cutover attempt action identity or bounds are invalid")
	}
	if action.LeaseTransitionActionID != "" && !fenceTransitionIDPattern.MatchString(action.LeaseTransitionActionID) {
		return "", fmt.Errorf("cutover attempt lease transition action ID is invalid")
	}
	if !validCutoverAttemptEventType(action.EventType) {
		return "", fmt.Errorf("cutover attempt action event type is invalid")
	}
	expectedActionID, err := cutoverAttemptActionID(action.AttemptID, action.Sequence, action.EventType)
	if err != nil || action.ActionID != expectedActionID {
		return "", fmt.Errorf("cutover attempt action ID is not deterministic")
	}
	if !validCutoverAttemptState(action.CurrentState) || action.SourceAuthority == action.TargetAuthority {
		return "", fmt.Errorf("cutover attempt action state or authorities are invalid")
	}
	for _, authority := range []Authority{action.SourceAuthority, action.TargetAuthority} {
		parsed, err := ParseAuthority(string(authority))
		if err != nil || parsed != authority {
			return "", fmt.Errorf("cutover attempt action authority is invalid")
		}
	}
	payload, err := json.Marshal(action)
	if err != nil {
		return "", fmt.Errorf("encode cutover attempt action: %w", err)
	}
	return hashBytes(payload), nil
}

func validCutoverAttemptState(state CutoverAttemptState) bool {
	switch state {
	case CutoverAttemptCreated, CutoverAttemptSourceCheckpointed, CutoverAttemptFreezeApplied,
		CutoverAttemptFrozenConfirmed, CutoverAttemptTargetActivated, CutoverAttemptTargetCheckpointed,
		CutoverAttemptCompleted, CutoverAttemptRollbackRequested, CutoverAttemptRollbackFreezeApplied,
		CutoverAttemptRollbackFrozenConfirmed, CutoverAttemptSourceReactivated,
		CutoverAttemptRollbackCheckpointed, CutoverAttemptRolledBack:
		return true
	default:
		return false
	}
}

func validateAndHashCutoverActionArtifact(artifact CutoverActionArtifact, action CutoverAttemptAction) (string, error) {
	actionSHA256, err := validateAndHashCutoverAction(action)
	if err != nil {
		return "", err
	}
	canonicalPayload, err := validateCutoverActionPayload(artifact.Payload)
	if err != nil {
		return "", err
	}
	if artifact.SchemaVersion != CutoverActionArtifactSchemaV1 || artifact.ActionID != action.ActionID ||
		artifact.ActionSHA256 != actionSHA256 || artifact.Action != action ||
		artifact.AttemptID != action.AttemptID || artifact.Sequence != action.Sequence ||
		artifact.EventType != action.EventType || !validSHA256(artifact.PayloadSHA256) ||
		artifact.PayloadSHA256 != hashBytes(canonicalPayload) || artifact.RecordedAtUnixMS <= 0 {
		return "", fmt.Errorf("cutover action artifact binding is invalid")
	}
	artifact.Payload = canonicalPayload
	payload, err := json.Marshal(artifact)
	if err != nil {
		return "", fmt.Errorf("encode cutover action artifact: %w", err)
	}
	return hashBytes(payload), nil
}

func validateCutoverActionPayload(payload []byte) ([]byte, error) {
	if len(payload) == 0 || len(payload) > maxCutoverActionPayloadBytes {
		return nil, fmt.Errorf("cutover action artifact payload size is invalid")
	}
	if err := rejectDuplicateJSONFields(payload); err != nil {
		return nil, fmt.Errorf("cutover action artifact payload is invalid: %w", err)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, payload); err != nil {
		return nil, fmt.Errorf("cutover action artifact payload is invalid: %w", err)
	}
	canonical := compact.Bytes()
	if len(canonical) < 2 || canonical[0] != '{' || canonical[len(canonical)-1] != '}' {
		return nil, fmt.Errorf("cutover action artifact payload must be a JSON object")
	}
	return append([]byte(nil), canonical...), nil
}

func (s *CutoverActionArtifactStore) path(actionID string) string {
	return filepath.Join(s.directory, strings.TrimSpace(actionID)+cutoverActionArtifactSuffix)
}

func validCutoverAttemptEventType(eventType CutoverAttemptEventType) bool {
	switch eventType {
	case CutoverEventSourceCheckpointed, CutoverEventFreezeApplied, CutoverEventFrozenConfirmed,
		CutoverEventTargetActivated, CutoverEventTargetCheckpointed, CutoverEventCompleted,
		CutoverEventRollbackRequested, CutoverEventRollbackFreezeApplied, CutoverEventRollbackFrozenConfirmed,
		CutoverEventSourceReactivated, CutoverEventRollbackCheckpointed, CutoverEventRolledBack,
		CutoverEventLeaseRenewed:
		return true
	default:
		return false
	}
}
