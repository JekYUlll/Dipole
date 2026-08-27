package application

import (
	"context"
	"errors"
	"math"
	"regexp"
	"strings"
	"time"
)

var (
	ErrAgentToolInvocationInvalid  = errors.New("agent tool invocation invalid")
	ErrAgentToolInvocationDenied   = errors.New("agent tool invocation denied")
	ErrAgentToolInvocationConflict = errors.New("agent tool invocation conflict")
)

type AgentToolInvocationStatusV1 string
type AgentToolTransportV1 string
type AgentToolActionResourceTypeV1 string

const (
	AgentToolInvocationStatusRunning   AgentToolInvocationStatusV1   = "running"
	AgentToolInvocationStatusCompleted AgentToolInvocationStatusV1   = "completed"
	AgentToolInvocationStatusFailed    AgentToolInvocationStatusV1   = "failed"
	AgentToolTransportMCP              AgentToolTransportV1          = "mcp"
	AgentToolActionResourceMessage     AgentToolActionResourceTypeV1 = "message"
)

type AgentToolActionReferenceV1 struct {
	ResourceType AgentToolActionResourceTypeV1
	ResourceUUID string
	CommandKind  AgentMessageCommandKindV1
	CommandID    string
}

type AgentToolInvocationV1 struct {
	InvocationUUID  string
	TenantID        string
	PrincipalUUID   string
	AgentUUID       string
	TaskUUID        string
	RunUUID         string
	Transport       AgentToolTransportV1
	ToolName        string
	CapabilityID    string
	ArgumentsSHA256 string
	Status          AgentToolInvocationStatusV1
	RequestID       string
	TraceID         string
	ApprovalUUID    string
	StartedAt       time.Time
}

type AgentToolInvocationBeginV1 struct {
	InvocationUUID  string
	TaskUUID        string
	RunUUID         string
	Transport       AgentToolTransportV1
	ToolName        string
	CapabilityID    string
	ArgumentsSHA256 string
	RequestID       string
	TraceID         string
	ApprovalUUID    string
}

type AgentToolInvocationFinishV1 struct {
	InvocationUUID  string
	TaskUUID        string
	RunUUID         string
	Status          AgentToolInvocationStatusV1
	ResultSHA256    string
	ResultBytes     uint64
	LatencyMS       uint64
	ErrorCode       string
	ActionReference *AgentToolActionReferenceV1
}

type AgentToolInvocationReaderV1 interface {
	GetToolInvocation(ctx context.Context, invocationUUID string) (*AgentToolInvocationV1, error)
}

type AgentToolInvocationStoreV1 interface {
	AgentToolInvocationReaderV1
	BeginToolInvocation(ctx context.Context, invocation AgentToolInvocationV1) (bool, error)
	FinishToolInvocation(ctx context.Context, finish AgentToolInvocationFinishV1) (bool, error)
}

type AgentToolApprovalReaderV1 interface {
	GetApproval(ctx context.Context, approvalUUID string) (*AgentApprovalV1, error)
	GetRun(ctx context.Context, runUUID string) (*AgentRunV1, error)
}

type AgentToolInvocationAuditServiceV1 interface {
	Begin(ctx context.Context, begin AgentToolInvocationBeginV1) (*AgentToolInvocationV1, error)
	Finish(ctx context.Context, finish AgentToolInvocationFinishV1) error
}

var agentToolNamePatternV1 = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.-]{0,63}$`)
var agentToolSHA256PatternV1 = regexp.MustCompile(`^[a-f0-9]{64}$`)
var agentToolErrorCodePatternV1 = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

func (v AgentToolInvocationBeginV1) Validate() error {
	if !validAgentToolIdentifierV1(v.InvocationUUID, 64) || !validAgentToolIdentifierV1(v.TaskUUID, 64) || !validAgentToolIdentifierV1(v.RunUUID, 64) ||
		v.Transport != AgentToolTransportMCP || !agentToolNamePatternV1.MatchString(strings.TrimSpace(v.ToolName)) ||
		!validAgentToolIdentifierV1(v.CapabilityID, 128) || !agentToolSHA256PatternV1.MatchString(strings.TrimSpace(v.ArgumentsSHA256)) ||
		!validOptionalAgentToolValueV1(v.RequestID, 128) || !validOptionalAgentToolValueV1(v.TraceID, 128) || !validOptionalAgentToolValueV1(v.ApprovalUUID, 64) {
		return ErrAgentToolInvocationInvalid
	}
	return nil
}

func (v AgentToolInvocationFinishV1) Validate() error {
	if !validAgentToolIdentifierV1(v.InvocationUUID, 64) || !validAgentToolIdentifierV1(v.TaskUUID, 64) || !validAgentToolIdentifierV1(v.RunUUID, 64) {
		return ErrAgentToolInvocationInvalid
	}
	switch v.Status {
	case AgentToolInvocationStatusCompleted:
		if !agentToolSHA256PatternV1.MatchString(strings.TrimSpace(v.ResultSHA256)) || v.ErrorCode != "" || v.ResultBytes > math.MaxInt64 || v.LatencyMS > math.MaxInt64 {
			return ErrAgentToolInvocationInvalid
		}
		if v.ActionReference != nil && v.ActionReference.Validate() != nil {
			return ErrAgentToolInvocationInvalid
		}
	case AgentToolInvocationStatusFailed:
		if v.ResultSHA256 != "" || v.ResultBytes != 0 || v.LatencyMS > math.MaxInt64 || !agentToolErrorCodePatternV1.MatchString(strings.TrimSpace(v.ErrorCode)) || v.ActionReference != nil {
			return ErrAgentToolInvocationInvalid
		}
	default:
		return ErrAgentToolInvocationInvalid
	}
	return nil
}

func (v AgentToolActionReferenceV1) Validate() error {
	if v.ResourceType != AgentToolActionResourceMessage || v.ResourceUUID != strings.TrimSpace(v.ResourceUUID) || v.CommandID != strings.TrimSpace(v.CommandID) ||
		!validAgentToolIdentifierV1(v.ResourceUUID, 64) || !validAgentToolIdentifierV1(v.CommandID, 128) {
		return ErrAgentToolInvocationInvalid
	}
	switch v.CommandKind {
	case AgentMessageCommandAssistantReplyV1, AgentMessageCommandSystemMessageV1:
		return nil
	default:
		return ErrAgentToolInvocationInvalid
	}
}

func validAgentToolIdentifierV1(value string, limit int) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed != "" && len(value) <= limit && len(trimmed) <= limit
}

func validOptionalAgentToolValueV1(value string, limit int) bool {
	return len(value) <= limit
}
