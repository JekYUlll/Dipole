package delivery

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

type productionObservationAggregatorStub struct {
	now   *time.Time
	calls []FenceTransitionReceipt
}

func (s *productionObservationAggregatorStub) Aggregate(
	_ context.Context,
	manifest FenceExpectedNodeManifest,
	transition FenceTransitionReceipt,
) (FenceObservationAggregateReceipt, error) {
	s.calls = append(s.calls, transition)
	nodes, manifestSHA, err := validateExpectedNodeManifest(manifest)
	if err != nil {
		return FenceObservationAggregateReceipt{}, err
	}
	now := s.now.UTC()
	observations := make([]FenceObservation, 0, len(nodes))
	for _, node := range nodes {
		expected := node.ExpectedAuthority
		if expected == "" {
			expected = transition.Authority
		}
		observation := validAggregateObservation(now, transition, node.Component, node.ObserverID)
		observation.ExpectedAuthority = expected
		if transition.Phase == FencePhaseFrozen {
			observation.Status = FenceObservationDenied
			observation.ReasonCode = FenceReasonFrozen
		}
		observations = append(observations, observation)
	}
	return FenceObservationAggregateReceipt{
		SchemaVersion: FenceObservationAggregateReceiptSchemaV1, Decision: FenceObservationAggregateEligible,
		ManifestID: manifest.ManifestID, ManifestSHA256: manifestSHA,
		TransitionID: transition.TransitionID, RequestSHA256: transition.RequestSHA256,
		LeaseSHA256: transition.NextSHA256, Authority: transition.Authority, Phase: transition.Phase,
		Epoch: transition.Epoch, LeaseUntilUnixMS: transition.LeaseUntilUnixMS,
		CapturedAtUnixMS: now.UnixMilli(), Observations: observations,
	}, nil
}

type productionExecutorFixture struct {
	now         time.Time
	server      *miniredis.Miniredis
	client      *redis.Client
	writer      *RedisAuthorityFenceWriter
	aggregator  *productionObservationAggregatorStub
	artifacts   *CutoverActionArtifactStore
	artifactDir string
	config      ProductionCutoverExecutorConfig
	executor    *ProductionCutoverAttemptExecutor
}

func newProductionExecutorFixture(t *testing.T) *productionExecutorFixture {
	t.Helper()
	f := &productionExecutorFixture{now: time.Date(2026, 8, 28, 11, 0, 0, 0, time.UTC)}
	f.server = miniredis.RunT(t)
	f.server.SetTime(f.now)
	f.client = redis.NewClient(&redis.Options{Addr: f.server.Addr()})
	t.Cleanup(func() { _ = f.client.Close() })
	var err error
	f.writer, err = NewRedisAuthorityFenceWriter(
		f.client, "cutover:fence", "cutover:fence:receipt:", 24*time.Hour, func() time.Time { return f.now },
	)
	if err != nil {
		t.Fatal(err)
	}
	initial, err := f.writer.Apply(context.Background(), FenceTransitionRequest{
		TransitionID: "initial-go", Action: FenceTransitionBootstrap, OperatorID: "operator-a", Reason: "initial source",
		TargetAuthority: AuthorityGo, LeaseUntil: f.now.Add(30 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	sourceNodes := productionNodes("source-nodes", AuthorityGo)
	frozenNodes := productionNodes("frozen-nodes", AuthorityCPP)
	targetNodes := productionNodes("target-nodes", AuthorityCPP)
	checkpointManifest := validCheckpointManifest()
	_, sourceSHA, _ := validateExpectedNodeManifest(sourceNodes)
	_, frozenSHA, _ := validateExpectedNodeManifest(frozenNodes)
	_, targetSHA, _ := validateExpectedNodeManifest(targetNodes)
	_, checkpointSHA, _ := validateDualGroupCheckpointManifest(checkpointManifest)
	manifest := CutoverAttemptManifest{
		SchemaVersion: CutoverAttemptManifestSchemaV1, AttemptID: "production-a",
		SourceAuthority: AuthorityGo, TargetAuthority: AuthorityCPP, InitialEpoch: 1,
		InitialLeaseSHA256: initial.NextSHA256,
		MaxInterruptionMS:  60_000, CreatedAtUnixMS: f.now.UnixMilli(),
		SourceNodesManifestSHA256: sourceSHA, FrozenNodesManifestSHA256: frozenSHA,
		TargetNodesManifestSHA256: targetSHA, CheckpointManifestSHA256: checkpointSHA,
	}
	f.config = ProductionCutoverExecutorConfig{
		Manifest: manifest, InitialTransition: initial,
		SourceNodes: sourceNodes, FrozenNodes: frozenNodes, TargetNodes: targetNodes,
		CheckpointManifest: checkpointManifest, OperatorID: "operator-a", LeaseDuration: 10 * time.Minute,
		Now: func() time.Time { return f.now },
	}
	f.aggregator = &productionObservationAggregatorStub{now: &f.now}
	collector, err := NewDualGroupCheckpointCollector(checkpointSourceStub{snapshot: validCheckpointSnapshot()}, func() time.Time { return f.now })
	if err != nil {
		t.Fatal(err)
	}
	f.artifactDir = t.TempDir()
	f.artifacts, err = NewCutoverActionArtifactStore(f.artifactDir)
	if err != nil {
		t.Fatal(err)
	}
	f.executor, err = NewProductionCutoverAttemptExecutor(f.config, f.writer, f.aggregator, collector, f.artifacts)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestProductionCutoverExecutorRenewsAndRebindsFollowingCheckpoint(t *testing.T) {
	f := newProductionExecutorFixture(t)
	journal, err := CreateCutoverAttemptJournal(t.TempDir(), f.config.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	orchestrator, err := NewCutoverAttemptOrchestrator(journal, f.executor, func() time.Time { return f.now })
	if err != nil {
		t.Fatal(err)
	}
	for range 4 {
		f.now = f.now.Add(time.Second)
		if _, err := orchestrator.Advance(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	f.now = f.now.Add(time.Second)
	renewed, err := orchestrator.RenewLease(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if renewed.State != CutoverAttemptTargetActivated || renewed.EventType != CutoverEventLeaseRenewed {
		t.Fatalf("renewed=%+v", renewed)
	}
	renewID, _ := cutoverAttemptActionID(f.config.Manifest.AttemptID, 5, CutoverEventLeaseRenewed)
	receipt, err := f.writer.GetReceipt(context.Background(), renewID)
	if err != nil || receipt.Action != FenceTransitionRenew || receipt.Authority != AuthorityCPP || receipt.Epoch != 2 {
		t.Fatalf("renew receipt=%+v err=%v", receipt, err)
	}
	f.now = f.now.Add(time.Second)
	if _, err := orchestrator.Advance(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := f.aggregator.calls[len(f.aggregator.calls)-1].TransitionID; got != renewID {
		t.Fatalf("checkpoint transition=%s, want %s", got, renewID)
	}
}

func TestProductionCutoverExecutorDoesNotAdoptUnjournaledRenewal(t *testing.T) {
	f := newProductionExecutorFixture(t)
	f.config.LeaseDuration = 40 * time.Minute
	collector, err := NewDualGroupCheckpointCollector(checkpointSourceStub{snapshot: validCheckpointSnapshot()}, func() time.Time { return f.now })
	if err != nil {
		t.Fatal(err)
	}
	f.executor, err = NewProductionCutoverAttemptExecutor(f.config, f.writer, f.aggregator, collector, f.artifacts)
	if err != nil {
		t.Fatal(err)
	}
	journal, err := CreateCutoverAttemptJournal(t.TempDir(), f.config.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	orchestrator, err := NewCutoverAttemptOrchestrator(journal, f.executor, func() time.Time { return f.now })
	if err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(time.Second)
	if _, err := orchestrator.Advance(context.Background()); err != nil {
		t.Fatal(err)
	}
	orphan, err := orchestrator.action(CutoverEventLeaseRenewed)
	if err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(time.Second)
	if _, err := f.executor.Execute(context.Background(), orphan); err != nil {
		t.Fatal(err)
	}
	if _, err := orchestrator.Advance(context.Background()); err == nil {
		t.Fatal("advance must not adopt a renewal absent from the journal")
	}
	if journal.Projection.State != CutoverAttemptSourceCheckpointed || journal.Projection.LastSequence != 1 {
		t.Fatalf("failed advance projection=%+v", journal.Projection)
	}
	result, err := orchestrator.RenewLease(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.EventType != CutoverEventLeaseRenewed || result.State != CutoverAttemptCreated || result.Sequence != 2 {
		t.Fatalf("recovered renewal=%+v", result)
	}
}

func TestProductionCutoverExecutorCompletesForwardPath(t *testing.T) {
	f := newProductionExecutorFixture(t)
	journal, err := CreateCutoverAttemptJournal(t.TempDir(), f.config.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	orchestrator, err := NewCutoverAttemptOrchestrator(journal, f.executor, func() time.Time { return f.now })
	if err != nil {
		t.Fatal(err)
	}
	for range 6 {
		f.now = f.now.Add(time.Second)
		if _, err := orchestrator.Advance(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if journal.Projection.State != CutoverAttemptCompleted || len(f.aggregator.calls) != 3 {
		t.Fatalf("forward projection=%+v aggregate calls=%d", journal.Projection, len(f.aggregator.calls))
	}
	freezeID, _ := cutoverAttemptActionID(f.config.Manifest.AttemptID, 2, CutoverEventFreezeApplied)
	freeze, err := f.writer.GetReceipt(context.Background(), freezeID)
	if err != nil || freeze.Phase != FencePhaseFrozen || freeze.Epoch != 2 {
		t.Fatalf("freeze receipt=%+v err=%v", freeze, err)
	}
	activateID, _ := cutoverAttemptActionID(f.config.Manifest.AttemptID, 4, CutoverEventTargetActivated)
	activate, err := f.writer.GetReceipt(context.Background(), activateID)
	if err != nil || activate.Authority != AuthorityCPP || activate.Phase != FencePhaseActive || activate.Epoch != 2 {
		t.Fatalf("activate receipt=%+v err=%v", activate, err)
	}
}

func TestProductionCutoverExecutorRecoversReceiptWhenArtifactWasNotPublished(t *testing.T) {
	f := newProductionExecutorFixture(t)
	journal, err := CreateCutoverAttemptJournal(t.TempDir(), f.config.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	orchestrator, err := NewCutoverAttemptOrchestrator(journal, f.executor, func() time.Time { return f.now })
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		f.now = f.now.Add(time.Second)
		if _, err := orchestrator.Advance(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	actionID, _ := cutoverAttemptActionID(f.config.Manifest.AttemptID, 2, CutoverEventFreezeApplied)
	artifact, digest, err := f.artifacts.LoadByActionID(actionID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(f.artifactDir, actionID+cutoverActionArtifactSuffix)); err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(time.Minute)
	recovered, err := f.executor.Execute(context.Background(), artifact.Action)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.ArtifactSHA256 != digest || recovered.RecordedAtUnixMS != artifact.RecordedAtUnixMS {
		t.Fatalf("recovered result=%+v want digest=%s time=%d", recovered, digest, artifact.RecordedAtUnixMS)
	}
	receipt, err := f.writer.GetReceipt(context.Background(), actionID)
	if err != nil || receipt.Epoch != 2 {
		t.Fatalf("recovered receipt=%+v err=%v", receipt, err)
	}
}

func TestProductionCutoverExecutorCompletesSecondFreezeRollback(t *testing.T) {
	f := newProductionExecutorFixture(t)
	journal, err := CreateCutoverAttemptJournal(t.TempDir(), f.config.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	orchestrator, err := NewCutoverAttemptOrchestrator(journal, f.executor, func() time.Time { return f.now })
	if err != nil {
		t.Fatal(err)
	}
	for range 4 {
		f.now = f.now.Add(time.Second)
		if _, err := orchestrator.Advance(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := orchestrator.RequestRollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	for !terminalCutoverAttemptState(journal.Projection.State) {
		f.now = f.now.Add(time.Second)
		if _, err := orchestrator.Advance(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if journal.Projection.State != CutoverAttemptRolledBack {
		t.Fatalf("rollback projection=%+v", journal.Projection)
	}
	recordPayload, err := f.client.Get(context.Background(), "cutover:fence").Bytes()
	if err != nil {
		t.Fatal(err)
	}
	record, err := decodeFenceRecord(recordPayload)
	if err != nil {
		t.Fatal(err)
	}
	if record.Authority != AuthorityGo || record.Phase != FencePhaseActive || record.Epoch != 3 {
		t.Fatalf("rollback fence=%+v", record)
	}
}

func TestProductionCutoverExecutorReactivatesSourceFromInitialFreeze(t *testing.T) {
	f := newProductionExecutorFixture(t)
	journal, err := CreateCutoverAttemptJournal(t.TempDir(), f.config.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	orchestrator, err := NewCutoverAttemptOrchestrator(journal, f.executor, func() time.Time { return f.now })
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		f.now = f.now.Add(time.Second)
		if _, err := orchestrator.Advance(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := orchestrator.RequestRollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	for !terminalCutoverAttemptState(journal.Projection.State) {
		f.now = f.now.Add(time.Second)
		if _, err := orchestrator.Advance(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	recordPayload, err := f.client.Get(context.Background(), "cutover:fence").Bytes()
	if err != nil {
		t.Fatal(err)
	}
	record, err := decodeFenceRecord(recordPayload)
	if err != nil {
		t.Fatal(err)
	}
	if journal.Projection.State != CutoverAttemptRolledBack || record.Authority != AuthorityGo || record.Epoch != 2 {
		t.Fatalf("direct rollback projection=%+v fence=%+v", journal.Projection, record)
	}
}

func TestProductionCutoverExecutorRejectsStateEventDrift(t *testing.T) {
	f := newProductionExecutorFixture(t)
	action := validCutoverArtifactAction(CutoverEventTargetActivated)
	action.AttemptID = f.config.Manifest.AttemptID
	action.SourceAuthority = f.config.Manifest.SourceAuthority
	action.TargetAuthority = f.config.Manifest.TargetAuthority
	action.CurrentState = CutoverAttemptCreated
	action.Sequence = 1
	action.ExpectedEpoch = 2
	action.MaxInterruptionMS = f.config.Manifest.MaxInterruptionMS
	action.ActionID, _ = cutoverAttemptActionID(action.AttemptID, action.Sequence, action.EventType)
	if _, err := f.executor.Execute(context.Background(), action); err == nil {
		t.Fatal("state/event drift must fail before side effects")
	}
}

func TestProductionCutoverExecutorRejectsManifestHashDrift(t *testing.T) {
	f := newProductionExecutorFixture(t)
	config := f.config
	config.Manifest.TargetNodesManifestSHA256 = hashBytes([]byte("drift"))
	collector, err := NewDualGroupCheckpointCollector(checkpointSourceStub{snapshot: validCheckpointSnapshot()}, func() time.Time { return f.now })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewProductionCutoverAttemptExecutor(config, f.writer, f.aggregator, collector, f.artifacts); err == nil {
		t.Fatal("manifest hash drift must fail")
	}
}

func productionNodes(manifestID string, authority Authority) FenceExpectedNodeManifest {
	return FenceExpectedNodeManifest{
		SchemaVersion: FenceExpectedNodeManifestSchemaV1, ManifestID: manifestID,
		Nodes: []FenceExpectedNode{{Component: "gateway", ObserverID: "gateway-a", ExpectedAuthority: authority}},
	}
}
