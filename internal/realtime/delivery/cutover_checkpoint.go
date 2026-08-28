package delivery

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	DualGroupCheckpointManifestSchemaV1 = "dipole.realtime.delivery-checkpoint-manifest.v1"
	DualGroupCheckpointReceiptSchemaV1  = "dipole.realtime.delivery-checkpoint-receipt.v1"
	DualGroupCheckpointEligible         = "eligible"

	KafkaCheckpointRoleCompatibility = "compatibility"
	KafkaCheckpointRolePrimary       = "primary"
)

type KafkaCheckpointGroupSpec struct {
	Role    string `json:"role"`
	GroupID string `json:"group_id"`
}

type DualGroupCheckpointManifest struct {
	SchemaVersion string                     `json:"schema_version"`
	ManifestID    string                     `json:"manifest_id"`
	Topics        []string                   `json:"topics"`
	Groups        []KafkaCheckpointGroupSpec `json:"groups"`
}

type KafkaPartitionCheckpoint struct {
	Topic           string `json:"topic"`
	Partition       int    `json:"partition"`
	CommittedOffset int64  `json:"committed_offset"`
	LogEndOffset    int64  `json:"log_end_offset"`
	Lag             int64  `json:"lag"`
}

type KafkaConsumerGroupCheckpoint struct {
	Role               string                     `json:"role,omitempty"`
	GroupID            string                     `json:"group_id"`
	State              string                     `json:"state"`
	AssignedPartitions map[string][]int           `json:"-"`
	Partitions         []KafkaPartitionCheckpoint `json:"partitions"`
}

type KafkaCheckpointSourceSnapshot struct {
	ClusterID       string
	TopicPartitions map[string][]int
	Groups          []KafkaConsumerGroupCheckpoint
}

type KafkaCheckpointSource interface {
	Capture(ctx context.Context, groupIDs, topics []string) (KafkaCheckpointSourceSnapshot, error)
}

type DualGroupCheckpointReceipt struct {
	SchemaVersion              string                         `json:"schema_version"`
	Decision                   string                         `json:"decision"`
	ManifestID                 string                         `json:"manifest_id"`
	ManifestSHA256             string                         `json:"manifest_sha256"`
	ObservationAggregateSHA256 string                         `json:"observation_aggregate_sha256"`
	TransitionID               string                         `json:"transition_id"`
	LeaseSHA256                string                         `json:"lease_sha256"`
	Authority                  Authority                      `json:"authority"`
	Phase                      FencePhase                     `json:"phase"`
	Epoch                      uint64                         `json:"epoch"`
	ClusterID                  string                         `json:"cluster_id"`
	CapturedAtUnixMS           int64                          `json:"captured_at_unix_ms"`
	Groups                     []KafkaConsumerGroupCheckpoint `json:"groups"`
}

type DualGroupCheckpointCollector struct {
	source KafkaCheckpointSource
	now    func() time.Time
}

func NewDualGroupCheckpointCollector(source KafkaCheckpointSource, now func() time.Time) (*DualGroupCheckpointCollector, error) {
	if source == nil || now == nil {
		return nil, fmt.Errorf("dual-group checkpoint collector configuration is invalid")
	}
	return &DualGroupCheckpointCollector{source: source, now: now}, nil
}

func (c *DualGroupCheckpointCollector) Capture(
	ctx context.Context,
	manifest DualGroupCheckpointManifest,
	proof FenceObservationAggregateReceipt,
) (DualGroupCheckpointReceipt, error) {
	manifest, manifestSHA256, err := validateDualGroupCheckpointManifest(manifest)
	if err != nil {
		return DualGroupCheckpointReceipt{}, err
	}
	proofSHA256, err := validateObservationAggregateProof(proof, c.now().UTC())
	if err != nil {
		return DualGroupCheckpointReceipt{}, err
	}
	groupIDs := make([]string, len(manifest.Groups))
	for index := range manifest.Groups {
		groupIDs[index] = manifest.Groups[index].GroupID
	}
	snapshot, err := c.source.Capture(ctx, groupIDs, manifest.Topics)
	if err != nil {
		return DualGroupCheckpointReceipt{}, fmt.Errorf("capture Kafka dual-group checkpoint: %w", err)
	}
	now := c.now().UTC()
	finalProofSHA256, err := validateObservationAggregateProof(proof, now)
	if err != nil {
		return DualGroupCheckpointReceipt{}, fmt.Errorf("delivery authority observation aggregate expired during Kafka checkpoint capture: %w", err)
	}
	if finalProofSHA256 != proofSHA256 {
		return DualGroupCheckpointReceipt{}, fmt.Errorf("delivery authority observation aggregate changed during Kafka checkpoint capture")
	}
	groups, err := validateKafkaCheckpointSnapshot(manifest, snapshot)
	if err != nil {
		return DualGroupCheckpointReceipt{}, err
	}
	return DualGroupCheckpointReceipt{
		SchemaVersion: DualGroupCheckpointReceiptSchemaV1, Decision: DualGroupCheckpointEligible,
		ManifestID: manifest.ManifestID, ManifestSHA256: manifestSHA256,
		ObservationAggregateSHA256: proofSHA256, TransitionID: proof.TransitionID,
		LeaseSHA256: proof.LeaseSHA256, Authority: proof.Authority, Phase: proof.Phase, Epoch: proof.Epoch,
		ClusterID: snapshot.ClusterID, CapturedAtUnixMS: now.UnixMilli(), Groups: groups,
	}, nil
}

func validateDualGroupCheckpointManifest(manifest DualGroupCheckpointManifest) (DualGroupCheckpointManifest, string, error) {
	manifest.Topics = append([]string(nil), manifest.Topics...)
	manifest.Groups = append([]KafkaCheckpointGroupSpec(nil), manifest.Groups...)
	manifest.ManifestID = strings.TrimSpace(manifest.ManifestID)
	if manifest.SchemaVersion != DualGroupCheckpointManifestSchemaV1 || !fenceTransitionIDPattern.MatchString(manifest.ManifestID) {
		return manifest, "", fmt.Errorf("dual-group checkpoint manifest identity is invalid")
	}
	if len(manifest.Topics) == 0 || len(manifest.Topics) > 16 || len(manifest.Groups) != 2 {
		return manifest, "", fmt.Errorf("dual-group checkpoint manifest size is invalid")
	}
	for index := range manifest.Topics {
		manifest.Topics[index] = strings.TrimSpace(manifest.Topics[index])
		if !fenceTransitionIDPattern.MatchString(manifest.Topics[index]) {
			return manifest, "", fmt.Errorf("dual-group checkpoint topic is invalid")
		}
	}
	sort.Strings(manifest.Topics)
	if hasAdjacentDuplicate(manifest.Topics) {
		return manifest, "", fmt.Errorf("dual-group checkpoint topics contain duplicates")
	}
	seenRoles := make(map[string]struct{}, 2)
	seenGroups := make(map[string]struct{}, 2)
	for index := range manifest.Groups {
		manifest.Groups[index].Role = strings.TrimSpace(manifest.Groups[index].Role)
		manifest.Groups[index].GroupID = strings.TrimSpace(manifest.Groups[index].GroupID)
		if manifest.Groups[index].Role != KafkaCheckpointRoleCompatibility && manifest.Groups[index].Role != KafkaCheckpointRolePrimary {
			return manifest, "", fmt.Errorf("dual-group checkpoint role is invalid")
		}
		if !fenceTransitionIDPattern.MatchString(manifest.Groups[index].GroupID) {
			return manifest, "", fmt.Errorf("dual-group checkpoint group ID is invalid")
		}
		if _, exists := seenRoles[manifest.Groups[index].Role]; exists {
			return manifest, "", fmt.Errorf("dual-group checkpoint roles must be unique")
		}
		if _, exists := seenGroups[manifest.Groups[index].GroupID]; exists {
			return manifest, "", fmt.Errorf("dual-group checkpoint group IDs must be unique")
		}
		seenRoles[manifest.Groups[index].Role] = struct{}{}
		seenGroups[manifest.Groups[index].GroupID] = struct{}{}
	}
	sort.Slice(manifest.Groups, func(i, j int) bool { return manifest.Groups[i].Role < manifest.Groups[j].Role })
	payload, err := json.Marshal(manifest)
	if err != nil {
		return manifest, "", fmt.Errorf("encode dual-group checkpoint manifest: %w", err)
	}
	return manifest, hashBytes(payload), nil
}

func validateObservationAggregateProof(proof FenceObservationAggregateReceipt, now time.Time) (string, error) {
	if proof.SchemaVersion != FenceObservationAggregateReceiptSchemaV1 || proof.Decision != FenceObservationAggregateEligible ||
		!fenceTransitionIDPattern.MatchString(proof.ManifestID) || !fenceTransitionIDPattern.MatchString(proof.TransitionID) ||
		!validSHA256(proof.ManifestSHA256) || !validSHA256(proof.RequestSHA256) || !validSHA256(proof.LeaseSHA256) ||
		proof.Epoch == 0 || proof.LeaseUntilUnixMS <= 0 || proof.CapturedAtUnixMS <= 0 || len(proof.Observations) == 0 {
		return "", fmt.Errorf("delivery authority observation aggregate proof is invalid")
	}
	if time.UnixMilli(proof.CapturedAtUnixMS).After(now.Add(2*time.Second)) || !time.UnixMilli(proof.LeaseUntilUnixMS).After(now) {
		return "", fmt.Errorf("delivery authority observation aggregate proof is expired or from the future")
	}
	transition := FenceTransitionReceipt{
		SchemaVersion: FenceTransitionReceiptSchemaV1, TransitionID: proof.TransitionID,
		RequestSHA256: proof.RequestSHA256, NextSHA256: proof.LeaseSHA256,
		Authority: proof.Authority, Phase: proof.Phase, Epoch: proof.Epoch,
		LeaseUntilUnixMS: proof.LeaseUntilUnixMS,
	}
	seen := make(map[string]struct{}, len(proof.Observations))
	for _, observation := range proof.Observations {
		node := FenceExpectedNode{Component: observation.Component, ObserverID: observation.ObserverID}
		identity := node.Component + "\x00" + node.ObserverID
		if _, exists := seen[identity]; exists {
			return "", fmt.Errorf("delivery authority observation aggregate contains duplicate node")
		}
		seen[identity] = struct{}{}
		if err := validateAggregateObservation(observation, node, transition, now); err != nil {
			return "", fmt.Errorf("delivery authority observation aggregate node %s/%s: %w", node.Component, node.ObserverID, err)
		}
	}
	payload, err := json.Marshal(proof)
	if err != nil {
		return "", fmt.Errorf("encode delivery authority observation aggregate proof: %w", err)
	}
	return hashBytes(payload), nil
}

func validateKafkaCheckpointSnapshot(manifest DualGroupCheckpointManifest, snapshot KafkaCheckpointSourceSnapshot) ([]KafkaConsumerGroupCheckpoint, error) {
	if strings.TrimSpace(snapshot.ClusterID) == "" {
		return nil, fmt.Errorf("Kafka checkpoint cluster ID is empty")
	}
	expectedCoordinates, err := checkpointCoordinates(manifest.Topics, snapshot.TopicPartitions)
	if err != nil {
		return nil, err
	}
	groupsByID := make(map[string]KafkaConsumerGroupCheckpoint, len(snapshot.Groups))
	for _, group := range snapshot.Groups {
		if _, exists := groupsByID[group.GroupID]; exists {
			return nil, fmt.Errorf("Kafka checkpoint source returned duplicate group %s", group.GroupID)
		}
		groupsByID[group.GroupID] = group
	}
	result := make([]KafkaConsumerGroupCheckpoint, 0, len(manifest.Groups))
	var reference map[string]int64
	for _, spec := range manifest.Groups {
		group, exists := groupsByID[spec.GroupID]
		if !exists {
			return nil, fmt.Errorf("Kafka checkpoint group %s is missing", spec.GroupID)
		}
		if group.State != "Stable" {
			return nil, fmt.Errorf("Kafka checkpoint group %s state must be Stable, got %s", spec.GroupID, group.State)
		}
		if err := validateCheckpointAssignments(spec.GroupID, expectedCoordinates, group.AssignedPartitions); err != nil {
			return nil, err
		}
		partitions, highWater, err := validateCheckpointPartitions(spec.GroupID, expectedCoordinates, group.Partitions)
		if err != nil {
			return nil, err
		}
		if reference == nil {
			reference = highWater
		} else {
			for coordinate, offset := range reference {
				if highWater[coordinate] != offset {
					return nil, fmt.Errorf("Kafka checkpoint groups disagree on log end for %s", coordinate)
				}
			}
		}
		result = append(result, KafkaConsumerGroupCheckpoint{Role: spec.Role, GroupID: spec.GroupID, State: group.State, Partitions: partitions})
	}
	return result, nil
}

func checkpointCoordinates(topics []string, partitions map[string][]int) (map[string]struct{}, error) {
	coordinates := make(map[string]struct{})
	for _, topic := range topics {
		values, exists := partitions[topic]
		if !exists || len(values) == 0 {
			return nil, fmt.Errorf("Kafka checkpoint topic %s has no partitions", topic)
		}
		seen := make(map[int]struct{}, len(values))
		for _, partition := range values {
			if partition < 0 {
				return nil, fmt.Errorf("Kafka checkpoint topic %s has invalid partition", topic)
			}
			if _, exists := seen[partition]; exists {
				return nil, fmt.Errorf("Kafka checkpoint topic %s has duplicate partition", topic)
			}
			seen[partition] = struct{}{}
			coordinates[checkpointCoordinate(topic, partition)] = struct{}{}
		}
	}
	return coordinates, nil
}

func validateCheckpointAssignments(groupID string, expected map[string]struct{}, assigned map[string][]int) error {
	actual := make(map[string]struct{})
	for topic, partitions := range assigned {
		for _, partition := range partitions {
			actual[checkpointCoordinate(topic, partition)] = struct{}{}
		}
	}
	if !sameCoordinateSet(expected, actual) {
		return fmt.Errorf("Kafka checkpoint group %s assignment does not cover the exact topic partitions", groupID)
	}
	return nil
}

func validateCheckpointPartitions(groupID string, expected map[string]struct{}, input []KafkaPartitionCheckpoint) ([]KafkaPartitionCheckpoint, map[string]int64, error) {
	partitions := append([]KafkaPartitionCheckpoint(nil), input...)
	actual := make(map[string]struct{}, len(partitions))
	highWater := make(map[string]int64, len(partitions))
	for index := range partitions {
		coordinate := checkpointCoordinate(partitions[index].Topic, partitions[index].Partition)
		if _, exists := actual[coordinate]; exists {
			return nil, nil, fmt.Errorf("Kafka checkpoint group %s returned duplicate partition %s", groupID, coordinate)
		}
		actual[coordinate] = struct{}{}
		if partitions[index].CommittedOffset == -1 && partitions[index].LogEndOffset == 0 {
			partitions[index].Lag = 0
		} else if partitions[index].CommittedOffset < 0 || partitions[index].LogEndOffset < partitions[index].CommittedOffset {
			return nil, nil, fmt.Errorf("Kafka checkpoint group %s has invalid offsets for %s", groupID, coordinate)
		} else {
			partitions[index].Lag = partitions[index].LogEndOffset - partitions[index].CommittedOffset
		}
		if partitions[index].Lag != 0 {
			return nil, nil, fmt.Errorf("Kafka checkpoint group %s has nonzero lag for %s", groupID, coordinate)
		}
		highWater[coordinate] = partitions[index].LogEndOffset
	}
	if !sameCoordinateSet(expected, actual) {
		return nil, nil, fmt.Errorf("Kafka checkpoint group %s offsets do not cover the exact topic partitions", groupID)
	}
	sort.Slice(partitions, func(i, j int) bool {
		if partitions[i].Topic == partitions[j].Topic {
			return partitions[i].Partition < partitions[j].Partition
		}
		return partitions[i].Topic < partitions[j].Topic
	})
	return partitions, highWater, nil
}

func checkpointCoordinate(topic string, partition int) string {
	return fmt.Sprintf("%s/%d", topic, partition)
}

func sameCoordinateSet(left, right map[string]struct{}) bool {
	if len(left) != len(right) {
		return false
	}
	for coordinate := range left {
		if _, exists := right[coordinate]; !exists {
			return false
		}
	}
	return true
}

func hasAdjacentDuplicate(values []string) bool {
	for index := 1; index < len(values); index++ {
		if values[index] == values[index-1] {
			return true
		}
	}
	return false
}
