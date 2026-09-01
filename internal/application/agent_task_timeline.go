package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	AgentTaskTimelineSchemaVersionV1      = "dipole.agent.task_timeline.v1"
	AgentTaskTimelineEventUUIDMaxLengthV1 = 64
)

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
	ArtifactUUID string
	OccurredAt   time.Time
}

type AgentTaskTimelineStoreV1 interface {
	AppendAgentTaskTimelineEvent(context.Context, AgentTaskTimelineEventV1) (uint64, error)
	ListAgentTaskTimelineEvents(context.Context, string, uint64, int) ([]AgentTaskTimelineEventV1, error)
}

type AgentTaskTimelineRepairV1 struct {
	EventUUID    string
	TaskUUID     string
	RunUUID      string
	Kind         AgentTaskTimelineEventKindV1
	Status       string
	CapabilityID string
	ApprovalUUID string
	ArtifactUUID string
	OccurredAt   time.Time
	RepairStatus string
	RetryCount   uint32
	LastError    string
	NextRetryAt  *time.Time
	LockedAt     *time.Time
}

type AgentTaskTimelineRepairStoreV1 interface {
	EnqueueAgentTaskTimelineRepair(context.Context, AgentTaskTimelineEventV1, error) error
	ClaimAgentTaskTimelineRepairs(int, time.Time, time.Duration) ([]AgentTaskTimelineRepairV1, error)
	MarkAgentTaskTimelineRepairCompleted(string) error
	MarkAgentTaskTimelineRepairRetry(string, uint32, time.Time, error) error
}

func (e AgentTaskTimelineEventV1) Validate() error {
	if strings.TrimSpace(e.EventUUID) == "" || strings.TrimSpace(e.TaskUUID) == "" || strings.TrimSpace(e.Status) == "" || e.OccurredAt.IsZero() {
		return errors.New("Agent Task Timeline event identity, status and timestamp are required")
	}
	if len(e.EventUUID) > AgentTaskTimelineEventUUIDMaxLengthV1 {
		return fmt.Errorf("Agent Task Timeline event ID exceeds %d characters", AgentTaskTimelineEventUUIDMaxLengthV1)
	}
	switch e.Kind {
	case AgentTaskTimelineEventTask, AgentTaskTimelineEventRun, AgentTaskTimelineEventContextCompile,
		AgentTaskTimelineEventModelCall, AgentTaskTimelineEventToolInvocation, AgentTaskTimelineEventApproval,
		AgentTaskTimelineEventInputRequest, AgentTaskTimelineEventArtifact, AgentTaskTimelineEventTerminal:
	default:
		return fmt.Errorf("unsupported Agent Task Timeline event kind %q", e.Kind)
	}
	artifactUUID := strings.TrimSpace(e.ArtifactUUID)
	if artifactUUID == "" {
		if e.Kind == AgentTaskTimelineEventArtifact {
			return errors.New("Agent Task Timeline artifact event requires an Artifact ID")
		}
		return nil
	}
	if artifactUUID != e.ArtifactUUID || e.Kind != AgentTaskTimelineEventArtifact || !validSHA256V1(artifactUUID) {
		return errors.New("Agent Task Timeline Artifact ID is invalid")
	}
	return nil
}
