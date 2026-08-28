//go:build integration

package delivery

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
)

type cutoverFaultDrillReport struct {
	SchemaVersion            string              `json:"schema_version"`
	GitRevision              string              `json:"git_revision"`
	RedisImage               string              `json:"redis_image"`
	KafkaImage               string              `json:"kafka_image"`
	KafkaClusterID           string              `json:"kafka_cluster_id"`
	AttemptID                string              `json:"attempt_id"`
	ControllerCrashArtifact  string              `json:"controller_crash_artifact_sha256"`
	ControllerCrashRecovered bool                `json:"controller_crash_recovered"`
	RedisOutageBlocked       bool                `json:"redis_outage_blocked"`
	KafkaRebalanceBlocked    bool                `json:"kafka_rebalance_blocked"`
	CPPPrimaryReady          bool                `json:"cpp_primary_ready"`
	CPPPrimaryStoppedCleanly bool                `json:"cpp_primary_stopped_cleanly"`
	CPPPrimaryBinarySHA256   string              `json:"cpp_primary_binary_sha256"`
	CPPPrimaryInstanceID     string              `json:"cpp_primary_instance_id"`
	CPPPrimaryGroupID        string              `json:"cpp_primary_group_id"`
	CPPPrimaryObservationKey string              `json:"cpp_primary_observation_key"`
	CPPPrimaryObservationSHA string              `json:"cpp_primary_observation_sha256"`
	ExpiredFreezeRolledBack  bool                `json:"expired_freeze_rolled_back"`
	RollbackFinalSequence    uint64              `json:"rollback_final_sequence"`
	RollbackJournalHead      string              `json:"rollback_journal_head_sha256"`
	FinalState               CutoverAttemptState `json:"final_state"`
	FinalSequence            uint64              `json:"final_sequence"`
	FinalJournalHeadSHA256   string              `json:"final_journal_head_sha256"`
	CompletedAtUnixMS        int64               `json:"completed_at_unix_ms"`
}

type cutoverFaultProxy struct {
	listener net.Listener
	upstream string
	mu       sync.Mutex
	enabled  bool
	conns    map[net.Conn]struct{}
}

func newCutoverFaultProxy(t *testing.T, upstream string) *cutoverFaultProxy {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := &cutoverFaultProxy{listener: listener, upstream: upstream, enabled: true, conns: make(map[net.Conn]struct{})}
	go p.serve()
	t.Cleanup(p.Close)
	return p
}

func (p *cutoverFaultProxy) Addr() string { return p.listener.Addr().String() }

func (p *cutoverFaultProxy) SetEnabled(enabled bool) {
	p.mu.Lock()
	p.enabled = enabled
	if !enabled {
		for conn := range p.conns {
			_ = conn.Close()
		}
	}
	p.mu.Unlock()
}

func (p *cutoverFaultProxy) Close() {
	_ = p.listener.Close()
	p.SetEnabled(false)
}

func (p *cutoverFaultProxy) serve() {
	for {
		downstream, err := p.listener.Accept()
		if err != nil {
			return
		}
		p.mu.Lock()
		enabled := p.enabled
		p.mu.Unlock()
		if !enabled {
			_ = downstream.Close()
			continue
		}
		upstream, err := net.DialTimeout("tcp", p.upstream, time.Second)
		if err != nil {
			_ = downstream.Close()
			continue
		}
		p.track(downstream, true)
		p.track(upstream, true)
		go p.pipe(downstream, upstream)
		go p.pipe(upstream, downstream)
	}
}

func (p *cutoverFaultProxy) pipe(dst, src net.Conn) {
	_, _ = io.Copy(dst, src)
	_ = dst.Close()
	_ = src.Close()
	p.track(dst, false)
	p.track(src, false)
}

func (p *cutoverFaultProxy) track(conn net.Conn, add bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if add {
		p.conns[conn] = struct{}{}
	} else {
		delete(p.conns, conn)
	}
}

type cutoverDrillAggregator struct {
	client                      redis.Cmdable
	key                         string
	now                         func() time.Time
	inner                       *RedisFenceObservationAggregator
	externalObservationManifest string
}

func (a cutoverDrillAggregator) Aggregate(ctx context.Context, manifest FenceExpectedNodeManifest, transition FenceTransitionReceipt) (FenceObservationAggregateReceipt, error) {
	for _, node := range manifest.Nodes {
		if manifest.ManifestID == a.externalObservationManifest && node.Component == "realtime-delivery" {
			continue
		}
		reader, err := NewRedisAuthorityFence(a.client, a.key, transition.Epoch, a.now)
		if err != nil {
			return FenceObservationAggregateReceipt{}, err
		}
		observer, err := NewRedisObservedAuthorityFence(reader, a.client, a.key+":observation:", node.Component, node.ObserverID, 30*time.Second, a.now)
		if err != nil {
			return FenceObservationAggregateReceipt{}, err
		}
		err = observer.Assert(ctx, node.ExpectedAuthority)
		if transition.Phase != FencePhaseFrozen && err != nil {
			return FenceObservationAggregateReceipt{}, err
		}
	}
	return a.inner.Aggregate(ctx, manifest, transition)
}

type cutoverCPPPrimary struct {
	cmd            *exec.Cmd
	output         bytes.Buffer
	healthAddress  string
	evidencePath   string
	instanceID     string
	groupID        string
	observationKey string
	binarySHA256   string
	done           chan error
}

type cutoverDrillGroup struct {
	reader *kafka.Reader
	done   chan struct{}
}

func startCutoverDrillGroup(t *testing.T, broker, groupID string, topics []string) *cutoverDrillGroup {
	t.Helper()
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{broker}, GroupID: groupID, GroupTopics: topics,
		MinBytes: 1, MaxBytes: 1 << 20, MaxWait: 200 * time.Millisecond,
	})
	g := &cutoverDrillGroup{reader: reader, done: make(chan struct{})}
	go func() {
		defer close(g.done)
		for {
			if _, err := reader.ReadMessage(context.Background()); err != nil {
				return
			}
		}
	}()
	return g
}

func (g *cutoverDrillGroup) Close() {
	_ = g.reader.Close()
	<-g.done
}

func startCutoverCPPPrimary(
	t *testing.T,
	ctx context.Context,
	binary, goldenDir, kafkaAddr, redisAddr, groupID, fenceKey, instanceID string,
	epoch uint64,
) *cutoverCPPPrimary {
	t.Helper()
	binaryPayload, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	healthListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	healthAddress := healthListener.Addr().String()
	_ = healthListener.Close()
	_, healthPort, err := net.SplitHostPort(healthAddress)
	if err != nil {
		t.Fatal(err)
	}
	unusedNodeListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	unusedNodeTarget := unusedNodeListener.Addr().String()
	_ = unusedNodeListener.Close()

	process := &cutoverCPPPrimary{
		healthAddress:  healthAddress,
		evidencePath:   filepath.Join(t.TempDir(), "cpp-primary.ndjson"),
		instanceID:     instanceID,
		groupID:        groupID,
		observationKey: fenceKey + ":observation:realtime-delivery:" + instanceID,
		binarySHA256:   fmt.Sprintf("%x", sha256.Sum256(binaryPayload)),
		done:           make(chan error, 1),
	}
	process.cmd = exec.CommandContext(ctx, binary, "primary", goldenDir)
	process.cmd.Env = append(os.Environ(),
		"DIPOLE_REALTIME_DELIVERY=cpp",
		"DIPOLE_REALTIME_PRIMARY_ENABLED=true",
		"DIPOLE_REALTIME_HOST=127.0.0.1",
		"DIPOLE_REALTIME_PORT="+healthPort,
		"DIPOLE_REALTIME_KAFKA_BROKERS="+kafkaAddr,
		"DIPOLE_REALTIME_KAFKA_CLIENT_ID="+instanceID,
		"DIPOLE_REALTIME_KAFKA_GROUP_ID="+groupID,
		"DIPOLE_REALTIME_EVIDENCE_FILE="+process.evidencePath,
		"DIPOLE_REALTIME_POLL_TIMEOUT_MS=50",
		"DIPOLE_REALTIME_ERROR_BACKOFF_MS=50",
		"DIPOLE_REALTIME_PRESENCE_MODE=primary",
		"DIPOLE_REALTIME_REDIS_ENDPOINT="+redisAddr,
		"DIPOLE_REALTIME_REDIS_TIMEOUT_MS=500",
		"DIPOLE_REALTIME_FENCING_ENABLED=true",
		"DIPOLE_REALTIME_FENCING_KEY="+fenceKey,
		"DIPOLE_REALTIME_FENCING_EPOCH="+strconv.FormatUint(epoch, 10),
		"DIPOLE_REALTIME_INSTANCE_ID="+instanceID,
		"DIPOLE_REALTIME_NODE_TRANSPORT_MODE=primary",
		"DIPOLE_REALTIME_NODE_TARGETS=gateway-a="+unusedNodeTarget,
		"DIPOLE_INTERNAL_RPC_SHARED_SECRET=cutover-fault-drill",
	)
	process.cmd.Stdout = &process.output
	process.cmd.Stderr = &process.output
	if err := process.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { process.done <- process.cmd.Wait() }()
	waitCutoverCPPPrimaryReady(t, ctx, process)
	return process
}

func waitCutoverCPPPrimaryReady(t *testing.T, ctx context.Context, process *cutoverCPPPrimary) {
	t.Helper()
	waitCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 500 * time.Millisecond}
	for {
		select {
		case err := <-process.done:
			t.Fatalf("C++ primary exited before readiness: %v\n%s", err, process.output.String())
		default:
		}
		request, err := http.NewRequestWithContext(waitCtx, http.MethodGet, "http://"+process.healthAddress+"/readyz", nil)
		if err != nil {
			t.Fatal(err)
		}
		response, err := client.Do(request)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
		select {
		case <-waitCtx.Done():
			t.Fatalf("wait for C++ primary readiness: %v\n%s", waitCtx.Err(), process.output.String())
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (p *cutoverCPPPrimary) Stop() error {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return nil
	}
	if err := p.cmd.Process.Signal(os.Interrupt); err != nil {
		return err
	}
	select {
	case err := <-p.done:
		return err
	case <-time.After(5 * time.Second):
		_ = p.cmd.Process.Kill()
		<-p.done
		return fmt.Errorf("C++ primary did not stop within deadline")
	}
}

func TestRealtimeCutoverFaultDrill(t *testing.T) {
	redisAddr := os.Getenv("DIPOLE_CUTOVER_DRILL_REDIS_ADDR")
	kafkaAddr := os.Getenv("DIPOLE_CUTOVER_DRILL_KAFKA_ADDR")
	reportPath := os.Getenv("DIPOLE_CUTOVER_DRILL_REPORT")
	cppBinary := os.Getenv("DIPOLE_CUTOVER_DRILL_CPP_BINARY")
	goldenDir := os.Getenv("DIPOLE_CUTOVER_DRILL_GOLDEN_DIR")
	if redisAddr == "" || kafkaAddr == "" || reportPath == "" || cppBinary == "" || goldenDir == "" {
		t.Skip("isolated cutover drill endpoints are not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	proxy := newCutoverFaultProxy(t, redisAddr)
	client := redis.NewClient(&redis.Options{Addr: proxy.Addr(), DialTimeout: time.Second, ReadTimeout: time.Second, WriteTimeout: time.Second})
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatal(err)
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	topics := []string{"dipole.message.direct.created", "dipole.message.group.created"}
	groups := []string{"dipole-c3-compat-" + suffix, "dipole-realtime-primary-fault-" + suffix}
	createCutoverDrillTopics(t, kafkaAddr, topics)
	writer := &kafka.Writer{Addr: kafka.TCP(kafkaAddr), RequiredAcks: kafka.RequireAll}
	for _, topic := range topics {
		if err := writer.WriteMessages(ctx, kafka.Message{Topic: topic, Value: []byte(`{"fixture":true}`)}); err != nil {
			t.Fatal(err)
		}
	}
	_ = writer.Close()
	compat := startCutoverDrillGroup(t, kafkaAddr, groups[0], topics)
	defer compat.Close()
	primary := startCutoverDrillGroup(t, kafkaAddr, groups[1], topics)
	defer func() {
		if primary != nil {
			primary.Close()
		}
	}()

	now := time.Now
	key := "dipole:c3:fault:" + suffix
	fenceWriter, err := NewRedisAuthorityFenceWriter(client, key, key+":receipt:", time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	initial, err := fenceWriter.Apply(ctx, FenceTransitionRequest{
		TransitionID: "initial-" + suffix, Action: FenceTransitionBootstrap, OperatorID: "fault-drill",
		Reason: "isolated fault drill", TargetAuthority: AuthorityGo, LeaseUntil: now().Add(5 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}

	sourceNodes := cutoverDrillNodes("source-"+suffix, AuthorityGo)
	frozenNodes := cutoverDrillNodes("frozen-"+suffix, AuthorityCPP)
	targetNodes := cutoverDrillNodes("target-"+suffix, AuthorityCPP)
	checkpointManifest := DualGroupCheckpointManifest{
		SchemaVersion: DualGroupCheckpointManifestSchemaV1, ManifestID: "checkpoint-" + suffix,
		Topics: topics,
		Groups: []KafkaCheckpointGroupSpec{
			{Role: KafkaCheckpointRoleCompatibility, GroupID: groups[0]},
			{Role: KafkaCheckpointRolePrimary, GroupID: groups[1]},
		},
	}
	_, sourceSHA, _ := validateExpectedNodeManifest(sourceNodes)
	_, frozenSHA, _ := validateExpectedNodeManifest(frozenNodes)
	_, targetSHA, _ := validateExpectedNodeManifest(targetNodes)
	_, checkpointSHA, _ := validateDualGroupCheckpointManifest(checkpointManifest)
	manifest := CutoverAttemptManifest{
		SchemaVersion: CutoverAttemptManifestSchemaV1, AttemptID: "fault-" + suffix,
		SourceAuthority: AuthorityGo, TargetAuthority: AuthorityCPP, InitialEpoch: 1,
		InitialLeaseSHA256: initial.NextSHA256, MaxInterruptionMS: 60_000, CreatedAtUnixMS: now().UnixMilli(),
		SourceNodesManifestSHA256: sourceSHA, FrozenNodesManifestSHA256: frozenSHA,
		TargetNodesManifestSHA256: targetSHA, CheckpointManifestSHA256: checkpointSHA,
	}
	journal, err := CreateCutoverAttemptJournal(filepath.Join(t.TempDir(), "attempt"), manifest)
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := NewCutoverActionArtifactStore(filepath.Join(journal.Directory, "artifacts"))
	if err != nil {
		t.Fatal(err)
	}
	innerAggregator, err := NewRedisFenceObservationAggregator(client, key+":observation:", now)
	if err != nil {
		t.Fatal(err)
	}
	aggregator := cutoverDrillAggregator{
		client: client, key: key, now: now, inner: innerAggregator,
		externalObservationManifest: targetNodes.ManifestID,
	}
	source, err := NewKafkaGoCheckpointSource([]string{kafkaAddr}, "c3-fault-drill", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	collector, err := NewDualGroupCheckpointCollector(source, now)
	if err != nil {
		t.Fatal(err)
	}
	executor, err := NewProductionCutoverAttemptExecutor(ProductionCutoverExecutorConfig{
		Manifest: manifest, InitialTransition: initial, SourceNodes: sourceNodes, FrozenNodes: frozenNodes,
		TargetNodes: targetNodes, CheckpointManifest: checkpointManifest, OperatorID: "fault-drill",
		LeaseDuration: 2 * time.Minute, Now: now,
	}, fenceWriter, aggregator, collector, artifacts)
	if err != nil {
		t.Fatal(err)
	}
	orchestrator, err := NewCutoverAttemptOrchestrator(journal, executor, now)
	if err != nil {
		t.Fatal(err)
	}
	stableSnapshot := waitCutoverDrillGroups(t, ctx, source, checkpointManifest)

	crashAction, err := orchestrator.action(CutoverEventSourceCheckpointed)
	if err != nil {
		t.Fatal(err)
	}
	crashResult, err := executor.Execute(ctx, crashAction)
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := LoadCutoverAttemptJournal(journal.Directory)
	if err != nil {
		t.Fatal(err)
	}
	orchestrator, err = NewCutoverAttemptOrchestrator(reloaded, executor, now)
	if err != nil {
		t.Fatal(err)
	}
	if result, err := orchestrator.Advance(ctx); err != nil || result.State != CutoverAttemptSourceCheckpointed {
		t.Fatalf("recover controller crash: result=%+v err=%v", result, err)
	}
	for range 3 {
		if _, err := orchestrator.Advance(ctx); err != nil {
			t.Fatal(err)
		}
	}

	beforeOutage := reloaded.Projection.LastSequence
	proxy.SetEnabled(false)
	outageCtx, outageCancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	_, outageErr := orchestrator.RenewLease(outageCtx)
	outageCancel()
	if outageErr == nil {
		t.Fatal("Redis outage must block renewal")
	}
	current, err := LoadCutoverAttemptJournal(journal.Directory)
	if err != nil {
		t.Fatal(err)
	}
	if current.Projection.LastSequence != beforeOutage {
		t.Fatal("Redis outage advanced the journal")
	}
	proxy.SetEnabled(true)
	if _, err := orchestrator.RenewLease(ctx); err != nil {
		t.Fatal(err)
	}

	primary.Close()
	primary = nil
	waitCutoverDrillGroupBlocked(t, ctx, source, checkpointManifest)
	beforeRebalance := current.Projection.LastSequence + 1
	if _, err := orchestrator.Advance(ctx); err == nil {
		t.Fatal("missing primary group must block target checkpoint")
	}
	current, err = LoadCutoverAttemptJournal(journal.Directory)
	if err != nil {
		t.Fatal(err)
	}
	if current.Projection.LastSequence != beforeRebalance {
		t.Fatal("Kafka rebalance failure advanced the journal")
	}
	cppPrimary := startCutoverCPPPrimary(
		t, ctx, cppBinary, goldenDir, kafkaAddr, proxy.Addr(), groups[1], key, "cpp-a", manifest.InitialEpoch+1,
	)
	defer func() {
		if cppPrimary != nil {
			_ = cppPrimary.Stop()
		}
	}()
	stableSnapshot = waitCutoverDrillGroups(t, ctx, source, checkpointManifest)
	if _, err := orchestrator.Advance(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := orchestrator.Advance(ctx); err != nil {
		t.Fatal(err)
	}
	current, err = LoadCutoverAttemptJournal(journal.Directory)
	if err != nil {
		t.Fatal(err)
	}
	cppObservationPayload, err := client.Get(ctx, cppPrimary.observationKey).Bytes()
	if err != nil {
		t.Fatal(err)
	}
	var cppObservation FenceObservation
	if err := json.Unmarshal(cppObservationPayload, &cppObservation); err != nil {
		t.Fatal(err)
	}
	if cppObservation.Status != FenceObservationAuthorized || cppObservation.ObserverID != cppPrimary.instanceID ||
		cppObservation.Component != "realtime-delivery" || cppObservation.ExpectedAuthority != AuthorityCPP ||
		cppObservation.ObservedAuthority != AuthorityCPP || cppObservation.ExpectedEpoch != manifest.InitialEpoch+1 ||
		cppObservation.ObservedEpoch != manifest.InitialEpoch+1 || cppObservation.ObservedPhase != FencePhaseActive {
		t.Fatalf("C++ primary observation identity drifted: %+v", cppObservation)
	}
	cppObservationSHA := fmt.Sprintf("%x", sha256.Sum256(cppObservationPayload))
	cppStoppedCleanly := cppPrimary.Stop() == nil
	if !cppStoppedCleanly {
		t.Fatalf("stop C++ primary:\n%s", cppPrimary.output.String())
	}
	cppPrimaryForReport := cppPrimary
	cppPrimary = nil
	primary = startCutoverDrillGroup(t, kafkaAddr, groups[1], topics)
	stableSnapshot = waitCutoverDrillGroups(t, ctx, source, checkpointManifest)
	rollbackJournal := runExpiredFreezeRollbackDrill(t, ctx, client, source, topics, groups, suffix+"-rollback")
	report := cutoverFaultDrillReport{
		SchemaVersion: "dipole.realtime.cutover-fault-drill.v1",
		GitRevision:   os.Getenv("DIPOLE_CUTOVER_DRILL_REVISION"), RedisImage: os.Getenv("DIPOLE_CUTOVER_DRILL_REDIS_IMAGE"),
		KafkaImage: os.Getenv("DIPOLE_CUTOVER_DRILL_KAFKA_IMAGE"), KafkaClusterID: stableSnapshot.ClusterID,
		AttemptID: manifest.AttemptID, ControllerCrashArtifact: crashResult.ArtifactSHA256,
		ControllerCrashRecovered: true, RedisOutageBlocked: true, KafkaRebalanceBlocked: true,
		CPPPrimaryReady: true, CPPPrimaryStoppedCleanly: cppStoppedCleanly,
		CPPPrimaryBinarySHA256: cppPrimaryForReport.binarySHA256,
		CPPPrimaryInstanceID:   cppPrimaryForReport.instanceID, CPPPrimaryGroupID: cppPrimaryForReport.groupID,
		CPPPrimaryObservationKey: cppPrimaryForReport.observationKey,
		CPPPrimaryObservationSHA: cppObservationSHA,
		ExpiredFreezeRolledBack:  true, RollbackFinalSequence: rollbackJournal.Projection.LastSequence,
		RollbackJournalHead: rollbackJournal.HeadSHA256,
		FinalState:          current.Projection.State, FinalSequence: current.Projection.LastSequence,
		FinalJournalHeadSHA256: current.HeadSHA256, CompletedAtUnixMS: now().UnixMilli(),
	}
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(reportPath, append(payload, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func runExpiredFreezeRollbackDrill(
	t *testing.T,
	ctx context.Context,
	client *redis.Client,
	source *KafkaGoCheckpointSource,
	topics, groups []string,
	suffix string,
) *CutoverAttemptJournal {
	t.Helper()
	now := time.Now
	key := "dipole:c3:fault:" + suffix
	writer, err := NewRedisAuthorityFenceWriter(client, key, key+":receipt:", time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	initial, err := writer.Apply(ctx, FenceTransitionRequest{
		TransitionID: "initial-" + suffix, Action: FenceTransitionBootstrap, OperatorID: "fault-drill",
		Reason: "expired freeze rollback drill", TargetAuthority: AuthorityGo, LeaseUntil: now().Add(5 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	sourceNodes := cutoverDrillNodes("source-"+suffix, AuthorityGo)
	frozenNodes := cutoverDrillNodes("frozen-"+suffix, AuthorityCPP)
	targetNodes := cutoverDrillNodes("target-"+suffix, AuthorityCPP)
	checkpointManifest := DualGroupCheckpointManifest{
		SchemaVersion: DualGroupCheckpointManifestSchemaV1, ManifestID: "checkpoint-" + suffix,
		Topics: topics,
		Groups: []KafkaCheckpointGroupSpec{
			{Role: KafkaCheckpointRoleCompatibility, GroupID: groups[0]},
			{Role: KafkaCheckpointRolePrimary, GroupID: groups[1]},
		},
	}
	_, sourceSHA, _ := validateExpectedNodeManifest(sourceNodes)
	_, frozenSHA, _ := validateExpectedNodeManifest(frozenNodes)
	_, targetSHA, _ := validateExpectedNodeManifest(targetNodes)
	_, checkpointSHA, _ := validateDualGroupCheckpointManifest(checkpointManifest)
	manifest := CutoverAttemptManifest{
		SchemaVersion: CutoverAttemptManifestSchemaV1, AttemptID: "fault-" + suffix,
		SourceAuthority: AuthorityGo, TargetAuthority: AuthorityCPP, InitialEpoch: 1,
		InitialLeaseSHA256: initial.NextSHA256, MaxInterruptionMS: 500, CreatedAtUnixMS: now().UnixMilli(),
		SourceNodesManifestSHA256: sourceSHA, FrozenNodesManifestSHA256: frozenSHA,
		TargetNodesManifestSHA256: targetSHA, CheckpointManifestSHA256: checkpointSHA,
	}
	journal, err := CreateCutoverAttemptJournal(filepath.Join(t.TempDir(), "rollback-attempt"), manifest)
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := NewCutoverActionArtifactStore(filepath.Join(journal.Directory, "artifacts"))
	if err != nil {
		t.Fatal(err)
	}
	inner, err := NewRedisFenceObservationAggregator(client, key+":observation:", now)
	if err != nil {
		t.Fatal(err)
	}
	aggregator := cutoverDrillAggregator{client: client, key: key, now: now, inner: inner}
	collector, err := NewDualGroupCheckpointCollector(source, now)
	if err != nil {
		t.Fatal(err)
	}
	executor, err := NewProductionCutoverAttemptExecutor(ProductionCutoverExecutorConfig{
		Manifest: manifest, InitialTransition: initial, SourceNodes: sourceNodes, FrozenNodes: frozenNodes,
		TargetNodes: targetNodes, CheckpointManifest: checkpointManifest, OperatorID: "fault-drill",
		LeaseDuration: 2 * time.Minute, Now: now,
	}, writer, aggregator, collector, artifacts)
	if err != nil {
		t.Fatal(err)
	}
	orchestrator, err := NewCutoverAttemptOrchestrator(journal, executor, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := orchestrator.Advance(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := orchestrator.Advance(ctx); err != nil {
		t.Fatal(err)
	}
	time.Sleep(750 * time.Millisecond)
	rollback, err := orchestrator.Advance(ctx)
	if err != nil || !rollback.RollbackTriggered || rollback.EventType != CutoverEventRollbackRequested {
		t.Fatalf("expired freeze rollback decision=%+v err=%v", rollback, err)
	}
	for !terminalCutoverAttemptState(journal.Projection.State) {
		if _, err := orchestrator.Advance(ctx); err != nil {
			t.Fatal(err)
		}
	}
	loaded, err := LoadCutoverAttemptJournal(journal.Directory)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Projection.State != CutoverAttemptRolledBack || loaded.Projection.LastSequence != 7 {
		t.Fatalf("expired freeze rollback projection=%+v", loaded.Projection)
	}
	payload, err := client.Get(ctx, key).Bytes()
	if err != nil {
		t.Fatal(err)
	}
	record, err := decodeFenceRecord(payload)
	if err != nil {
		t.Fatal(err)
	}
	if record.Authority != AuthorityGo || record.Phase != FencePhaseActive || record.Epoch != 2 {
		t.Fatalf("expired freeze rollback fence=%+v", record)
	}
	return loaded
}

func createCutoverDrillTopics(t *testing.T, broker string, topics []string) {
	t.Helper()
	conn, err := kafka.Dial("tcp", broker)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	controller, err := conn.Controller()
	if err != nil {
		t.Fatal(err)
	}
	controllerConn, err := kafka.Dial("tcp", net.JoinHostPort(controller.Host, fmt.Sprintf("%d", controller.Port)))
	if err != nil {
		t.Fatal(err)
	}
	defer controllerConn.Close()
	configs := make([]kafka.TopicConfig, 0, len(topics))
	for _, topic := range topics {
		configs = append(configs, kafka.TopicConfig{Topic: topic, NumPartitions: 1, ReplicationFactor: 1})
	}
	if err := controllerConn.CreateTopics(configs...); err != nil {
		t.Fatal(err)
	}
}

func cutoverDrillNodes(manifestID string, authority Authority) FenceExpectedNodeManifest {
	return FenceExpectedNodeManifest{
		SchemaVersion: FenceExpectedNodeManifestSchemaV1, ManifestID: manifestID,
		Nodes: []FenceExpectedNode{
			{Component: "gateway", ObserverID: "gateway-a", ExpectedAuthority: authority},
			{Component: "realtime-delivery", ObserverID: "cpp-a", ExpectedAuthority: authority},
		},
	}
}

func waitCutoverDrillGroups(t *testing.T, ctx context.Context, source *KafkaGoCheckpointSource, manifest DualGroupCheckpointManifest) KafkaCheckpointSourceSnapshot {
	t.Helper()
	waitCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	var lastErr error
	for {
		if snapshot, err := source.Capture(waitCtx, []string{manifest.Groups[0].GroupID, manifest.Groups[1].GroupID}, manifest.Topics); err == nil {
			stable := len(snapshot.Groups) == 2
			for _, group := range snapshot.Groups {
				stable = stable && group.State == "Stable"
			}
			if stable {
				return snapshot
			}
			lastErr = fmt.Errorf("consumer groups are not both Stable: %+v", snapshot.Groups)
		} else {
			lastErr = err
		}
		select {
		case <-waitCtx.Done():
			t.Fatalf("wait for cutover groups: %v (last capture: %v)", waitCtx.Err(), lastErr)
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func waitCutoverDrillGroupBlocked(t *testing.T, ctx context.Context, source *KafkaGoCheckpointSource, manifest DualGroupCheckpointManifest) {
	t.Helper()
	waitCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	var lastErr error
	for {
		snapshot, err := source.Capture(waitCtx, []string{manifest.Groups[0].GroupID, manifest.Groups[1].GroupID}, manifest.Topics)
		if err == nil {
			for _, group := range snapshot.Groups {
				if group.GroupID == manifest.Groups[1].GroupID && group.State != "Stable" {
					return
				}
			}
		} else {
			lastErr = err
		}
		select {
		case <-waitCtx.Done():
			t.Fatalf("wait for blocked primary group: %v (last capture: %v)", waitCtx.Err(), lastErr)
		case <-time.After(250 * time.Millisecond):
		}
	}
}
