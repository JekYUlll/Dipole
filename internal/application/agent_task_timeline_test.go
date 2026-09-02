package application

import (
	"strings"
	"testing"
	"time"
)

func TestAgentTaskTimelineRepairUsesSameValidatedEventContract(t *testing.T) {
	event := AgentTaskTimelineEventV1{
		EventUUID: "model:call-1:begin", TaskUUID: "task-1", RunUUID: "run-1",
		Kind: AgentTaskTimelineEventModelCall, Status: "running", OccurredAt: time.Now().UTC(),
	}
	if err := event.Validate(); err != nil {
		t.Fatalf("valid event rejected: %v", err)
	}
	artifact := AgentTaskTimelineEventV1{EventUUID: "artifact:1:create", TaskUUID: "task-1", RunUUID: "run-1", Kind: AgentTaskTimelineEventArtifact, Status: "created", ArtifactUUID: strings.Repeat("a", 64), OccurredAt: time.Now().UTC()}
	if err := artifact.Validate(); err != nil {
		t.Fatalf("valid Artifact event rejected: %v", err)
	}
	for _, invalid := range []AgentTaskTimelineEventV1{
		{TaskUUID: event.TaskUUID, Kind: event.Kind, Status: event.Status, OccurredAt: event.OccurredAt},
		{EventUUID: strings.Repeat("e", AgentTaskTimelineEventUUIDMaxLengthV1+1), TaskUUID: event.TaskUUID, Kind: event.Kind, Status: event.Status, OccurredAt: event.OccurredAt},
		{EventUUID: event.EventUUID, TaskUUID: event.TaskUUID, Kind: AgentTaskTimelineEventKindV1("unknown"), Status: event.Status, OccurredAt: event.OccurredAt},
		{EventUUID: artifact.EventUUID, TaskUUID: artifact.TaskUUID, Kind: AgentTaskTimelineEventArtifact, Status: artifact.Status, OccurredAt: artifact.OccurredAt},
		{EventUUID: artifact.EventUUID, TaskUUID: artifact.TaskUUID, Kind: AgentTaskTimelineEventArtifact, Status: artifact.Status, ArtifactUUID: " " + artifact.ArtifactUUID, OccurredAt: artifact.OccurredAt},
		{EventUUID: event.EventUUID, TaskUUID: event.TaskUUID, Kind: event.Kind, Status: event.Status, ArtifactUUID: artifact.ArtifactUUID, OccurredAt: event.OccurredAt},
	} {
		if err := invalid.Validate(); err == nil || !strings.Contains(err.Error(), "Agent Task Timeline") {
			t.Fatalf("invalid event validation = %v", err)
		}
	}
}
