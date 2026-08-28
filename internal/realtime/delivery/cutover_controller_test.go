package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

type controllerOrchestratorStub struct {
	advanceResults []CutoverAttemptAdvance
	advanceErrors  []error
	renewResult    CutoverAttemptAdvance
	calls          []string
}

func (s *controllerOrchestratorStub) Advance(context.Context) (CutoverAttemptAdvance, error) {
	s.calls = append(s.calls, "advance")
	index := len(s.callsOf("advance")) - 1
	if index < len(s.advanceErrors) && s.advanceErrors[index] != nil {
		return CutoverAttemptAdvance{}, s.advanceErrors[index]
	}
	if index < len(s.advanceResults) {
		return s.advanceResults[index], nil
	}
	return CutoverAttemptAdvance{}, errors.New("unexpected advance")
}

func (s *controllerOrchestratorStub) RenewLease(context.Context) (CutoverAttemptAdvance, error) {
	s.calls = append(s.calls, "renew")
	return s.renewResult, nil
}

func (s *controllerOrchestratorStub) callsOf(value string) []string {
	result := make([]string, 0)
	for _, call := range s.calls {
		if call == value {
			result = append(result, call)
		}
	}
	return result
}

type controllerOwnershipStub struct {
	acquired bool
	renewed  bool
	released bool
}

func (s *controllerOwnershipStub) Acquire(context.Context, string, time.Duration) (bool, error) {
	return s.acquired, nil
}

func (s *controllerOwnershipStub) Renew(context.Context, string, time.Duration) (bool, error) {
	return s.renewed, nil
}

func (s *controllerOwnershipStub) Release(context.Context, string) error {
	s.released = true
	return nil
}

type controllerAuthorityLeaseStub struct {
	lease FenceTransitionReceipt
}

type cutoverControllerDrillReport struct {
	SchemaVersion       string `json:"schema_version"`
	GitRevision         string `json:"git_revision"`
	RedisMode           string `json:"redis_mode"`
	AttemptID           string `json:"attempt_id"`
	ProcessAExitCode    int    `json:"process_a_exit_code"`
	ProcessASequence    uint64 `json:"process_a_sequence"`
	PreExpiryBlocked    bool   `json:"pre_expiry_blocked"`
	ProcessBResumed     bool   `json:"process_b_resumed"`
	FinalState          string `json:"final_state"`
	FinalSequence       uint64 `json:"final_sequence"`
	FinalJournalHeadSHA string `json:"final_journal_head_sha256"`
	ControlLeaseTTLMS   int64  `json:"control_lease_ttl_ms"`
	CompletedAtUnixMS   int64  `json:"completed_at_unix_ms"`
}

type controllerArtifactExecutor struct {
	artifacts *CutoverActionArtifactStore
	now       time.Time
	cancel    context.CancelFunc
}

func (e *controllerArtifactExecutor) Execute(_ context.Context, action CutoverAttemptAction) (CutoverAttemptActionResult, error) {
	artifact, digest, err := e.artifacts.Publish(action, struct {
		Status string `json:"status"`
	}{Status: "captured"}, e.now)
	if err != nil {
		return CutoverAttemptActionResult{}, err
	}
	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
	}
	return CutoverAttemptActionResult{ArtifactSHA256: digest, RecordedAtUnixMS: artifact.RecordedAtUnixMS}, nil
}

type noReleaseControllerOwnership struct {
	CutoverControllerOwnership
}

func (noReleaseControllerOwnership) Release(context.Context, string) error { return nil }

type exitAfterAdvanceOrchestrator struct {
	CutoverControllerOrchestrator
}

func (o exitAfterAdvanceOrchestrator) Advance(ctx context.Context) (CutoverAttemptAdvance, error) {
	result, err := o.CutoverControllerOrchestrator.Advance(ctx)
	if err == nil {
		os.Exit(91)
	}
	return result, err
}

func (s controllerAuthorityLeaseStub) CurrentLease(context.Context) (FenceTransitionReceipt, error) {
	return s.lease, nil
}

func TestCutoverControllerAdvancesBeforeRenewingAndRecoversBlockedAction(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	orchestrator := &controllerOrchestratorStub{
		advanceErrors: []error{errors.New("primary group is rebalancing"), nil},
		advanceResults: []CutoverAttemptAdvance{{}, {
			State: CutoverAttemptCompleted, Sequence: 3, EventType: CutoverEventCompleted, Terminal: true,
		}},
		renewResult: CutoverAttemptAdvance{State: CutoverAttemptTargetActivated, Sequence: 2, EventType: CutoverEventLeaseRenewed},
	}
	ownership := &controllerOwnershipStub{acquired: true, renewed: true}
	controller, err := NewCutoverAttemptController(CutoverAttemptControllerConfig{
		Orchestrator: orchestrator, Ownership: ownership,
		AuthorityLease: controllerAuthorityLeaseStub{lease: FenceTransitionReceipt{LeaseUntilUnixMS: now.Add(20 * time.Second).UnixMilli()}},
		OwnerID:        "controller-a", OwnershipTTL: 2 * time.Minute, ActionTimeout: 30 * time.Second,
		RenewBefore: time.Minute, RetryInterval: time.Millisecond, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := controller.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Terminal || result.State != CutoverAttemptCompleted || !ownership.released {
		t.Fatalf("result=%+v ownership=%+v", result, ownership)
	}
	expected := []string{"advance", "renew", "advance"}
	if len(orchestrator.calls) != len(expected) {
		t.Fatalf("calls=%v", orchestrator.calls)
	}
	for index := range expected {
		if orchestrator.calls[index] != expected[index] {
			t.Fatalf("calls=%v", orchestrator.calls)
		}
	}
}

func TestCutoverControllerRejectsConcurrentOwnerAndLostRenewal(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name      string
		ownership *controllerOwnershipStub
	}{
		{name: "already owned", ownership: &controllerOwnershipStub{}},
		{name: "ownership lost", ownership: &controllerOwnershipStub{acquired: true}},
	} {
		t.Run(test.name, func(t *testing.T) {
			orchestrator := &controllerOrchestratorStub{advanceErrors: []error{errors.New("blocked")}}
			controller, err := NewCutoverAttemptController(CutoverAttemptControllerConfig{
				Orchestrator: orchestrator, Ownership: test.ownership,
				AuthorityLease: controllerAuthorityLeaseStub{lease: FenceTransitionReceipt{LeaseUntilUnixMS: now.Add(time.Hour).UnixMilli()}},
				OwnerID:        "controller-a", OwnershipTTL: 2 * time.Minute, ActionTimeout: 30 * time.Second,
				RenewBefore: time.Minute, RetryInterval: time.Millisecond, Now: func() time.Time { return now },
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := controller.Run(context.Background()); !errors.Is(err, ErrCutoverControllerOwnershipUnavailable) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestRedisCutoverControllerOwnershipSupportsExpiryTakeover(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	ownership, err := NewRedisCutoverControllerOwnership(client, "dipole:cutover:controller:attempt-a")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if acquired, err := ownership.Acquire(ctx, "owner-a", 10*time.Second); err != nil || !acquired {
		t.Fatalf("first acquire=%v err=%v", acquired, err)
	}
	if acquired, err := ownership.Acquire(ctx, "owner-b", 10*time.Second); err != nil || acquired {
		t.Fatalf("concurrent acquire=%v err=%v", acquired, err)
	}
	server.FastForward(11 * time.Second)
	if acquired, err := ownership.Acquire(ctx, "owner-b", 10*time.Second); err != nil || !acquired {
		t.Fatalf("takeover acquire=%v err=%v", acquired, err)
	}
	if renewed, err := ownership.Renew(ctx, "owner-a", 10*time.Second); err != nil || renewed {
		t.Fatalf("stale renew=%v err=%v", renewed, err)
	}
	if err := ownership.Release(ctx, "owner-a"); err != nil {
		t.Fatal(err)
	}
	if renewed, err := ownership.Renew(ctx, "owner-b", 10*time.Second); err != nil || !renewed {
		t.Fatalf("current renew=%v err=%v", renewed, err)
	}
}

func TestCutoverWorkspaceAuthorityLeaseReadsOnlyJournaledTransition(t *testing.T) {
	f := newProductionExecutorFixture(t)
	workspace, err := CreateCutoverAttemptWorkspace(
		t.TempDir(), f.config.Manifest.AttemptID, f.config.Manifest.SourceAuthority, f.config.Manifest.TargetAuthority,
		time.Duration(f.config.Manifest.MaxInterruptionMS)*time.Millisecond, productionWorkspaceInputs(f.config), f.now,
	)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewCutoverWorkspaceAuthorityLease(workspace)
	if err != nil {
		t.Fatal(err)
	}
	initial, err := source.CurrentLease(context.Background())
	if err != nil || initial != f.config.InitialTransition {
		t.Fatalf("initial=%+v err=%v", initial, err)
	}
	collector, err := NewDualGroupCheckpointCollector(checkpointSourceStub{snapshot: validCheckpointSnapshot()}, func() time.Time { return f.now })
	if err != nil {
		t.Fatal(err)
	}
	executor, err := NewProductionCutoverAttemptExecutor(f.config, f.writer, f.aggregator, collector, workspace.Artifacts)
	if err != nil {
		t.Fatal(err)
	}
	orchestrator, err := NewCutoverAttemptOrchestrator(workspace.Journal, executor, func() time.Time { return f.now })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := orchestrator.Advance(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := orchestrator.Advance(context.Background()); err != nil {
		t.Fatal(err)
	}
	frozen, err := source.CurrentLease(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if frozen.Action != FenceTransitionFreeze || frozen.Phase != FencePhaseFrozen || frozen.Epoch != f.config.Manifest.InitialEpoch+1 {
		t.Fatalf("frozen=%+v", frozen)
	}
}

func TestCutoverControllerReplacementResumesJournalAfterOwnershipExpiry(t *testing.T) {
	f := newProductionExecutorFixture(t)
	now := f.now
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	workspace, err := CreateCutoverAttemptWorkspace(
		t.TempDir(), "controller-replace-a", AuthorityGo, AuthorityCPP, time.Minute,
		productionWorkspaceInputs(f.config), now,
	)
	if err != nil {
		t.Fatal(err)
	}
	ownership, err := NewRedisCutoverControllerOwnership(client, "cutover:controller:replace-a")
	if err != nil {
		t.Fatal(err)
	}
	authorityLease, err := NewCutoverWorkspaceAuthorityLease(workspace)
	if err != nil {
		t.Fatal(err)
	}
	ctxA, cancelA := context.WithCancel(context.Background())
	executorA := &controllerArtifactExecutor{artifacts: workspace.Artifacts, now: now.Add(time.Second), cancel: cancelA}
	orchestratorA, err := NewCutoverAttemptOrchestrator(workspace.Journal, executorA, func() time.Time { return now.Add(time.Second) })
	if err != nil {
		t.Fatal(err)
	}
	controllerA, err := NewCutoverAttemptController(CutoverAttemptControllerConfig{
		Orchestrator: orchestratorA, Ownership: noReleaseControllerOwnership{ownership}, AuthorityLease: authorityLease,
		OwnerID: "controller-a", OwnershipTTL: 5 * time.Second, ActionTimeout: 2 * time.Second,
		RenewBefore: time.Second, RetryInterval: time.Millisecond, Now: func() time.Time { return now.Add(time.Second) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := controllerA.Run(ctxA); !errors.Is(err, context.Canceled) {
		t.Fatalf("controller A err=%v", err)
	}
	loaded, err := LoadCutoverAttemptJournal(workspace.Directory)
	if err != nil || loaded.Projection.LastSequence != 1 {
		t.Fatalf("controller A journal=%+v err=%v", loaded, err)
	}

	executorB := &controllerArtifactExecutor{artifacts: workspace.Artifacts, now: now.Add(2 * time.Second)}
	orchestratorB, err := NewCutoverAttemptOrchestrator(loaded, executorB, func() time.Time { return now.Add(2 * time.Second) })
	if err != nil {
		t.Fatal(err)
	}
	controllerB, err := NewCutoverAttemptController(CutoverAttemptControllerConfig{
		Orchestrator: orchestratorB, Ownership: ownership, AuthorityLease: authorityLease,
		OwnerID: "controller-b", OwnershipTTL: 5 * time.Second, ActionTimeout: 2 * time.Second,
		RenewBefore: time.Second, RetryInterval: time.Millisecond, Now: func() time.Time { return now.Add(2 * time.Second) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := controllerB.Run(context.Background()); !errors.Is(err, ErrCutoverControllerOwnershipUnavailable) {
		t.Fatalf("controller B early err=%v", err)
	}
	server.FastForward(6 * time.Second)
	result, err := controllerB.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Terminal || result.State != CutoverAttemptCompleted || result.Sequence != 6 {
		t.Fatalf("controller B result=%+v", result)
	}
}

func TestCutoverControllerRealProcessReplacement(t *testing.T) {
	if os.Getenv("DIPOLE_CUTOVER_CONTROLLER_HELPER") == "1" {
		runCutoverControllerProcessHelper(t)
		return
	}
	f := newProductionExecutorFixture(t)
	workspace, err := CreateCutoverAttemptWorkspace(
		t.TempDir(), "controller-process-a", AuthorityGo, AuthorityCPP, time.Minute,
		productionWorkspaceInputs(f.config), f.now,
	)
	if err != nil {
		t.Fatal(err)
	}
	redisAddr := os.Getenv("DIPOLE_CUTOVER_CONTROLLER_DRILL_REDIS_ADDR")
	redisMode := "miniredis"
	var fastForward func()
	if redisAddr == "" {
		server := miniredis.RunT(t)
		redisAddr = server.Addr()
		fastForward = func() { server.FastForward(6 * time.Second) }
	} else {
		redisMode = "redis"
		fastForward = func() { time.Sleep(6 * time.Second) }
	}
	key := fmt.Sprintf("cutover:controller:process:%d", time.Now().UnixNano())
	runHelper := func(owner, mode string, now time.Time) *exec.Cmd {
		command := exec.Command(os.Args[0], "-test.run=^TestCutoverControllerRealProcessReplacement$", "-test.count=1")
		command.Env = append(os.Environ(),
			"DIPOLE_CUTOVER_CONTROLLER_HELPER=1",
			"DIPOLE_CUTOVER_CONTROLLER_REDIS="+redisAddr,
			"DIPOLE_CUTOVER_CONTROLLER_KEY="+key,
			"DIPOLE_CUTOVER_CONTROLLER_WORKSPACE="+workspace.Directory,
			"DIPOLE_CUTOVER_CONTROLLER_OWNER="+owner,
			"DIPOLE_CUTOVER_CONTROLLER_MODE="+mode,
			"DIPOLE_CUTOVER_CONTROLLER_NOW="+strconv.FormatInt(now.UnixMilli(), 10),
		)
		return command
	}
	commandA := runHelper("process-a", "crash", f.now.Add(time.Second))
	outputA, err := commandA.CombinedOutput()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 91 {
		t.Fatalf("controller process A err=%v output=%s", err, outputA)
	}
	journal, err := LoadCutoverAttemptJournal(workspace.Directory)
	if err != nil || journal.Projection.LastSequence != 1 {
		t.Fatalf("controller process A journal=%+v err=%v", journal, err)
	}
	client := redis.NewClient(&redis.Options{Addr: redisAddr})
	t.Cleanup(func() { _ = client.Close() })
	ownership, err := NewRedisCutoverControllerOwnership(client, key)
	if err != nil {
		t.Fatal(err)
	}
	if acquired, err := ownership.Acquire(context.Background(), "process-b", 5*time.Second); err != nil || acquired {
		t.Fatalf("pre-expiry process B acquire=%v err=%v", acquired, err)
	}
	fastForward()
	commandB := runHelper("process-b", "resume", f.now.Add(2*time.Second))
	if outputB, err := commandB.CombinedOutput(); err != nil {
		t.Fatalf("controller process B err=%v output=%s", err, outputB)
	}
	journal, err = LoadCutoverAttemptJournal(workspace.Directory)
	if err != nil || journal.Projection.State != CutoverAttemptCompleted || journal.Projection.LastSequence != 6 {
		t.Fatalf("controller process B journal=%+v err=%v", journal, err)
	}
	if reportPath := os.Getenv("DIPOLE_CUTOVER_CONTROLLER_DRILL_REPORT"); reportPath != "" {
		report := cutoverControllerDrillReport{
			SchemaVersion: "dipole.realtime.cutover-controller-drill.v1",
			GitRevision:   os.Getenv("DIPOLE_CUTOVER_DRILL_REVISION"), RedisMode: redisMode,
			AttemptID: workspace.Journal.Manifest.AttemptID, ProcessAExitCode: 91, ProcessASequence: 1,
			PreExpiryBlocked: true, ProcessBResumed: true, FinalState: string(journal.Projection.State),
			FinalSequence: journal.Projection.LastSequence, FinalJournalHeadSHA: journal.HeadSHA256,
			ControlLeaseTTLMS: int64((5 * time.Second) / time.Millisecond), CompletedAtUnixMS: time.Now().UnixMilli(),
		}
		payload, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(reportPath, append(payload, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func runCutoverControllerProcessHelper(t *testing.T) {
	workspace, err := LoadCutoverAttemptWorkspace(os.Getenv("DIPOLE_CUTOVER_CONTROLLER_WORKSPACE"))
	if err != nil {
		t.Fatal(err)
	}
	nowMS, err := strconv.ParseInt(os.Getenv("DIPOLE_CUTOVER_CONTROLLER_NOW"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	now := time.UnixMilli(nowMS)
	client := redis.NewClient(&redis.Options{Addr: os.Getenv("DIPOLE_CUTOVER_CONTROLLER_REDIS")})
	defer client.Close()
	ownership, err := NewRedisCutoverControllerOwnership(client, os.Getenv("DIPOLE_CUTOVER_CONTROLLER_KEY"))
	if err != nil {
		t.Fatal(err)
	}
	authorityLease, err := NewCutoverWorkspaceAuthorityLease(workspace)
	if err != nil {
		t.Fatal(err)
	}
	executor := &controllerArtifactExecutor{artifacts: workspace.Artifacts, now: now}
	orchestrator, err := NewCutoverAttemptOrchestrator(workspace.Journal, executor, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	var controllerOrchestrator CutoverControllerOrchestrator = orchestrator
	if os.Getenv("DIPOLE_CUTOVER_CONTROLLER_MODE") == "crash" {
		controllerOrchestrator = exitAfterAdvanceOrchestrator{CutoverControllerOrchestrator: orchestrator}
	}
	controller, err := NewCutoverAttemptController(CutoverAttemptControllerConfig{
		Orchestrator: controllerOrchestrator, Ownership: ownership, AuthorityLease: authorityLease,
		OwnerID: os.Getenv("DIPOLE_CUTOVER_CONTROLLER_OWNER"), OwnershipTTL: 5 * time.Second,
		ActionTimeout: 2 * time.Second, RenewBefore: time.Second, RetryInterval: time.Millisecond,
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := controller.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Terminal {
		t.Fatal(fmt.Errorf("controller helper did not reach terminal state: %+v", result))
	}
}
