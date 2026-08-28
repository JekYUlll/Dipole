package application

import (
	"strings"
	"testing"
	"time"
)

func TestAgentTaskTimelineEventValidatesLowSensitivityIdentity(t *testing.T) {
	valid := AgentTaskTimelineEventV1{
		EventUUID: "EVT-1", TaskUUID: "TASK-1", RunUUID: "RUN-1",
		Kind: AgentTaskTimelineEventToolInvocation, Status: "completed", OccurredAt: time.UnixMilli(1),
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid event rejected: %v", err)
	}
	for _, invalid := range []AgentTaskTimelineEventV1{
		{EventUUID: "EVT-1", TaskUUID: "TASK-1", Kind: "prompt", Status: "completed", OccurredAt: valid.OccurredAt},
		{EventUUID: "EVT-1", TaskUUID: "TASK-1", Kind: AgentTaskTimelineEventTask, Status: "", OccurredAt: valid.OccurredAt},
		{EventUUID: strings.Repeat("x", 1), TaskUUID: "TASK-1", Kind: AgentTaskTimelineEventTask},
	} {
		if err := invalid.Validate(); err == nil {
			t.Fatalf("invalid event accepted: %+v", invalid)
		}
	}
}
