//go:build integration

package delivery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
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
	client redis.Cmdable
	key    string
	now    func() time.Time
	inner  *RedisFenceObservationAggregator
}

func (a cutoverDrillAggregator) Aggregate(ctx context.Context, manifest FenceExpectedNodeManifest, transition FenceTransitionReceipt) (FenceObservationAggregateReceipt, error) {
	for _, node := range manifest.Nodes {
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

func TestRealtimeCutoverFaultDrill(t *testing.T) {
	redisAddr := os.Getenv("DIPOLE_CUTOVER_DRILL_REDIS_ADDR")
	kafkaAddr := os.Getenv("DIPOLE_CUTOVER_DRILL_KAFKA_ADDR")
	reportPath := os.Getenv("DIPOLE_CUTOVER_DRILL_REPORT")
	if redisAddr == "" || kafkaAddr == "" || reportPath == "" {
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
	topics := []string{"dipole.c3.direct." + suffix, "dipole.c3.group." + suffix}
	groups := []string{"dipole-c3-compat-" + suffix, "dipole-c3-primary-" + suffix}
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
	aggregator := cutoverDrillAggregator{client: client, key: key, now: now, inner: innerAggregator}
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
	primary = startCutoverDrillGroup(t, kafkaAddr, groups[1], topics)
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
	report := cutoverFaultDrillReport{
		SchemaVersion: "dipole.realtime.cutover-fault-drill.v1",
		GitRevision:   os.Getenv("DIPOLE_CUTOVER_DRILL_REVISION"), RedisImage: os.Getenv("DIPOLE_CUTOVER_DRILL_REDIS_IMAGE"),
		KafkaImage: os.Getenv("DIPOLE_CUTOVER_DRILL_KAFKA_IMAGE"), KafkaClusterID: stableSnapshot.ClusterID,
		AttemptID: manifest.AttemptID, ControllerCrashArtifact: crashResult.ArtifactSHA256,
		ControllerCrashRecovered: true, RedisOutageBlocked: true, KafkaRebalanceBlocked: true,
		FinalState: current.Projection.State, FinalSequence: current.Projection.LastSequence,
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
