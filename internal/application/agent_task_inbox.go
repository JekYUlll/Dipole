package application

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const AgentTaskInboxGoalMaxRunesV1 = 200

var (
	ErrAgentTaskInboxInvalid     = errors.New("agent task inbox request is invalid")
	ErrAgentTaskInboxDenied      = errors.New("agent task inbox access denied")
	ErrAgentTaskInboxConflict    = errors.New("agent task inbox changed concurrently")
	ErrAgentTaskInboxUnavailable = errors.New("agent task inbox is unavailable")
)

type AgentTaskInboxPendingKindV1 string

const (
	AgentTaskInboxPendingInput    AgentTaskInboxPendingKindV1 = "input"
	AgentTaskInboxPendingApproval AgentTaskInboxPendingKindV1 = "approval"
)

type AgentTaskOwnerInboxListRequestV1 struct {
	TenantID       string
	PrincipalUUID  string
	AfterUpdatedAt time.Time
	AfterTaskUUID  string
	Limit          int
}

type AgentTaskInboxItemV1 struct {
	TaskUUID    string
	Status      string
	Revision    uint64
	PendingKind AgentTaskInboxPendingKindV1
	Goal        string
	UpdatedAt   time.Time
}

type AgentTaskOwnerInboxPageV1 struct {
	Tasks         []AgentTaskInboxItemV1
	NextUpdatedAt time.Time
	NextTaskUUID  string
}

type AgentTaskOwnerInboxStoreV1 interface {
	ListOwnedTasks(ctx context.Context, request AgentTaskOwnerInboxListRequestV1) ([]AgentTaskV1, error)
}

type AgentTaskOwnerInboxServiceV1 interface {
	ListOwnedTasks(ctx context.Context, request AgentTaskOwnerInboxListRequestV1) (*AgentTaskOwnerInboxPageV1, error)
}

func AgentTaskInboxPublicStatusV1(task AgentTaskV1) string {
	if task.Workflow != nil && strings.TrimSpace(string(task.Workflow.Status)) != "" {
		return string(task.Workflow.Status)
	}
	return string(task.Status)
}

func AgentTaskInboxPendingKindV1FromStatus(status string) AgentTaskInboxPendingKindV1 {
	switch strings.TrimSpace(status) {
	case string(AgentTaskWorkflowStatusWaitingInput):
		return AgentTaskInboxPendingInput
	case string(AgentTaskWorkflowStatusWaitingApproval):
		return AgentTaskInboxPendingApproval
	default:
		return ""
	}
}

func TruncateAgentTaskInboxGoalV1(goal string) string {
	goal = strings.TrimSpace(goal)
	if utf8.RuneCountInString(goal) <= AgentTaskInboxGoalMaxRunesV1 {
		return goal
	}
	return string([]rune(goal)[:AgentTaskInboxGoalMaxRunesV1])
}

func AgentTaskInboxItemFromTaskV1(task AgentTaskV1) AgentTaskInboxItemV1 {
	status := AgentTaskInboxPublicStatusV1(task)
	revision := uint64(0)
	if task.Workflow != nil {
		revision = task.Workflow.Revision
	}
	return AgentTaskInboxItemV1{
		TaskUUID:    strings.TrimSpace(task.TaskUUID),
		Status:      status,
		Revision:    revision,
		PendingKind: AgentTaskInboxPendingKindV1FromStatus(status),
		Goal:        TruncateAgentTaskInboxGoalV1(task.Goal),
		UpdatedAt:   task.UpdatedAt.UTC(),
	}
}
