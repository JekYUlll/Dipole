package delivery

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	CutoverAttemptManifestSchemaV1 = "dipole.realtime.cutover-attempt-manifest.v1"
	CutoverAttemptEventSchemaV1    = "dipole.realtime.cutover-attempt-event.v1"

	cutoverAttemptManifestFilename = "attempt.json"
	cutoverAttemptEventSuffix      = ".event.json"
)

type CutoverAttemptEventType string

const (
	CutoverEventSourceCheckpointed      CutoverAttemptEventType = "source_checkpointed"
	CutoverEventFreezeApplied           CutoverAttemptEventType = "freeze_applied"
	CutoverEventFrozenConfirmed         CutoverAttemptEventType = "frozen_confirmed"
	CutoverEventTargetActivated         CutoverAttemptEventType = "target_activated"
	CutoverEventTargetCheckpointed      CutoverAttemptEventType = "target_checkpointed"
	CutoverEventCompleted               CutoverAttemptEventType = "completed"
	CutoverEventRollbackRequested       CutoverAttemptEventType = "rollback_requested"
	CutoverEventRollbackFreezeApplied   CutoverAttemptEventType = "rollback_freeze_applied"
	CutoverEventRollbackFrozenConfirmed CutoverAttemptEventType = "rollback_frozen_confirmed"
	CutoverEventSourceReactivated       CutoverAttemptEventType = "source_reactivated"
	CutoverEventRollbackCheckpointed    CutoverAttemptEventType = "rollback_checkpointed"
	CutoverEventRolledBack              CutoverAttemptEventType = "rolled_back"
	CutoverEventLeaseRenewed            CutoverAttemptEventType = "lease_renewed"
)

type CutoverAttemptState string

const (
	CutoverAttemptCreated                 CutoverAttemptState = "created"
	CutoverAttemptSourceCheckpointed      CutoverAttemptState = "source_checkpointed"
	CutoverAttemptFreezeApplied           CutoverAttemptState = "freeze_applied"
	CutoverAttemptFrozenConfirmed         CutoverAttemptState = "frozen_confirmed"
	CutoverAttemptTargetActivated         CutoverAttemptState = "target_activated"
	CutoverAttemptTargetCheckpointed      CutoverAttemptState = "target_checkpointed"
	CutoverAttemptCompleted               CutoverAttemptState = "completed"
	CutoverAttemptRollbackRequested       CutoverAttemptState = "rollback_requested"
	CutoverAttemptRollbackFreezeApplied   CutoverAttemptState = "rollback_freeze_applied"
	CutoverAttemptRollbackFrozenConfirmed CutoverAttemptState = "rollback_frozen_confirmed"
	CutoverAttemptSourceReactivated       CutoverAttemptState = "source_reactivated"
	CutoverAttemptRollbackCheckpointed    CutoverAttemptState = "rollback_checkpointed"
	CutoverAttemptRolledBack              CutoverAttemptState = "rolled_back"
)

type CutoverAttemptManifest struct {
	SchemaVersion             string    `json:"schema_version"`
	AttemptID                 string    `json:"attempt_id"`
	SourceAuthority           Authority `json:"source_authority"`
	TargetAuthority           Authority `json:"target_authority"`
	InitialEpoch              uint64    `json:"initial_epoch"`
	InitialLeaseSHA256        string    `json:"initial_lease_sha256"`
	MaxInterruptionMS         int64     `json:"max_interruption_ms"`
	CreatedAtUnixMS           int64     `json:"created_at_unix_ms"`
	SourceNodesManifestSHA256 string    `json:"source_nodes_manifest_sha256"`
	FrozenNodesManifestSHA256 string    `json:"frozen_nodes_manifest_sha256"`
	TargetNodesManifestSHA256 string    `json:"target_nodes_manifest_sha256"`
	CheckpointManifestSHA256  string    `json:"checkpoint_manifest_sha256"`
}

type CutoverAttemptEvent struct {
	SchemaVersion       string                  `json:"schema_version"`
	AttemptID           string                  `json:"attempt_id"`
	Sequence            uint64                  `json:"sequence"`
	EventType           CutoverAttemptEventType `json:"event_type"`
	PreviousEventSHA256 string                  `json:"previous_event_sha256"`
	ArtifactSHA256      string                  `json:"artifact_sha256"`
	RecordedAtUnixMS    int64                   `json:"recorded_at_unix_ms"`
}

type CutoverAttemptProjection struct {
	AttemptID            string
	State                CutoverAttemptState
	LastSequence         uint64
	LastRecordedAtUnixMS int64
	RollbackNeedsFreeze  bool
}

func newCutoverAttemptProjection(attemptID string) CutoverAttemptProjection {
	return CutoverAttemptProjection{AttemptID: attemptID, State: CutoverAttemptCreated}
}

func (p *CutoverAttemptProjection) Apply(event CutoverAttemptEvent) error {
	if err := validateCutoverAttemptEvent(event); err != nil {
		return err
	}
	if event.AttemptID != p.AttemptID {
		return fmt.Errorf("cutover attempt event belongs to %q, expected %q", event.AttemptID, p.AttemptID)
	}
	if event.Sequence != p.LastSequence+1 {
		return fmt.Errorf("cutover attempt event sequence is %d, expected %d", event.Sequence, p.LastSequence+1)
	}
	if event.RecordedAtUnixMS < p.LastRecordedAtUnixMS {
		return fmt.Errorf("cutover attempt event timestamp moved backwards")
	}

	next, rollbackNeedsFreeze, err := p.transition(event.EventType)
	if err != nil {
		return err
	}
	p.State = next
	p.LastSequence = event.Sequence
	p.LastRecordedAtUnixMS = event.RecordedAtUnixMS
	p.RollbackNeedsFreeze = rollbackNeedsFreeze
	return nil
}

func (p CutoverAttemptProjection) transition(eventType CutoverAttemptEventType) (CutoverAttemptState, bool, error) {
	if eventType == CutoverEventLeaseRenewed {
		switch p.State {
		case CutoverAttemptCreated, CutoverAttemptFreezeApplied, CutoverAttemptTargetActivated,
			CutoverAttemptRollbackRequested, CutoverAttemptRollbackFreezeApplied, CutoverAttemptSourceReactivated:
			return p.State, p.RollbackNeedsFreeze, nil
		case CutoverAttemptSourceCheckpointed:
			return CutoverAttemptCreated, p.RollbackNeedsFreeze, nil
		case CutoverAttemptFrozenConfirmed:
			return CutoverAttemptFreezeApplied, p.RollbackNeedsFreeze, nil
		case CutoverAttemptTargetCheckpointed:
			return CutoverAttemptTargetActivated, p.RollbackNeedsFreeze, nil
		case CutoverAttemptRollbackFrozenConfirmed:
			return CutoverAttemptRollbackFreezeApplied, p.RollbackNeedsFreeze, nil
		}
		return "", p.RollbackNeedsFreeze, fmt.Errorf("cutover attempt lease cannot renew in state %q", p.State)
	}
	switch p.State {
	case CutoverAttemptCreated:
		if eventType == CutoverEventSourceCheckpointed {
			return CutoverAttemptSourceCheckpointed, false, nil
		}
	case CutoverAttemptSourceCheckpointed:
		if eventType == CutoverEventFreezeApplied {
			return CutoverAttemptFreezeApplied, false, nil
		}
	case CutoverAttemptFreezeApplied:
		switch eventType {
		case CutoverEventFrozenConfirmed:
			return CutoverAttemptFrozenConfirmed, false, nil
		case CutoverEventRollbackRequested:
			return CutoverAttemptRollbackRequested, false, nil
		}
	case CutoverAttemptFrozenConfirmed:
		switch eventType {
		case CutoverEventTargetActivated:
			return CutoverAttemptTargetActivated, false, nil
		case CutoverEventRollbackRequested:
			return CutoverAttemptRollbackRequested, false, nil
		}
	case CutoverAttemptTargetActivated, CutoverAttemptTargetCheckpointed, CutoverAttemptCompleted:
		if eventType == CutoverEventRollbackRequested {
			return CutoverAttemptRollbackRequested, true, nil
		}
		if p.State == CutoverAttemptTargetActivated && eventType == CutoverEventTargetCheckpointed {
			return CutoverAttemptTargetCheckpointed, false, nil
		}
		if p.State == CutoverAttemptTargetCheckpointed && eventType == CutoverEventCompleted {
			return CutoverAttemptCompleted, false, nil
		}
	case CutoverAttemptRollbackRequested:
		if p.RollbackNeedsFreeze && eventType == CutoverEventRollbackFreezeApplied {
			return CutoverAttemptRollbackFreezeApplied, true, nil
		}
		if !p.RollbackNeedsFreeze && eventType == CutoverEventRollbackFrozenConfirmed {
			return CutoverAttemptRollbackFrozenConfirmed, false, nil
		}
	case CutoverAttemptRollbackFreezeApplied:
		if eventType == CutoverEventRollbackFrozenConfirmed {
			return CutoverAttemptRollbackFrozenConfirmed, true, nil
		}
	case CutoverAttemptRollbackFrozenConfirmed:
		if eventType == CutoverEventSourceReactivated {
			return CutoverAttemptSourceReactivated, false, nil
		}
	case CutoverAttemptSourceReactivated:
		if eventType == CutoverEventRollbackCheckpointed {
			return CutoverAttemptRollbackCheckpointed, false, nil
		}
	case CutoverAttemptRollbackCheckpointed:
		if eventType == CutoverEventRolledBack {
			return CutoverAttemptRolledBack, false, nil
		}
	}
	return "", p.RollbackNeedsFreeze, fmt.Errorf("cutover attempt event %q is invalid in state %q", eventType, p.State)
}

type CutoverAttemptJournal struct {
	Directory      string
	Manifest       CutoverAttemptManifest
	ManifestSHA256 string
	Events         []CutoverAttemptEvent
	Projection     CutoverAttemptProjection
	HeadSHA256     string
}

func CreateCutoverAttemptJournal(directory string, manifest CutoverAttemptManifest) (*CutoverAttemptJournal, error) {
	canonical, manifestSHA256, err := validateCutoverAttemptManifest(manifest)
	if err != nil {
		return nil, err
	}
	directory = filepath.Clean(directory)
	if directory == "." || directory == string(filepath.Separator) {
		return nil, fmt.Errorf("cutover attempt journal directory is invalid")
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create cutover attempt journal directory: %w", err)
	}
	if err := writeImmutableCutoverJSON(filepath.Join(directory, cutoverAttemptManifestFilename), canonical); err != nil {
		return nil, fmt.Errorf("publish cutover attempt manifest: %w", err)
	}
	return &CutoverAttemptJournal{
		Directory: directory, Manifest: canonical, ManifestSHA256: manifestSHA256,
		Projection: newCutoverAttemptProjection(canonical.AttemptID), HeadSHA256: manifestSHA256,
	}, nil
}

func LoadCutoverAttemptJournal(directory string) (*CutoverAttemptJournal, error) {
	directory = filepath.Clean(directory)
	payload, err := os.ReadFile(filepath.Join(directory, cutoverAttemptManifestFilename))
	if err != nil {
		return nil, fmt.Errorf("read cutover attempt manifest: %w", err)
	}
	manifest, err := DecodeStrictJSON[CutoverAttemptManifest](payload)
	if err != nil {
		return nil, fmt.Errorf("decode cutover attempt manifest: %w", err)
	}
	manifest, manifestSHA256, err := validateCutoverAttemptManifest(manifest)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("read cutover attempt journal directory: %w", err)
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), cutoverAttemptEventSuffix) {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	events := make([]CutoverAttemptEvent, 0, len(names))
	for index, name := range names {
		expected := cutoverAttemptEventFilename(uint64(index + 1))
		if name != expected {
			return nil, fmt.Errorf("cutover attempt event filename %q is invalid, expected %q", name, expected)
		}
		payload, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			return nil, fmt.Errorf("read cutover attempt event %q: %w", name, err)
		}
		event, err := DecodeStrictJSON[CutoverAttemptEvent](payload)
		if err != nil {
			return nil, fmt.Errorf("decode cutover attempt event %q: %w", name, err)
		}
		events = append(events, event)
	}
	projection, headSHA256, err := reduceCutoverAttemptEventsWithManifestHash(manifest, manifestSHA256, events)
	if err != nil {
		return nil, err
	}
	return &CutoverAttemptJournal{
		Directory: directory, Manifest: manifest, ManifestSHA256: manifestSHA256,
		Events: events, Projection: projection, HeadSHA256: headSHA256,
	}, nil
}

func (j *CutoverAttemptJournal) Append(eventType CutoverAttemptEventType, artifactSHA256 string, recordedAt time.Time) (CutoverAttemptEvent, error) {
	current, err := LoadCutoverAttemptJournal(j.Directory)
	if err != nil {
		return CutoverAttemptEvent{}, err
	}
	event := CutoverAttemptEvent{
		SchemaVersion: CutoverAttemptEventSchemaV1, AttemptID: current.Manifest.AttemptID,
		Sequence: current.Projection.LastSequence + 1, EventType: eventType,
		PreviousEventSHA256: current.HeadSHA256, ArtifactSHA256: artifactSHA256,
		RecordedAtUnixMS: recordedAt.UTC().UnixMilli(),
	}
	projection := current.Projection
	if err := projection.Apply(event); err != nil {
		return CutoverAttemptEvent{}, err
	}
	path := filepath.Join(j.Directory, cutoverAttemptEventFilename(event.Sequence))
	if err := writeImmutableCutoverJSON(path, event); err != nil {
		return CutoverAttemptEvent{}, fmt.Errorf("publish cutover attempt event: %w", err)
	}
	current.Events = append(current.Events, event)
	current.Projection = projection
	current.HeadSHA256, err = hashCanonicalCutoverValue(event)
	if err != nil {
		return CutoverAttemptEvent{}, err
	}
	*j = *current
	return event, nil
}

func reduceCutoverAttemptEvents(manifest CutoverAttemptManifest, events []CutoverAttemptEvent) (CutoverAttemptProjection, error) {
	manifest, manifestSHA256, err := validateCutoverAttemptManifest(manifest)
	if err != nil {
		return CutoverAttemptProjection{}, err
	}
	projection, _, err := reduceCutoverAttemptEventsWithManifestHash(manifest, manifestSHA256, events)
	return projection, err
}

func reduceCutoverAttemptEventsWithManifestHash(manifest CutoverAttemptManifest, manifestSHA256 string, events []CutoverAttemptEvent) (CutoverAttemptProjection, string, error) {
	projection := newCutoverAttemptProjection(manifest.AttemptID)
	projection.LastRecordedAtUnixMS = manifest.CreatedAtUnixMS
	headSHA256 := manifestSHA256
	for _, event := range events {
		if event.PreviousEventSHA256 != headSHA256 {
			return CutoverAttemptProjection{}, "", fmt.Errorf("cutover attempt event %d hash chain is invalid", event.Sequence)
		}
		if err := projection.Apply(event); err != nil {
			return CutoverAttemptProjection{}, "", err
		}
		var err error
		headSHA256, err = hashCanonicalCutoverValue(event)
		if err != nil {
			return CutoverAttemptProjection{}, "", err
		}
	}
	return projection, headSHA256, nil
}

func validateCutoverAttemptManifest(manifest CutoverAttemptManifest) (CutoverAttemptManifest, string, error) {
	manifest.AttemptID = strings.TrimSpace(manifest.AttemptID)
	if manifest.SchemaVersion != CutoverAttemptManifestSchemaV1 || !fenceTransitionIDPattern.MatchString(manifest.AttemptID) {
		return manifest, "", fmt.Errorf("cutover attempt manifest identity is invalid")
	}
	parsedSource, err := ParseAuthority(string(manifest.SourceAuthority))
	if err != nil {
		return manifest, "", fmt.Errorf("cutover attempt source authority is invalid: %w", err)
	}
	if parsedSource != manifest.SourceAuthority {
		return manifest, "", fmt.Errorf("cutover attempt source authority is not canonical")
	}
	parsedTarget, err := ParseAuthority(string(manifest.TargetAuthority))
	if err != nil {
		return manifest, "", fmt.Errorf("cutover attempt target authority is invalid: %w", err)
	}
	if parsedTarget != manifest.TargetAuthority {
		return manifest, "", fmt.Errorf("cutover attempt target authority is not canonical")
	}
	if manifest.SourceAuthority == manifest.TargetAuthority || manifest.InitialEpoch == 0 || manifest.InitialEpoch > ^uint64(0)-2 ||
		manifest.MaxInterruptionMS <= 0 || manifest.MaxInterruptionMS > int64((10*time.Minute)/time.Millisecond) || manifest.CreatedAtUnixMS <= 0 {
		return manifest, "", fmt.Errorf("cutover attempt manifest transition bounds are invalid")
	}
	for _, digest := range []string{
		manifest.InitialLeaseSHA256, manifest.SourceNodesManifestSHA256, manifest.FrozenNodesManifestSHA256,
		manifest.TargetNodesManifestSHA256, manifest.CheckpointManifestSHA256,
	} {
		if !validSHA256(digest) {
			return manifest, "", fmt.Errorf("cutover attempt manifest contains invalid SHA-256 binding")
		}
	}
	digest, err := hashCanonicalCutoverValue(manifest)
	return manifest, digest, err
}

func validateCutoverAttemptEvent(event CutoverAttemptEvent) error {
	if event.SchemaVersion != CutoverAttemptEventSchemaV1 || !fenceTransitionIDPattern.MatchString(event.AttemptID) ||
		event.Sequence == 0 || !validSHA256(event.PreviousEventSHA256) || !validSHA256(event.ArtifactSHA256) || event.RecordedAtUnixMS <= 0 {
		return fmt.Errorf("cutover attempt event is invalid")
	}
	if !validCutoverAttemptEventType(event.EventType) {
		return fmt.Errorf("cutover attempt event type %q is invalid", event.EventType)
	}
	return nil
}

func cutoverAttemptEventFilename(sequence uint64) string {
	return fmt.Sprintf("%020d%s", sequence, cutoverAttemptEventSuffix)
}

func hashCanonicalCutoverValue(value any) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode cutover attempt value: %w", err)
	}
	return hashBytes(payload), nil
}

func writeImmutableCutoverJSON(path string, value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode immutable cutover JSON: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".dipole-cutover-attempt-*.json")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err = temporary.Chmod(0o600); err == nil {
		_, err = temporary.Write(append(payload, '\n'))
	}
	if err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err := os.Link(temporaryPath, path); err != nil {
		return err
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}
