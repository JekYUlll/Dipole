package kafka

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"

	kafkago "github.com/segmentio/kafka-go"
)

type consumerGroupSnapshot struct {
	State              string
	AssignedPartitions map[string]int
}

func (c *Consumer) registeredTopics() []string {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	topics := make([]string, 0, len(c.handlers))
	for topic := range c.handlers {
		topics = append(topics, topic)
	}
	sort.Strings(topics)
	return topics
}

func validateConsumerGroupReadiness(groupID string, expectedTopics []string, snapshot consumerGroupSnapshot) error {
	state := strings.TrimSpace(snapshot.State)
	if state != "Stable" {
		if state == "" {
			state = "empty"
		}
		return fmt.Errorf("kafka consumer group %s state is %s", groupID, state)
	}
	if len(expectedTopics) == 0 {
		return fmt.Errorf("kafka consumer group %s has no registered topics", groupID)
	}
	for _, topic := range expectedTopics {
		if snapshot.AssignedPartitions[topic] <= 0 {
			return fmt.Errorf("kafka consumer group %s has no assigned partition for topic %s", groupID, topic)
		}
	}
	return nil
}

// ValidateReadiness checks the group-wide assignment because each topic reader
// is an independent member and delivery can be routed across Gateway nodes.
func (c *Consumer) ValidateReadiness(ctx context.Context) error {
	if c == nil {
		return fmt.Errorf("kafka consumer is unavailable")
	}
	expectedTopics := c.registeredTopics()
	if len(expectedTopics) == 0 {
		return fmt.Errorf("kafka consumer group %s has no registered topics", c.groupID)
	}

	transport := &kafkago.Transport{
		ClientID:    c.clientID,
		DialTimeout: c.dialTimeout,
	}
	defer transport.CloseIdleConnections()
	client := &kafkago.Client{Addr: kafkago.TCP(c.brokers...), Transport: transport}

	coordinator, err := client.FindCoordinator(ctx, &kafkago.FindCoordinatorRequest{
		Key:     c.groupID,
		KeyType: kafkago.CoordinatorKeyTypeConsumer,
	})
	if err != nil {
		return fmt.Errorf("find kafka consumer group %s coordinator: %w", c.groupID, err)
	}
	if coordinator.Error != nil {
		return fmt.Errorf("find kafka consumer group %s coordinator: %w", c.groupID, coordinator.Error)
	}
	if coordinator.Coordinator == nil || strings.TrimSpace(coordinator.Coordinator.Host) == "" {
		return fmt.Errorf("kafka consumer group %s coordinator is unavailable", c.groupID)
	}

	response, err := client.DescribeGroups(ctx, &kafkago.DescribeGroupsRequest{
		Addr: kafkago.TCP(net.JoinHostPort(
			coordinator.Coordinator.Host,
			strconv.Itoa(coordinator.Coordinator.Port),
		)),
		GroupIDs: []string{c.groupID},
	})
	if err != nil {
		return fmt.Errorf("describe kafka consumer group %s: %w", c.groupID, err)
	}
	for _, group := range response.Groups {
		if group.GroupID != c.groupID {
			continue
		}
		if group.Error != nil {
			return fmt.Errorf("describe kafka consumer group %s: %w", c.groupID, group.Error)
		}
		snapshot := consumerGroupSnapshot{
			State:              group.GroupState,
			AssignedPartitions: make(map[string]int),
		}
		for _, member := range group.Members {
			for _, topic := range member.MemberAssignments.Topics {
				snapshot.AssignedPartitions[topic.Topic] += len(topic.Partitions)
			}
		}
		return validateConsumerGroupReadiness(c.groupID, expectedTopics, snapshot)
	}
	return fmt.Errorf("kafka consumer group %s was not described", c.groupID)
}
