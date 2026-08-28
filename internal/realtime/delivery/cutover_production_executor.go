package delivery

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const CutoverDecisionArtifactSchemaV1 = "dipole.realtime.cutover-decision-artifact.v1"

type CutoverFenceTransitionStore interface {
	Apply(context.Context, FenceTransitionRequest) (FenceTransitionReceipt, error)
	GetReceipt(context.Context, string) (FenceTransitionReceipt, error)
}

type CutoverFenceObservationAggregator interface {
	Aggregate(context.Context, FenceExpectedNodeManifest, FenceTransitionReceipt) (FenceObservationAggregateReceipt, error)
}

type CutoverDualGroupCheckpointCollector interface {
	Capture(context.Context, DualGroupCheckpointManifest, FenceObservationAggregateReceipt) (DualGroupCheckpointReceipt, error)
}

type ProductionCutoverExecutorConfig struct {
	Manifest           CutoverAttemptManifest
	InitialTransition  FenceTransitionReceipt
	SourceNodes        FenceExpectedNodeManifest
	FrozenNodes        FenceExpectedNodeManifest
	TargetNodes        FenceExpectedNodeManifest
	CheckpointManifest DualGroupCheckpointManifest
	OperatorID         string
	LeaseDuration      time.Duration
	Now                func() time.Time
}

type CutoverDecisionArtifact struct {
	SchemaVersion string                  `json:"schema_version"`
	AttemptID     string                  `json:"attempt_id"`
	EventType     CutoverAttemptEventType `json:"event_type"`
	Decision      string                  `json:"decision"`
	ExpectedEpoch uint64                  `json:"expected_epoch"`
	DecidedAtMS   int64                   `json:"decided_at_unix_ms"`
}

type ProductionCutoverAttemptExecutor struct {
	config     ProductionCutoverExecutorConfig
	writer     CutoverFenceTransitionStore
	aggregator CutoverFenceObservationAggregator
	collector  CutoverDualGroupCheckpointCollector
	artifacts  *CutoverActionArtifactStore
}

func NewProductionCutoverAttemptExecutor(
	config ProductionCutoverExecutorConfig,
	writer CutoverFenceTransitionStore,
	aggregator CutoverFenceObservationAggregator,
	collector CutoverDualGroupCheckpointCollector,
	artifacts *CutoverActionArtifactStore,
) (*ProductionCutoverAttemptExecutor, error) {
	manifest, _, err := validateCutoverAttemptManifest(config.Manifest)
	if err != nil {
		return nil, err
	}
	config.Manifest = manifest
	config.OperatorID = strings.TrimSpace(config.OperatorID)
	if writer == nil || aggregator == nil || collector == nil || artifacts == nil || config.Now == nil ||
		!fenceTransitionIDPattern.MatchString(config.OperatorID) || config.LeaseDuration < 30*time.Second || config.LeaseDuration > time.Hour {
		return nil, fmt.Errorf("production cutover executor configuration is invalid")
	}
	if err := validateProductionCutoverInputs(config); err != nil {
		return nil, err
	}
	return &ProductionCutoverAttemptExecutor{
		config: config, writer: writer, aggregator: aggregator, collector: collector, artifacts: artifacts,
	}, nil
}

func (e *ProductionCutoverAttemptExecutor) Execute(ctx context.Context, action CutoverAttemptAction) (CutoverAttemptActionResult, error) {
	if err := e.validateAction(action); err != nil {
		return CutoverAttemptActionResult{}, err
	}
	if artifact, digest, err := e.artifacts.Load(action); err == nil {
		return actionResultFromArtifact(artifact, digest), nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return CutoverAttemptActionResult{}, err
	}

	switch action.EventType {
	case CutoverEventSourceCheckpointed:
		transition, err := e.latestTransition(action)
		if err != nil {
			return CutoverAttemptActionResult{}, err
		}
		return e.captureCheckpoint(ctx, action, e.config.SourceNodes, transition)
	case CutoverEventFreezeApplied:
		transition, err := e.latestTransition(action)
		if err != nil {
			return CutoverAttemptActionResult{}, err
		}
		bundle, err := latestCutoverPayload[CheckpointBundle](e, action, CutoverEventSourceCheckpointed)
		if err != nil {
			return CutoverAttemptActionResult{}, err
		}
		if bundle.Checkpoint.LeaseSHA256 != transition.NextSHA256 {
			return CutoverAttemptActionResult{}, fmt.Errorf("production cutover source checkpoint does not bind the current lease")
		}
		return e.applyTransition(ctx, action, FenceTransitionFreeze, "", bundle.Checkpoint.LeaseSHA256)
	case CutoverEventFrozenConfirmed:
		receipt, err := e.latestTransition(action)
		if err != nil {
			return CutoverAttemptActionResult{}, err
		}
		return e.captureObservation(ctx, action, e.config.FrozenNodes, receipt)
	case CutoverEventTargetActivated:
		transition, err := e.latestTransition(action)
		if err != nil {
			return CutoverAttemptActionResult{}, err
		}
		proof, err := latestCutoverPayload[FenceObservationAggregateReceipt](e, action, CutoverEventFrozenConfirmed)
		if err != nil {
			return CutoverAttemptActionResult{}, err
		}
		if proof.LeaseSHA256 != transition.NextSHA256 {
			return CutoverAttemptActionResult{}, fmt.Errorf("production cutover frozen proof does not bind the current lease")
		}
		return e.applyTransition(ctx, action, FenceTransitionActivate, action.TargetAuthority, proof.LeaseSHA256)
	case CutoverEventTargetCheckpointed:
		receipt, err := e.latestTransition(action)
		if err != nil {
			return CutoverAttemptActionResult{}, err
		}
		return e.captureCheckpoint(ctx, action, e.config.TargetNodes, receipt)
	case CutoverEventCompleted, CutoverEventRollbackRequested, CutoverEventRolledBack:
		return e.publishDecision(action)
	case CutoverEventRollbackFreezeApplied:
		receipt, err := e.latestTransition(action)
		if err != nil {
			return CutoverAttemptActionResult{}, err
		}
		return e.applyTransition(ctx, action, FenceTransitionFreeze, "", receipt.NextSHA256)
	case CutoverEventRollbackFrozenConfirmed:
		receipt, err := e.latestTransition(action)
		if err != nil {
			return CutoverAttemptActionResult{}, err
		}
		return e.captureObservation(ctx, action, e.config.SourceNodes, receipt)
	case CutoverEventSourceReactivated:
		transition, err := e.latestTransition(action)
		if err != nil {
			return CutoverAttemptActionResult{}, err
		}
		proof, err := latestCutoverPayload[FenceObservationAggregateReceipt](e, action, CutoverEventRollbackFrozenConfirmed)
		if err != nil {
			return CutoverAttemptActionResult{}, err
		}
		if proof.LeaseSHA256 != transition.NextSHA256 {
			return CutoverAttemptActionResult{}, fmt.Errorf("production cutover rollback proof does not bind the current lease")
		}
		return e.applyTransition(ctx, action, FenceTransitionActivate, action.SourceAuthority, proof.LeaseSHA256)
	case CutoverEventRollbackCheckpointed:
		receipt, err := e.latestTransition(action)
		if err != nil {
			return CutoverAttemptActionResult{}, err
		}
		return e.captureCheckpoint(ctx, action, e.config.SourceNodes, receipt)
	case CutoverEventLeaseRenewed:
		receipt, err := e.latestTransition(action)
		if err != nil {
			return CutoverAttemptActionResult{}, err
		}
		return e.applyTransition(ctx, action, FenceTransitionRenew, "", receipt.NextSHA256)
	default:
		return CutoverAttemptActionResult{}, fmt.Errorf("production cutover action %q is unsupported", action.EventType)
	}
}

func (e *ProductionCutoverAttemptExecutor) validateAction(action CutoverAttemptAction) error {
	if _, err := validateAndHashCutoverAction(action); err != nil {
		return err
	}
	m := e.config.Manifest
	if action.AttemptID != m.AttemptID || action.SourceAuthority != m.SourceAuthority || action.TargetAuthority != m.TargetAuthority ||
		action.MaxInterruptionMS != m.MaxInterruptionMS || action.ExpectedEpoch < m.InitialEpoch || action.ExpectedEpoch > m.InitialEpoch+2 {
		return fmt.Errorf("production cutover action does not match attempt manifest")
	}
	if !validProductionCutoverStateEvent(action.CurrentState, action.EventType) {
		return fmt.Errorf("production cutover action event does not match current state")
	}
	return nil
}

func validProductionCutoverStateEvent(state CutoverAttemptState, eventType CutoverAttemptEventType) bool {
	if eventType == CutoverEventLeaseRenewed {
		projection := CutoverAttemptProjection{State: state}
		_, _, err := projection.transition(eventType)
		return err == nil
	}
	switch state {
	case CutoverAttemptCreated:
		return eventType == CutoverEventSourceCheckpointed
	case CutoverAttemptSourceCheckpointed:
		return eventType == CutoverEventFreezeApplied
	case CutoverAttemptFreezeApplied:
		return eventType == CutoverEventFrozenConfirmed || eventType == CutoverEventRollbackRequested
	case CutoverAttemptFrozenConfirmed:
		return eventType == CutoverEventTargetActivated || eventType == CutoverEventRollbackRequested
	case CutoverAttemptTargetActivated:
		return eventType == CutoverEventTargetCheckpointed || eventType == CutoverEventRollbackRequested
	case CutoverAttemptTargetCheckpointed:
		return eventType == CutoverEventCompleted || eventType == CutoverEventRollbackRequested
	case CutoverAttemptCompleted:
		return eventType == CutoverEventRollbackRequested
	case CutoverAttemptRollbackRequested:
		return eventType == CutoverEventRollbackFreezeApplied || eventType == CutoverEventRollbackFrozenConfirmed
	case CutoverAttemptRollbackFreezeApplied:
		return eventType == CutoverEventRollbackFrozenConfirmed
	case CutoverAttemptRollbackFrozenConfirmed:
		return eventType == CutoverEventSourceReactivated
	case CutoverAttemptSourceReactivated:
		return eventType == CutoverEventRollbackCheckpointed
	case CutoverAttemptRollbackCheckpointed:
		return eventType == CutoverEventRolledBack
	default:
		return false
	}
}

func (e *ProductionCutoverAttemptExecutor) applyTransition(
	ctx context.Context,
	action CutoverAttemptAction,
	transitionAction FenceTransitionAction,
	target Authority,
	expectedSHA256 string,
) (CutoverAttemptActionResult, error) {
	receipt, err := e.writer.GetReceipt(ctx, action.ActionID)
	if err != nil && !errors.Is(err, ErrFenceTransitionReceiptNotFound) {
		return CutoverAttemptActionResult{}, err
	}
	if errors.Is(err, ErrFenceTransitionReceiptNotFound) {
		receipt, err = e.writer.Apply(ctx, FenceTransitionRequest{
			TransitionID: action.ActionID, Action: transitionAction, OperatorID: e.config.OperatorID,
			Reason:         "cutover attempt " + action.AttemptID + " " + string(action.EventType),
			ExpectedSHA256: expectedSHA256, TargetAuthority: target,
			LeaseUntil: e.config.Now().UTC().Add(e.config.LeaseDuration),
		})
		if err != nil {
			return CutoverAttemptActionResult{}, err
		}
	}
	if err := validateProductionTransitionReceipt(receipt, action, transitionAction, target, expectedSHA256); err != nil {
		return CutoverAttemptActionResult{}, err
	}
	return e.publish(action, receipt, time.UnixMilli(receipt.AppliedAtUnixMS))
}

func (e *ProductionCutoverAttemptExecutor) captureObservation(
	ctx context.Context,
	action CutoverAttemptAction,
	manifest FenceExpectedNodeManifest,
	transition FenceTransitionReceipt,
) (CutoverAttemptActionResult, error) {
	proof, err := e.aggregator.Aggregate(ctx, manifest, transition)
	if err != nil {
		return CutoverAttemptActionResult{}, err
	}
	_, expectedManifestSHA, manifestErr := validateExpectedNodeManifest(manifest)
	if manifestErr != nil || proof.ManifestSHA256 != expectedManifestSHA || proof.ManifestID != manifest.ManifestID ||
		proof.TransitionID != transition.TransitionID || proof.RequestSHA256 != transition.RequestSHA256 ||
		proof.Authority != transition.Authority || proof.Epoch != action.ExpectedEpoch ||
		proof.Phase != FencePhaseFrozen || proof.LeaseSHA256 != transition.NextSHA256 {
		return CutoverAttemptActionResult{}, fmt.Errorf("production cutover frozen observation proof binding is invalid")
	}
	return e.publish(action, proof, time.UnixMilli(proof.CapturedAtUnixMS))
}

func (e *ProductionCutoverAttemptExecutor) captureCheckpoint(
	ctx context.Context,
	action CutoverAttemptAction,
	nodes FenceExpectedNodeManifest,
	transition FenceTransitionReceipt,
) (CutoverAttemptActionResult, error) {
	proof, err := e.aggregator.Aggregate(ctx, nodes, transition)
	if err != nil {
		return CutoverAttemptActionResult{}, err
	}
	checkpoint, err := e.collector.Capture(ctx, e.config.CheckpointManifest, proof)
	if err != nil {
		return CutoverAttemptActionResult{}, err
	}
	bundle, err := NewCheckpointBundle(proof, checkpoint)
	if err != nil {
		return CutoverAttemptActionResult{}, err
	}
	_, expectedNodesSHA, nodesErr := validateExpectedNodeManifest(nodes)
	if nodesErr != nil || proof.ManifestSHA256 != expectedNodesSHA || proof.ManifestID != nodes.ManifestID ||
		bundle.Checkpoint.ManifestID != e.config.CheckpointManifest.ManifestID ||
		bundle.Checkpoint.ManifestSHA256 != e.config.Manifest.CheckpointManifestSHA256 ||
		bundle.Checkpoint.Epoch != action.ExpectedEpoch || bundle.Checkpoint.Phase != FencePhaseActive ||
		bundle.Checkpoint.Authority != transition.Authority || strings.TrimSpace(bundle.Checkpoint.ClusterID) == "" {
		return CutoverAttemptActionResult{}, fmt.Errorf("production cutover checkpoint authority binding is invalid")
	}
	return e.publish(action, bundle, time.UnixMilli(bundle.Checkpoint.CapturedAtUnixMS))
}

func (e *ProductionCutoverAttemptExecutor) publishDecision(action CutoverAttemptAction) (CutoverAttemptActionResult, error) {
	now := e.config.Now().UTC()
	decision := CutoverDecisionArtifact{
		SchemaVersion: CutoverDecisionArtifactSchemaV1, AttemptID: action.AttemptID,
		EventType: action.EventType, Decision: string(action.EventType), ExpectedEpoch: action.ExpectedEpoch,
		DecidedAtMS: now.UnixMilli(),
	}
	return e.publish(action, decision, now)
}

func (e *ProductionCutoverAttemptExecutor) publish(action CutoverAttemptAction, payload any, recordedAt time.Time) (CutoverAttemptActionResult, error) {
	artifact, digest, err := e.artifacts.Publish(action, payload, recordedAt)
	if err != nil {
		return CutoverAttemptActionResult{}, err
	}
	return actionResultFromArtifact(artifact, digest), nil
}

func latestCutoverPayload[T any](e *ProductionCutoverAttemptExecutor, action CutoverAttemptAction, eventType CutoverAttemptEventType) (T, error) {
	for sequence := action.Sequence - 1; sequence > 0; sequence-- {
		actionID, err := cutoverAttemptActionID(action.AttemptID, sequence, eventType)
		if err != nil {
			var value T
			return value, err
		}
		artifact, _, err := e.artifacts.LoadByActionID(actionID)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			var value T
			return value, err
		}
		if artifact.AttemptID != action.AttemptID || artifact.EventType != eventType || artifact.Sequence != sequence {
			var value T
			return value, fmt.Errorf("production cutover predecessor artifact binding is invalid")
		}
		return DecodeCutoverActionArtifactPayload[T](artifact)
	}
	var value T
	return value, fmt.Errorf("production cutover predecessor artifact %s is missing", eventType)
}

func validateProductionCutoverInputs(config ProductionCutoverExecutorConfig) error {
	_, sourceSHA, err := validateExpectedNodeManifest(config.SourceNodes)
	if err != nil || sourceSHA != config.Manifest.SourceNodesManifestSHA256 {
		return fmt.Errorf("production cutover source-node manifest binding is invalid")
	}
	_, frozenSHA, err := validateExpectedNodeManifest(config.FrozenNodes)
	if err != nil || frozenSHA != config.Manifest.FrozenNodesManifestSHA256 {
		return fmt.Errorf("production cutover frozen-node manifest binding is invalid")
	}
	_, targetSHA, err := validateExpectedNodeManifest(config.TargetNodes)
	if err != nil || targetSHA != config.Manifest.TargetNodesManifestSHA256 {
		return fmt.Errorf("production cutover target-node manifest binding is invalid")
	}
	_, checkpointSHA, err := validateDualGroupCheckpointManifest(config.CheckpointManifest)
	if err != nil || checkpointSHA != config.Manifest.CheckpointManifestSHA256 {
		return fmt.Errorf("production cutover checkpoint manifest binding is invalid")
	}
	if err := validateAggregateTransitionReceipt(config.InitialTransition); err != nil ||
		config.InitialTransition.Authority != config.Manifest.SourceAuthority || config.InitialTransition.Phase != FencePhaseActive ||
		config.InitialTransition.Epoch != config.Manifest.InitialEpoch || config.InitialTransition.NextSHA256 != config.Manifest.InitialLeaseSHA256 {
		return fmt.Errorf("production cutover initial transition binding is invalid")
	}
	return nil
}

func validateProductionTransitionReceipt(
	receipt FenceTransitionReceipt,
	action CutoverAttemptAction,
	transitionAction FenceTransitionAction,
	target Authority,
	expectedSHA256 string,
) error {
	if err := validateAggregateTransitionReceipt(receipt); err != nil {
		return err
	}
	expectedAuthority := action.SourceAuthority
	expectedPhase := FencePhaseFrozen
	if transitionAction == FenceTransitionActivate {
		expectedAuthority = target
		expectedPhase = FencePhaseActive
	} else if action.EventType == CutoverEventRollbackFreezeApplied {
		expectedAuthority = action.TargetAuthority
	}
	if transitionAction == FenceTransitionRenew {
		expectedAuthority = action.SourceAuthority
		if action.CurrentState == CutoverAttemptTargetActivated || action.CurrentState == CutoverAttemptTargetCheckpointed ||
			action.CurrentState == CutoverAttemptRollbackFreezeApplied || action.CurrentState == CutoverAttemptRollbackFrozenConfirmed {
			expectedAuthority = action.TargetAuthority
		}
		expectedPhase = FencePhaseActive
		if action.CurrentState == CutoverAttemptFreezeApplied || action.CurrentState == CutoverAttemptFrozenConfirmed ||
			action.CurrentState == CutoverAttemptRollbackFreezeApplied || action.CurrentState == CutoverAttemptRollbackFrozenConfirmed {
			expectedPhase = FencePhaseFrozen
		}
	}
	if receipt.TransitionID != action.ActionID || receipt.Action != transitionAction || receipt.PreviousSHA256 != expectedSHA256 ||
		receipt.Authority != expectedAuthority || receipt.Phase != expectedPhase || receipt.Epoch != action.ExpectedEpoch {
		return fmt.Errorf("production cutover transition receipt binding is invalid")
	}
	return nil
}

func (e *ProductionCutoverAttemptExecutor) latestTransition(action CutoverAttemptAction) (FenceTransitionReceipt, error) {
	if action.LeaseTransitionActionID == "" {
		return e.config.InitialTransition, nil
	}
	artifact, _, err := e.artifacts.LoadByActionID(action.LeaseTransitionActionID)
	if err != nil {
		return FenceTransitionReceipt{}, err
	}
	if artifact.AttemptID != action.AttemptID || artifact.Sequence >= action.Sequence {
		return FenceTransitionReceipt{}, fmt.Errorf("production cutover transition artifact binding is invalid")
	}
	switch artifact.EventType {
	case CutoverEventLeaseRenewed, CutoverEventSourceReactivated, CutoverEventRollbackFreezeApplied,
		CutoverEventTargetActivated, CutoverEventFreezeApplied:
	default:
		return FenceTransitionReceipt{}, fmt.Errorf("production cutover transition artifact event is invalid")
	}
	receipt, err := DecodeCutoverActionArtifactPayload[FenceTransitionReceipt](artifact)
	if err != nil {
		return FenceTransitionReceipt{}, err
	}
	if err := validateAggregateTransitionReceipt(receipt); err != nil {
		return FenceTransitionReceipt{}, err
	}
	return receipt, nil
}

func actionResultFromArtifact(artifact CutoverActionArtifact, digest string) CutoverAttemptActionResult {
	return CutoverAttemptActionResult{ArtifactSHA256: digest, RecordedAtUnixMS: artifact.RecordedAtUnixMS}
}
