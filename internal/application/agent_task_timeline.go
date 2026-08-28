package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const AgentTaskTimelineSchemaVersionV1 = "dipole.agent.task_timeline.v1"

type AgentTaskTimelineEventKindV1 string

const (
	AgentTaskTimelineEventTask           AgentTaskTimelineEventKindV1 = "task"
	AgentTaskTimelineEventRun            AgentTaskTimelineEventKindV1 = "run"
	AgentTaskTimelineEventContextCompile AgentTaskTimelineEventKindV1 = "context_compile"
	AgentTaskTimelineEventModelCall      AgentTaskTimelineEventKindV1 = "model_call"
	AgentTaskTimelineEventToolInvocation AgentTaskTimelineEventKindV1 = "tool_invocation"
	AgentTaskTimelineEventApproval       AgentTaskTimelineEventKindV1 = "approval"
	AgentTaskTimelineEventInputRequest   AgentTaskTimelineEventKindV1 = "input_request"
	AgentTaskTimelineEventArtifact       AgentTaskTimelineEventKindV1 = "artifact"
	AgentTaskTimelineEventTerminal       AgentTaskTimelineEventKindV1 = "terminal"
)

type AgentTaskTimelineEventV1 struct {
	EventSeq     uint64
	EventUUID    string
	TaskUUID     string
	RunUUID      string
	Kind         AgentTaskTimelineEventKindV1
	Status       string
	CapabilityID string
	ApprovalUUID string
	OccurredAt   time.Time
}

type AgentTaskTimelineStoreV1 interface {
	AppendAgentTaskTimelineEvent(context.Context, AgentTaskTimelineEventV1) (uint64, error)
	ListAgentTaskTimelineEvents(context.Context, string, uint64, int) ([]AgentTaskTimelineEventV1, error)
}

func (e AgentTaskTimelineEventV1) Validate() error {
	if strings.TrimSpace(e.EventUUID) == "" || strings.TrimSpace(e.TaskUUID) == "" || strings.TrimSpace(e.Status) == "" || e.OccurredAt.IsZero() {
		return errors.New("Agent Task Timeline event identity, status and timestamp are required")
	}
	switch e.Kind {
	case AgentTaskTimelineEventTask, AgentTaskTimelineEventRun, AgentTaskTimelineEventContextCompile,
		AgentTaskTimelineEventModelCall, AgentTaskTimelineEventToolInvocation, AgentTaskTimelineEventApproval,
		AgentTaskTimelineEventInputRequest, AgentTaskTimelineEventArtifact, AgentTaskTimelineEventTerminal:
		return nil
	default:
		return fmt.Errorf("unsupported Agent Task Timeline event kind %q", e.Kind)
	}
}
