package delivery

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/protocol/describegroups"
)

type KafkaGoCheckpointSource struct {
	brokers     []string
	clientID    string
	dialTimeout time.Duration
}

func NewKafkaGoCheckpointSource(brokers []string, clientID string, dialTimeout time.Duration) (*KafkaGoCheckpointSource, error) {
	normalized := make([]string, 0, len(brokers))
	for _, broker := range brokers {
		broker = strings.TrimSpace(broker)
		if broker == "" {
			return nil, fmt.Errorf("Kafka checkpoint broker is empty")
		}
		normalized = append(normalized, broker)
	}
	clientID = strings.TrimSpace(clientID)
	if len(normalized) == 0 || clientID == "" || dialTimeout <= 0 || dialTimeout > time.Minute {
		return nil, fmt.Errorf("Kafka checkpoint source configuration is invalid")
	}
	return &KafkaGoCheckpointSource{brokers: normalized, clientID: clientID, dialTimeout: dialTimeout}, nil
}

func (s *KafkaGoCheckpointSource) Capture(ctx context.Context, groupIDs, topics []string) (KafkaCheckpointSourceSnapshot, error) {
	transport := &kafkago.Transport{ClientID: s.clientID, DialTimeout: s.dialTimeout}
	defer transport.CloseIdleConnections()
	client := &kafkago.Client{Addr: kafkago.TCP(s.brokers...), Transport: transport}

	metadata, err := client.Metadata(ctx, &kafkago.MetadataRequest{Topics: topics})
	if err != nil {
		return KafkaCheckpointSourceSnapshot{}, fmt.Errorf("read Kafka checkpoint metadata: %w", err)
	}
	topicPartitions, offsetRequests, err := checkpointTopicMetadata(metadata, topics)
	if err != nil {
		return KafkaCheckpointSourceSnapshot{}, err
	}
	offsets, err := client.ListOffsets(ctx, &kafkago.ListOffsetsRequest{Topics: offsetRequests, IsolationLevel: kafkago.ReadCommitted})
	if err != nil {
		return KafkaCheckpointSourceSnapshot{}, fmt.Errorf("read Kafka checkpoint log ends: %w", err)
	}
	logEnds, err := checkpointLogEnds(offsets, topicPartitions)
	if err != nil {
		return KafkaCheckpointSourceSnapshot{}, err
	}

	groups := make([]KafkaConsumerGroupCheckpoint, 0, len(groupIDs))
	for _, groupID := range groupIDs {
		group, err := s.captureGroup(ctx, client, groupID, topicPartitions, logEnds)
		if err != nil {
			return KafkaCheckpointSourceSnapshot{}, err
		}
		groups = append(groups, group)
	}
	for _, group := range groups {
		_, state, assignments, err := s.describeGroup(ctx, client, group.GroupID)
		if err != nil {
			return KafkaCheckpointSourceSnapshot{}, err
		}
		if state != group.State || !sameCheckpointAssignments(assignments, group.AssignedPartitions) {
			return KafkaCheckpointSourceSnapshot{}, fmt.Errorf("Kafka checkpoint group %s changed during capture", group.GroupID)
		}
	}
	finalOffsets, err := client.ListOffsets(ctx, &kafkago.ListOffsetsRequest{Topics: offsetRequests, IsolationLevel: kafkago.ReadCommitted})
	if err != nil {
		return KafkaCheckpointSourceSnapshot{}, fmt.Errorf("re-read Kafka checkpoint log ends: %w", err)
	}
	finalLogEnds, err := checkpointLogEnds(finalOffsets, topicPartitions)
	if err != nil {
		return KafkaCheckpointSourceSnapshot{}, err
	}
	if !sameCheckpointOffsets(logEnds, finalLogEnds) {
		return KafkaCheckpointSourceSnapshot{}, fmt.Errorf("Kafka checkpoint log ends changed during capture")
	}
	return KafkaCheckpointSourceSnapshot{ClusterID: metadata.ClusterID, TopicPartitions: topicPartitions, Groups: groups}, nil
}

func (s *KafkaGoCheckpointSource) captureGroup(
	ctx context.Context,
	client *kafkago.Client,
	groupID string,
	topicPartitions map[string][]int,
	logEnds map[string]int64,
) (KafkaConsumerGroupCheckpoint, error) {
	address, state, assignments, err := s.describeGroup(ctx, client, groupID)
	if err != nil {
		return KafkaConsumerGroupCheckpoint{}, err
	}
	fetched, err := client.OffsetFetch(ctx, &kafkago.OffsetFetchRequest{Addr: address, GroupID: groupID, Topics: topicPartitions})
	if err != nil {
		return KafkaConsumerGroupCheckpoint{}, fmt.Errorf("fetch Kafka checkpoint group %s offsets: %w", groupID, err)
	}
	if fetched.Error != nil {
		return KafkaConsumerGroupCheckpoint{}, fmt.Errorf("fetch Kafka checkpoint group %s offsets: %w", groupID, fetched.Error)
	}
	partitions, err := checkpointCommittedOffsets(groupID, fetched, topicPartitions, logEnds)
	if err != nil {
		return KafkaConsumerGroupCheckpoint{}, err
	}
	return KafkaConsumerGroupCheckpoint{
		GroupID: groupID, State: state, AssignedPartitions: assignments, Partitions: partitions,
	}, nil
}

func (s *KafkaGoCheckpointSource) describeGroup(
	ctx context.Context,
	client *kafkago.Client,
	groupID string,
) (net.Addr, string, map[string][]int, error) {
	coordinator, err := client.FindCoordinator(ctx, &kafkago.FindCoordinatorRequest{
		Key: groupID, KeyType: kafkago.CoordinatorKeyTypeConsumer,
	})
	if err != nil {
		return nil, "", nil, fmt.Errorf("find Kafka checkpoint group %s coordinator: %w", groupID, err)
	}
	if coordinator.Error != nil {
		return nil, "", nil, fmt.Errorf("find Kafka checkpoint group %s coordinator: %w", groupID, coordinator.Error)
	}
	if coordinator.Coordinator == nil || strings.TrimSpace(coordinator.Coordinator.Host) == "" {
		return nil, "", nil, fmt.Errorf("Kafka checkpoint group %s coordinator is unavailable", groupID)
	}
	address := kafkago.TCP(net.JoinHostPort(coordinator.Coordinator.Host, strconv.Itoa(coordinator.Coordinator.Port)))
	description, err := checkpointDescribeGroup(ctx, client, address, groupID)
	if err != nil {
		return nil, "", nil, fmt.Errorf("describe Kafka checkpoint group %s: %w", groupID, err)
	}
	return address, description.State, description.Assignments, nil
}

type rawCheckpointGroupDescription struct {
	State       string
	Assignments map[string][]int
}

func checkpointDescribeGroup(
	ctx context.Context,
	client *kafkago.Client,
	address net.Addr,
	groupID string,
) (rawCheckpointGroupDescription, error) {
	roundTripper := client.Transport
	if roundTripper == nil {
		roundTripper = kafkago.DefaultTransport
	}
	message, err := roundTripper.RoundTrip(ctx, address, &describegroups.Request{Groups: []string{groupID}})
	if err != nil {
		return rawCheckpointGroupDescription{}, err
	}
	response, ok := message.(*describegroups.Response)
	if !ok {
		return rawCheckpointGroupDescription{}, fmt.Errorf("Kafka checkpoint group response type is invalid")
	}
	for _, group := range response.Groups {
		if group.GroupID != groupID {
			continue
		}
		if group.ErrorCode != 0 {
			return rawCheckpointGroupDescription{}, kafkago.Error(group.ErrorCode)
		}
		assignments := make(map[string][]int)
		for _, member := range group.Members {
			memberAssignments, err := decodeCheckpointMemberAssignment(member.MemberAssignment)
			if err != nil {
				return rawCheckpointGroupDescription{}, fmt.Errorf("decode Kafka checkpoint group %s member %s assignment: %w", groupID, member.MemberID, err)
			}
			for topic, partitions := range memberAssignments {
				assignments[topic] = append(assignments[topic], partitions...)
			}
		}
		for topic := range assignments {
			sort.Ints(assignments[topic])
		}
		return rawCheckpointGroupDescription{State: group.GroupState, Assignments: assignments}, nil
	}
	return rawCheckpointGroupDescription{}, fmt.Errorf("Kafka checkpoint group %s was not described", groupID)
}

func decodeCheckpointMemberAssignment(payload []byte) (map[string][]int, error) {
	if len(payload) < 6 || len(payload) > 1<<20 {
		return nil, fmt.Errorf("member assignment size is invalid")
	}
	reader := bytes.NewReader(payload)
	var version int16
	if err := binary.Read(reader, binary.BigEndian, &version); err != nil {
		return nil, err
	}
	if version < 0 || version > 3 {
		return nil, fmt.Errorf("member assignment version %d is unsupported", version)
	}
	var topicCount int32
	if err := binary.Read(reader, binary.BigEndian, &topicCount); err != nil {
		return nil, err
	}
	if topicCount < 0 || topicCount > 1024 {
		return nil, fmt.Errorf("member assignment topic count is invalid")
	}
	assignments := make(map[string][]int, topicCount)
	for range topicCount {
		topic, err := readCheckpointKafkaString(reader)
		if err != nil {
			return nil, err
		}
		if _, exists := assignments[topic]; exists {
			return nil, fmt.Errorf("member assignment contains duplicate topic %s", topic)
		}
		var partitionCount int32
		if err := binary.Read(reader, binary.BigEndian, &partitionCount); err != nil {
			return nil, err
		}
		if partitionCount < 0 || partitionCount > 100000 {
			return nil, fmt.Errorf("member assignment partition count is invalid")
		}
		partitions := make([]int, 0, partitionCount)
		seen := make(map[int32]struct{}, partitionCount)
		for range partitionCount {
			var partition int32
			if err := binary.Read(reader, binary.BigEndian, &partition); err != nil {
				return nil, err
			}
			if partition < 0 {
				return nil, fmt.Errorf("member assignment partition is invalid")
			}
			if _, exists := seen[partition]; exists {
				return nil, fmt.Errorf("member assignment contains duplicate partition")
			}
			seen[partition] = struct{}{}
			partitions = append(partitions, int(partition))
		}
		assignments[topic] = partitions
	}
	// Consumer clients append opaque user data and version-specific extensions.
	// The canonical assignment is the bounded version/topic/partition prefix.
	return assignments, nil
}

func readCheckpointKafkaString(reader *bytes.Reader) (string, error) {
	var length int16
	if err := binary.Read(reader, binary.BigEndian, &length); err != nil {
		return "", err
	}
	if length <= 0 || length > 249 || int(length) > reader.Len() {
		return "", fmt.Errorf("member assignment topic length is invalid")
	}
	value := make([]byte, length)
	if _, err := io.ReadFull(reader, value); err != nil {
		return "", err
	}
	return string(value), nil
}

func checkpointTopicMetadata(metadata *kafkago.MetadataResponse, expectedTopics []string) (map[string][]int, map[string][]kafkago.OffsetRequest, error) {
	if metadata == nil {
		return nil, nil, fmt.Errorf("Kafka checkpoint metadata is unavailable")
	}
	wanted := make(map[string]struct{}, len(expectedTopics))
	for _, topic := range expectedTopics {
		wanted[topic] = struct{}{}
	}
	partitions := make(map[string][]int, len(expectedTopics))
	requests := make(map[string][]kafkago.OffsetRequest, len(expectedTopics))
	for _, topic := range metadata.Topics {
		if _, exists := wanted[topic.Name]; !exists {
			continue
		}
		if topic.Error != nil {
			return nil, nil, fmt.Errorf("read Kafka checkpoint topic %s metadata: %w", topic.Name, topic.Error)
		}
		for _, partition := range topic.Partitions {
			if partition.Error != nil {
				return nil, nil, fmt.Errorf("read Kafka checkpoint topic %s partition %d metadata: %w", topic.Name, partition.ID, partition.Error)
			}
			partitions[topic.Name] = append(partitions[topic.Name], partition.ID)
			requests[topic.Name] = append(requests[topic.Name], kafkago.LastOffsetOf(partition.ID))
		}
		sort.Ints(partitions[topic.Name])
	}
	for _, topic := range expectedTopics {
		if len(partitions[topic]) == 0 {
			return nil, nil, fmt.Errorf("Kafka checkpoint topic %s metadata is missing", topic)
		}
	}
	return partitions, requests, nil
}

func checkpointLogEnds(response *kafkago.ListOffsetsResponse, expected map[string][]int) (map[string]int64, error) {
	if response == nil {
		return nil, fmt.Errorf("Kafka checkpoint log-end response is unavailable")
	}
	logEnds := make(map[string]int64)
	for topic, partitions := range response.Topics {
		for _, partition := range partitions {
			if partition.Error != nil {
				return nil, fmt.Errorf("read Kafka checkpoint log end %s/%d: %w", topic, partition.Partition, partition.Error)
			}
			logEnds[checkpointCoordinate(topic, partition.Partition)] = partition.LastOffset
		}
	}
	for topic, partitions := range expected {
		for _, partition := range partitions {
			coordinate := checkpointCoordinate(topic, partition)
			if offset, exists := logEnds[coordinate]; !exists || offset < 0 {
				return nil, fmt.Errorf("Kafka checkpoint log end %s is missing", coordinate)
			}
		}
	}
	return logEnds, nil
}

func checkpointGroupDescription(response *kafkago.DescribeGroupsResponse, groupID string) (string, map[string][]int, error) {
	if response == nil {
		return "", nil, fmt.Errorf("Kafka checkpoint group %s description is unavailable", groupID)
	}
	for _, group := range response.Groups {
		if group.GroupID != groupID {
			continue
		}
		if group.Error != nil {
			return "", nil, fmt.Errorf("describe Kafka checkpoint group %s: %w", groupID, group.Error)
		}
		assignments := make(map[string][]int)
		for _, member := range group.Members {
			for _, topic := range member.MemberAssignments.Topics {
				assignments[topic.Topic] = append(assignments[topic.Topic], topic.Partitions...)
			}
		}
		for topic := range assignments {
			sort.Ints(assignments[topic])
		}
		return group.GroupState, assignments, nil
	}
	return "", nil, fmt.Errorf("Kafka checkpoint group %s was not described", groupID)
}

func checkpointCommittedOffsets(
	groupID string,
	response *kafkago.OffsetFetchResponse,
	expected map[string][]int,
	logEnds map[string]int64,
) ([]KafkaPartitionCheckpoint, error) {
	partitions := make([]KafkaPartitionCheckpoint, 0)
	for topic, values := range response.Topics {
		for _, partition := range values {
			if partition.Error != nil {
				return nil, fmt.Errorf("fetch Kafka checkpoint group %s offset %s/%d: %w", groupID, topic, partition.Partition, partition.Error)
			}
			coordinate := checkpointCoordinate(topic, partition.Partition)
			logEnd, exists := logEnds[coordinate]
			if !exists {
				return nil, fmt.Errorf("Kafka checkpoint log end %s is missing", coordinate)
			}
			partitions = append(partitions, KafkaPartitionCheckpoint{
				Topic: topic, Partition: partition.Partition,
				CommittedOffset: partition.CommittedOffset, LogEndOffset: logEnd,
			})
		}
	}
	expectedCount := 0
	for _, values := range expected {
		expectedCount += len(values)
	}
	if len(partitions) != expectedCount {
		return nil, fmt.Errorf("Kafka checkpoint group %s offsets are incomplete", groupID)
	}
	return partitions, nil
}

func sameCheckpointOffsets(left, right map[string]int64) bool {
	if len(left) != len(right) {
		return false
	}
	for coordinate, offset := range left {
		if right[coordinate] != offset {
			return false
		}
	}
	return true
}

func sameCheckpointAssignments(left, right map[string][]int) bool {
	if len(left) != len(right) {
		return false
	}
	for topic, leftPartitions := range left {
		rightPartitions, exists := right[topic]
		if !exists || len(leftPartitions) != len(rightPartitions) {
			return false
		}
		for index := range leftPartitions {
			if leftPartitions[index] != rightPartitions[index] {
				return false
			}
		}
	}
	return true
}
