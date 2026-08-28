package delivery

import (
	"context"
	"strings"
	"testing"
	"time"
)

type checkpointSourceStub struct {
	snapshot  KafkaCheckpointSourceSnapshot
	err       error
	onCapture func()
}

func (s checkpointSourceStub) Capture(context.Context, []string, []string) (KafkaCheckpointSourceSnapshot, error) {
	if s.onCapture != nil {
		s.onCapture()
	}
	return s.snapshot, s.err
}

func TestDualGroupCheckpointCollectorCapturesZeroLagReceipt(t *testing.T) {
	now := time.Date(2026, 8, 28, 5, 0, 0, 0, time.UTC)
	manifest := validCheckpointManifest()
	proof := validObservationAggregateProof(now)
	collector, err := NewDualGroupCheckpointCollector(checkpointSourceStub{snapshot: validCheckpointSnapshot()}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	receipt, err := collector.Capture(context.Background(), manifest, proof)
	if err != nil {
		t.Fatalf("Capture(): %v", err)
	}
	if receipt.SchemaVersion != DualGroupCheckpointReceiptSchemaV1 || receipt.Decision != DualGroupCheckpointEligible {
		t.Fatalf("unexpected checkpoint receipt: %+v", receipt)
	}
	if receipt.TransitionID != proof.TransitionID || receipt.LeaseSHA256 != proof.LeaseSHA256 || !validSHA256(receipt.ObservationAggregateSHA256) {
		t.Fatalf("checkpoint lost fence proof binding: %+v", receipt)
	}
	if len(receipt.Groups) != 2 || receipt.Groups[0].Role != KafkaCheckpointRoleCompatibility || receipt.Groups[1].Role != KafkaCheckpointRolePrimary {
		t.Fatalf("groups are not canonical: %+v", receipt.Groups)
	}
	for _, group := range receipt.Groups {
		if group.State != "Stable" || len(group.Partitions) != 3 {
			t.Fatalf("invalid group checkpoint: %+v", group)
		}
		for _, partition := range group.Partitions {
			if partition.Lag != 0 || partition.CommittedOffset != partition.LogEndOffset {
				t.Fatalf("nonzero checkpoint lag: %+v", partition)
			}
		}
	}
}

func TestDualGroupCheckpointCollectorFailsClosed(t *testing.T) {
	now := time.Date(2026, 8, 28, 5, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name   string
		mutate func(*KafkaCheckpointSourceSnapshot)
		want   string
	}{
		{name: "unstable group", mutate: func(s *KafkaCheckpointSourceSnapshot) { s.Groups[0].State = "PreparingRebalance" }, want: "Stable"},
		{name: "lagging group", mutate: func(s *KafkaCheckpointSourceSnapshot) { s.Groups[1].Partitions[0].CommittedOffset-- }, want: "lag"},
		{name: "missing assignment", mutate: func(s *KafkaCheckpointSourceSnapshot) {
			s.Groups[0].AssignedPartitions["dipole.message.direct.created"] = []int{0}
		}, want: "assignment"},
		{name: "different high water", mutate: func(s *KafkaCheckpointSourceSnapshot) {
			s.Groups[1].Partitions[0].LogEndOffset++
			s.Groups[1].Partitions[0].CommittedOffset++
		}, want: "log end"},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot := validCheckpointSnapshot()
			test.mutate(&snapshot)
			collector, err := NewDualGroupCheckpointCollector(checkpointSourceStub{snapshot: snapshot}, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}
			if _, err := collector.Capture(context.Background(), validCheckpointManifest(), validObservationAggregateProof(now)); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Capture() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestDualGroupCheckpointCollectorRejectsInvalidManifest(t *testing.T) {
	collector, err := NewDualGroupCheckpointCollector(checkpointSourceStub{snapshot: validCheckpointSnapshot()}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	manifest := validCheckpointManifest()
	manifest.Groups[0].Role = KafkaCheckpointRoleCompatibility
	if _, err := collector.Capture(context.Background(), manifest, validObservationAggregateProof(time.Now().UTC())); err == nil {
		t.Fatal("duplicate checkpoint role must fail")
	}
}

func TestCheckpointPartitionsAcceptUncommittedEmptyPartition(t *testing.T) {
	partitions, _, err := validateCheckpointPartitions(
		"group-a",
		map[string]struct{}{"direct/0": {}},
		[]KafkaPartitionCheckpoint{{Topic: "direct", Partition: 0, CommittedOffset: -1, LogEndOffset: 0}},
	)
	if err != nil {
		t.Fatalf("validateCheckpointPartitions(): %v", err)
	}
	if len(partitions) != 1 || partitions[0].CommittedOffset != -1 || partitions[0].Lag != 0 {
		t.Fatalf("unexpected empty partition checkpoint: %+v", partitions)
	}
}

func TestDualGroupCheckpointCollectorRejectsProofThatExpiresDuringCapture(t *testing.T) {
	started := time.Date(2026, 8, 28, 5, 0, 0, 0, time.UTC)
	now := started
	source := checkpointSourceStub{snapshot: validCheckpointSnapshot(), onCapture: func() { now = started.Add(20 * time.Second) }}
	collector, err := NewDualGroupCheckpointCollector(source, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := collector.Capture(context.Background(), validCheckpointManifest(), validObservationAggregateProof(started)); err == nil || !strings.Contains(err.Error(), "expired during") {
		t.Fatalf("Capture() error = %v", err)
	}
}

func validCheckpointManifest() DualGroupCheckpointManifest {
	return DualGroupCheckpointManifest{
		SchemaVersion: DualGroupCheckpointManifestSchemaV1,
		ManifestID:    "checkpoint-a",
		Topics: []string{
			"dipole.message.group.created",
			"dipole.message.direct.created",
		},
		Groups: []KafkaCheckpointGroupSpec{
			{Role: KafkaCheckpointRolePrimary, GroupID: "dipole-realtime-primary-v1"},
			{Role: KafkaCheckpointRoleCompatibility, GroupID: "dipole-gateway-consumer"},
		},
	}
}

func validCheckpointSnapshot() KafkaCheckpointSourceSnapshot {
	direct := "dipole.message.direct.created"
	group := "dipole.message.group.created"
	partitions := []KafkaPartitionCheckpoint{
		{Topic: group, Partition: 0, CommittedOffset: 20, LogEndOffset: 20},
		{Topic: direct, Partition: 1, CommittedOffset: 12, LogEndOffset: 12},
		{Topic: direct, Partition: 0, CommittedOffset: 10, LogEndOffset: 10},
	}
	return KafkaCheckpointSourceSnapshot{
		ClusterID:       "cluster-a",
		TopicPartitions: map[string][]int{direct: {0, 1}, group: {0}},
		Groups: []KafkaConsumerGroupCheckpoint{
			{GroupID: "dipole-realtime-primary-v1", State: "Stable", AssignedPartitions: map[string][]int{direct: {0, 1}, group: {0}}, Partitions: append([]KafkaPartitionCheckpoint(nil), partitions...)},
			{GroupID: "dipole-gateway-consumer", State: "Stable", AssignedPartitions: map[string][]int{direct: {1, 0}, group: {0}}, Partitions: append([]KafkaPartitionCheckpoint(nil), partitions...)},
		},
	}
}

func validObservationAggregateProof(now time.Time) FenceObservationAggregateReceipt {
	transition := validAggregateTransition(now, FencePhaseActive)
	observation := validAggregateObservation(now, transition, "gateway", "gateway-a")
	return FenceObservationAggregateReceipt{
		SchemaVersion: FenceObservationAggregateReceiptSchemaV1, Decision: FenceObservationAggregateEligible,
		ManifestID: "nodes-a", ManifestSHA256: strings.Repeat("5", 64),
		TransitionID: transition.TransitionID, RequestSHA256: transition.RequestSHA256,
		LeaseSHA256: transition.NextSHA256, Authority: transition.Authority, Phase: transition.Phase,
		Epoch: transition.Epoch, LeaseUntilUnixMS: transition.LeaseUntilUnixMS,
		CapturedAtUnixMS: now.UnixMilli(), Observations: []FenceObservation{observation},
	}
}
