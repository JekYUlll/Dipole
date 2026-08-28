package delivery

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type CutoverAttemptAction struct {
	ActionID                   string
	AttemptID                  string
	Sequence                   uint64
	EventType                  CutoverAttemptEventType
	CurrentState               CutoverAttemptState
	SourceAuthority            Authority
	TargetAuthority            Authority
	ExpectedEpoch              uint64
	MaxInterruptionMS          int64
	InterruptionDeadlineUnixMS int64
}

type CutoverAttemptActionResult struct {
	ArtifactSHA256   string
	RecordedAtUnixMS int64
}

type CutoverAttemptActionExecutor interface {
	// Execute must be idempotent for ActionID and return the same immutable artifact on retry.
	Execute(ctx context.Context, action CutoverAttemptAction) (CutoverAttemptActionResult, error)
}

type CutoverAttemptAdvance struct {
	State             CutoverAttemptState
	Sequence          uint64
	EventType         CutoverAttemptEventType
	Terminal          bool
	RollbackTriggered bool
}

type CutoverAttemptOrchestrator struct {
	journal  *CutoverAttemptJournal
	executor CutoverAttemptActionExecutor
	now      func() time.Time
}

func NewCutoverAttemptOrchestrator(
	journal *CutoverAttemptJournal,
	executor CutoverAttemptActionExecutor,
	now func() time.Time,
) (*CutoverAttemptOrchestrator, error) {
	if journal == nil || strings.TrimSpace(journal.Directory) == "" || executor == nil || now == nil {
		return nil, fmt.Errorf("cutover attempt orchestrator configuration is invalid")
	}
	return &CutoverAttemptOrchestrator{journal: journal, executor: executor, now: now}, nil
}

func (o *CutoverAttemptOrchestrator) Advance(ctx context.Context) (CutoverAttemptAdvance, error) {
	if err := o.reload(); err != nil {
		return CutoverAttemptAdvance{}, err
	}
	if terminalCutoverAttemptState(o.journal.Projection.State) {
		return o.currentAdvance(true), nil
	}
	eventType, rollbackTriggered, err := o.nextEvent()
	if err != nil {
		return CutoverAttemptAdvance{}, err
	}
	return o.execute(ctx, eventType, rollbackTriggered)
}

func (o *CutoverAttemptOrchestrator) RequestRollback(ctx context.Context) (CutoverAttemptAdvance, error) {
	if err := o.reload(); err != nil {
		return CutoverAttemptAdvance{}, err
	}
	switch o.journal.Projection.State {
	case CutoverAttemptFreezeApplied, CutoverAttemptFrozenConfirmed,
		CutoverAttemptTargetActivated, CutoverAttemptTargetCheckpointed, CutoverAttemptCompleted:
		return o.execute(ctx, CutoverEventRollbackRequested, false)
	default:
		return CutoverAttemptAdvance{}, fmt.Errorf("cutover attempt cannot request rollback from state %q", o.journal.Projection.State)
	}
}

func (o *CutoverAttemptOrchestrator) reload() error {
	journal, err := LoadCutoverAttemptJournal(o.journal.Directory)
	if err != nil {
		return err
	}
	*o.journal = *journal
	return nil
}

func (o *CutoverAttemptOrchestrator) nextEvent() (CutoverAttemptEventType, bool, error) {
	state := o.journal.Projection.State
	if (state == CutoverAttemptFreezeApplied || state == CutoverAttemptFrozenConfirmed) && o.initialFreezeExpired() {
		return CutoverEventRollbackRequested, true, nil
	}
	switch state {
	case CutoverAttemptCreated:
		return CutoverEventSourceCheckpointed, false, nil
	case CutoverAttemptSourceCheckpointed:
		return CutoverEventFreezeApplied, false, nil
	case CutoverAttemptFreezeApplied:
		return CutoverEventFrozenConfirmed, false, nil
	case CutoverAttemptFrozenConfirmed:
		return CutoverEventTargetActivated, false, nil
	case CutoverAttemptTargetActivated:
		return CutoverEventTargetCheckpointed, false, nil
	case CutoverAttemptTargetCheckpointed:
		return CutoverEventCompleted, false, nil
	case CutoverAttemptRollbackRequested:
		if o.journal.Projection.RollbackNeedsFreeze {
			return CutoverEventRollbackFreezeApplied, false, nil
		}
		return CutoverEventSourceReactivated, false, nil
	case CutoverAttemptRollbackFreezeApplied:
		return CutoverEventRollbackFrozenConfirmed, false, nil
	case CutoverAttemptRollbackFrozenConfirmed:
		return CutoverEventSourceReactivated, false, nil
	case CutoverAttemptSourceReactivated:
		return CutoverEventRollbackCheckpointed, false, nil
	case CutoverAttemptRollbackCheckpointed:
		return CutoverEventRolledBack, false, nil
	default:
		return "", false, fmt.Errorf("cutover attempt state %q has no continuation", state)
	}
}

func (o *CutoverAttemptOrchestrator) initialFreezeExpired() bool {
	for _, event := range o.journal.Events {
		if event.EventType == CutoverEventFreezeApplied {
			deadline := time.UnixMilli(event.RecordedAtUnixMS).Add(time.Duration(o.journal.Manifest.MaxInterruptionMS) * time.Millisecond)
			return !o.now().UTC().Before(deadline)
		}
	}
	return false
}

func (o *CutoverAttemptOrchestrator) execute(
	ctx context.Context,
	eventType CutoverAttemptEventType,
	rollbackTriggered bool,
) (CutoverAttemptAdvance, error) {
	action, err := o.action(eventType)
	if err != nil {
		return CutoverAttemptAdvance{}, err
	}
	result, err := o.executor.Execute(ctx, action)
	if err != nil {
		return CutoverAttemptAdvance{}, fmt.Errorf("execute cutover attempt action %s: %w", eventType, err)
	}
	now := o.now().UTC()
	if !validSHA256(result.ArtifactSHA256) || result.RecordedAtUnixMS <= 0 ||
		time.UnixMilli(result.RecordedAtUnixMS).After(now.Add(2*time.Second)) {
		return CutoverAttemptAdvance{}, fmt.Errorf("cutover attempt action %s returned invalid evidence", eventType)
	}
	event, err := o.journal.Append(eventType, result.ArtifactSHA256, time.UnixMilli(result.RecordedAtUnixMS))
	if err != nil {
		return CutoverAttemptAdvance{}, err
	}
	return CutoverAttemptAdvance{
		State: o.journal.Projection.State, Sequence: event.Sequence, EventType: event.EventType,
		Terminal: terminalCutoverAttemptState(o.journal.Projection.State), RollbackTriggered: rollbackTriggered,
	}, nil
}

func (o *CutoverAttemptOrchestrator) action(eventType CutoverAttemptEventType) (CutoverAttemptAction, error) {
	sequence := o.journal.Projection.LastSequence + 1
	actionID, err := cutoverAttemptActionID(o.journal.Manifest.AttemptID, sequence, eventType)
	if err != nil {
		return CutoverAttemptAction{}, err
	}
	return CutoverAttemptAction{
		ActionID: actionID, AttemptID: o.journal.Manifest.AttemptID, Sequence: sequence,
		EventType: eventType, CurrentState: o.journal.Projection.State,
		SourceAuthority: o.journal.Manifest.SourceAuthority, TargetAuthority: o.journal.Manifest.TargetAuthority,
		ExpectedEpoch: o.expectedEpoch(eventType), MaxInterruptionMS: o.journal.Manifest.MaxInterruptionMS,
		InterruptionDeadlineUnixMS: o.interruptionDeadlineUnixMS(),
	}, nil
}

func (o *CutoverAttemptOrchestrator) expectedEpoch(eventType CutoverAttemptEventType) uint64 {
	epoch := o.journal.Manifest.InitialEpoch
	switch eventType {
	case CutoverEventFreezeApplied, CutoverEventFrozenConfirmed, CutoverEventTargetActivated,
		CutoverEventTargetCheckpointed, CutoverEventCompleted, CutoverEventRollbackRequested:
		return epoch + 1
	case CutoverEventRollbackFreezeApplied, CutoverEventRollbackFrozenConfirmed,
		CutoverEventSourceReactivated, CutoverEventRollbackCheckpointed, CutoverEventRolledBack:
		if o.hasRollbackFreeze() {
			return epoch + 2
		}
		return epoch + 1
	default:
		return epoch
	}
}

func (o *CutoverAttemptOrchestrator) hasRollbackFreeze() bool {
	for _, event := range o.journal.Events {
		if event.EventType == CutoverEventRollbackFreezeApplied {
			return true
		}
	}
	return o.journal.Projection.RollbackNeedsFreeze
}

func (o *CutoverAttemptOrchestrator) interruptionDeadlineUnixMS() int64 {
	for index := len(o.journal.Events) - 1; index >= 0; index-- {
		event := o.journal.Events[index]
		if event.EventType == CutoverEventFreezeApplied || event.EventType == CutoverEventRollbackFreezeApplied {
			return time.UnixMilli(event.RecordedAtUnixMS).
				Add(time.Duration(o.journal.Manifest.MaxInterruptionMS) * time.Millisecond).UnixMilli()
		}
	}
	return 0
}

func (o *CutoverAttemptOrchestrator) currentAdvance(terminal bool) CutoverAttemptAdvance {
	return CutoverAttemptAdvance{
		State: o.journal.Projection.State, Sequence: o.journal.Projection.LastSequence, Terminal: terminal,
	}
}

func terminalCutoverAttemptState(state CutoverAttemptState) bool {
	return state == CutoverAttemptCompleted || state == CutoverAttemptRolledBack
}

func cutoverAttemptActionID(attemptID string, sequence uint64, eventType CutoverAttemptEventType) (string, error) {
	payload, err := json.Marshal(struct {
		AttemptID string                  `json:"attempt_id"`
		Sequence  uint64                  `json:"sequence"`
		EventType CutoverAttemptEventType `json:"event_type"`
	}{attemptID, sequence, eventType})
	if err != nil {
		return "", fmt.Errorf("encode cutover attempt action identity: %w", err)
	}
	return "cutover-" + hashBytes(payload)[:32], nil
}
