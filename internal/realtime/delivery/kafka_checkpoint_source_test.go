package delivery

import (
	"encoding/hex"
	"reflect"
	"strings"
	"testing"

	kafkago "github.com/segmentio/kafka-go"
)

func TestCheckpointKafkaResponseProjection(t *testing.T) {
	metadata := &kafkago.MetadataResponse{ClusterID: "cluster-a", Topics: []kafkago.Topic{
		{Name: "direct", Partitions: []kafkago.Partition{{ID: 1}, {ID: 0}}},
		{Name: "group", Partitions: []kafkago.Partition{{ID: 0}}},
	}}
	partitions, requests, err := checkpointTopicMetadata(metadata, []string{"group", "direct"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(partitions, map[string][]int{"direct": {0, 1}, "group": {0}}) || len(requests["direct"]) != 2 {
		t.Fatalf("unexpected topic metadata projection: partitions=%v requests=%v", partitions, requests)
	}
	description := &kafkago.DescribeGroupsResponse{Groups: []kafkago.DescribeGroupsResponseGroup{{
		GroupID: "group-a", GroupState: "Stable", Members: []kafkago.DescribeGroupsResponseMember{
			{MemberAssignments: kafkago.DescribeGroupsResponseAssignments{Topics: []kafkago.GroupMemberTopic{{Topic: "direct", Partitions: []int{1}}}}},
			{MemberAssignments: kafkago.DescribeGroupsResponseAssignments{Topics: []kafkago.GroupMemberTopic{{Topic: "direct", Partitions: []int{0}}, {Topic: "group", Partitions: []int{0}}}}},
		},
	}}}
	state, assignments, err := checkpointGroupDescription(description, "group-a")
	if err != nil {
		t.Fatal(err)
	}
	if state != "Stable" || !reflect.DeepEqual(assignments, map[string][]int{"direct": {0, 1}, "group": {0}}) {
		t.Fatalf("unexpected group projection: state=%s assignments=%v", state, assignments)
	}
}

func TestCheckpointCommittedOffsetsPreserveMissingOffset(t *testing.T) {
	response := &kafkago.OffsetFetchResponse{Topics: map[string][]kafkago.OffsetFetchPartition{
		"direct": {{Partition: 0, CommittedOffset: -1}},
	}}
	partitions, err := checkpointCommittedOffsets("group-a", response, map[string][]int{"direct": {0}}, map[string]int64{"direct/0": 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(partitions) != 1 || partitions[0].CommittedOffset != -1 {
		t.Fatalf("missing committed offset was not preserved: %+v", partitions)
	}
	if _, _, err := validateCheckpointPartitions("group-a", map[string]struct{}{"direct/0": {}}, partitions); err == nil || !strings.Contains(err.Error(), "invalid offsets") {
		t.Fatalf("missing committed offset must fail closed: %v", err)
	}
}

func TestNewKafkaGoCheckpointSourceValidatesConfiguration(t *testing.T) {
	if _, err := NewKafkaGoCheckpointSource(nil, "client", 1); err == nil {
		t.Fatal("empty brokers must fail")
	}
	if _, err := NewKafkaGoCheckpointSource([]string{"broker:9092"}, "", 1); err == nil {
		t.Fatal("empty client ID must fail")
	}
}

func TestDecodeCheckpointMemberAssignmentAcceptsLibrdkafkaExtension(t *testing.T) {
	payload, err := hex.DecodeString("000000000002001d6469706f6c652e6d6573736167652e6469726563742e637265617465640000000100000000001c6469706f6c652e6d6573736167652e67726f75702e63726561746564000000010000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	assignments, err := decodeCheckpointMemberAssignment(payload)
	if err != nil {
		t.Fatalf("decodeCheckpointMemberAssignment(): %v", err)
	}
	want := map[string][]int{
		"dipole.message.direct.created": {0},
		"dipole.message.group.created":  {0},
	}
	if !reflect.DeepEqual(assignments, want) {
		t.Fatalf("assignments = %v, want %v", assignments, want)
	}
}

func TestDecodeCheckpointMemberAssignmentAcceptsKafkaGoVersionOne(t *testing.T) {
	payload, err := hex.DecodeString("000100000002001d6469706f6c652e6d6573736167652e6469726563742e637265617465640000000100000000001c6469706f6c652e6d6573736167652e67726f75702e637265617465640000000100000000ffffffff")
	if err != nil {
		t.Fatal(err)
	}
	assignments, err := decodeCheckpointMemberAssignment(payload)
	if err != nil {
		t.Fatalf("decodeCheckpointMemberAssignment(): %v", err)
	}
	if !reflect.DeepEqual(assignments["dipole.message.direct.created"], []int{0}) ||
		!reflect.DeepEqual(assignments["dipole.message.group.created"], []int{0}) {
		t.Fatalf("unexpected kafka-go assignments: %v", assignments)
	}
}

func TestDecodeCheckpointMemberAssignmentFailsClosed(t *testing.T) {
	for _, payload := range [][]byte{
		{0, 4, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 1, 0, 10, 's', 'h', 'o', 'r', 't'},
	} {
		if _, err := decodeCheckpointMemberAssignment(payload); err == nil {
			t.Fatal("invalid member assignment must fail")
		}
	}
}

func TestSameCheckpointOffsetsRequiresStableCaptureWindow(t *testing.T) {
	if !sameCheckpointOffsets(map[string]int64{"direct/0": 10}, map[string]int64{"direct/0": 10}) {
		t.Fatal("equal checkpoint offsets must match")
	}
	if sameCheckpointOffsets(map[string]int64{"direct/0": 10}, map[string]int64{"direct/0": 11}) {
		t.Fatal("moving log end must fail")
	}
}

func TestSameCheckpointAssignmentsRequiresStableGroupWindow(t *testing.T) {
	if !sameCheckpointAssignments(map[string][]int{"direct": {0, 1}}, map[string][]int{"direct": {0, 1}}) {
		t.Fatal("equal assignments must match")
	}
	if sameCheckpointAssignments(map[string][]int{"direct": {0, 1}}, map[string][]int{"direct": {1, 0}}) {
		t.Fatal("changed assignments must fail")
	}
}
