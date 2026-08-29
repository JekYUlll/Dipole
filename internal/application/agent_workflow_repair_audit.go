package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	AgentWorkflowRepairProposalVersionV1 = "dipole.agent.workflow-repair-proposal.v1"
	AgentWorkflowRepairEvidenceVersionV1 = "dipole.agent.workflow-repair-evidence.v1"
	AgentWorkflowRepairActionV1          = "reproject_from_temporal"
)

var (
	ErrAgentWorkflowRepairDenied   = errors.New("agent workflow repair access denied")
	ErrAgentWorkflowRepairConflict = errors.New("agent workflow repair conflict")
)

type AgentWorkflowRepairOutcomeV1 string

const (
	AgentWorkflowRepairOutcomeMissing  AgentWorkflowRepairOutcomeV1 = "missing"
	AgentWorkflowRepairOutcomeStale    AgentWorkflowRepairOutcomeV1 = "stale"
	AgentWorkflowRepairOutcomeAhead    AgentWorkflowRepairOutcomeV1 = "ahead"
	AgentWorkflowRepairOutcomeConflict AgentWorkflowRepairOutcomeV1 = "conflict"
)

type AgentWorkflowRepairStatusV1 string

const (
	AgentWorkflowRepairStatusProposed AgentWorkflowRepairStatusV1 = "proposed"
	AgentWorkflowRepairStatusApproved AgentWorkflowRepairStatusV1 = "approved"
	AgentWorkflowRepairStatusRejected AgentWorkflowRepairStatusV1 = "rejected"
	AgentWorkflowRepairStatusExpired  AgentWorkflowRepairStatusV1 = "expired"
)

type AgentWorkflowRepairDecisionV1 string

const (
	AgentWorkflowRepairDecisionApproved AgentWorkflowRepairDecisionV1 = "approved"
	AgentWorkflowRepairDecisionRejected AgentWorkflowRepairDecisionV1 = "rejected"
)

type AgentWorkflowEvidenceV1 struct {
	WorkflowID    string `json:"workflowId"`
	WorkflowRunID string `json:"workflowRunId"`
	Status        string `json:"status"`
	Revision      uint64 `json:"revision"`
}

func (e AgentWorkflowEvidenceV1) Validate() error {
	if anyBlank(e.WorkflowID, e.WorkflowRunID, e.Status) {
		return fmt.Errorf("%w: workflow evidence identity is required", ErrAgentWorkflowRepairConflict)
	}
	return nil
}

type AgentWorkflowRepairOperatorGrantV1 struct {
	UserUUID, GrantedByUUID string
	Version                 uint64
	CanPropose, CanApprove  bool
	CanExecute              bool
	ValidFrom               time.Time
	ExpiresAt, RevokedAt    *time.Time
}

func (g AgentWorkflowRepairOperatorGrantV1) Active(at time.Time) bool {
	return strings.TrimSpace(g.UserUUID) != "" && !g.ValidFrom.After(at) &&
		(g.ExpiresAt == nil || g.ExpiresAt.After(at)) && (g.RevokedAt == nil || g.RevokedAt.After(at))
}

type AgentWorkflowRepairProposalV1 struct {
	ProposalUUID          string
	TaskUUID              string
	Outcome               AgentWorkflowRepairOutcomeV1
	Action                string
	ProposerUUID          string
	TicketRef             string
	Reason                string
	Projected             *AgentWorkflowEvidenceV1
	Temporal              AgentWorkflowEvidenceV1
	EvidenceSHA256        string
	Status                AgentWorkflowRepairStatusV1
	RequiredApprovals     uint8
	ProposedAt, ExpiresAt time.Time
	DecidedAt             *time.Time
}

type AgentWorkflowRepairProposalRequestV1 struct {
	TaskUUID              string
	Outcome               AgentWorkflowRepairOutcomeV1
	TicketRef, Reason     string
	Projected             *AgentWorkflowEvidenceV1
	Temporal              AgentWorkflowEvidenceV1
	ProposedAt, ExpiresAt time.Time
}

type AgentWorkflowRepairDecisionRecordV1 struct {
	ProposalUUID, ApproverUUID string
	Decision                   AgentWorkflowRepairDecisionV1
	DecidedAt                  time.Time
}

type AgentWorkflowRepairDecisionCountsV1 struct{ Approved, Rejected uint64 }

type AgentWorkflowRepairAuditStoreV1 interface {
	GetWorkflowRepairOperatorGrant(context.Context, string) (*AgentWorkflowRepairOperatorGrantV1, error)
	CreateWorkflowRepairProposal(context.Context, AgentWorkflowRepairProposalV1) (bool, error)
	GetWorkflowRepairProposal(context.Context, string) (*AgentWorkflowRepairProposalV1, error)
	RecordWorkflowRepairDecision(context.Context, AgentWorkflowRepairDecisionRecordV1) (bool, error)
	GetWorkflowRepairDecision(context.Context, string, string) (*AgentWorkflowRepairDecisionRecordV1, error)
	CountWorkflowRepairDecisions(context.Context, string) (AgentWorkflowRepairDecisionCountsV1, error)
	FinalizeWorkflowRepairProposal(context.Context, string) error
}

type AgentWorkflowRepairAuditServiceV1 interface {
	Propose(context.Context, string, AgentWorkflowRepairProposalRequestV1) (*AgentWorkflowRepairProposalV1, error)
	Decide(context.Context, string, string, AgentWorkflowRepairDecisionV1) (*AgentWorkflowRepairProposalV1, error)
	Get(context.Context, string, string) (*AgentWorkflowRepairProposalV1, error)
}

func NewAgentWorkflowRepairProposalV1(operatorUUID string, request AgentWorkflowRepairProposalRequestV1) (*AgentWorkflowRepairProposalV1, error) {
	operatorUUID, request.TaskUUID = strings.TrimSpace(operatorUUID), strings.TrimSpace(request.TaskUUID)
	request.TicketRef, request.Reason = strings.TrimSpace(request.TicketRef), strings.TrimSpace(request.Reason)
	if anyBlank(operatorUUID, request.TaskUUID, request.TicketRef, request.Reason) || !validRepairOutcomeV1(request.Outcome) {
		return nil, fmt.Errorf("%w: repair proposal identity is invalid", ErrAgentWorkflowRepairConflict)
	}
	if err := request.Temporal.Validate(); err != nil {
		return nil, err
	}
	if request.Projected != nil {
		if err := request.Projected.Validate(); err != nil {
			return nil, err
		}
	}
	proposedAt, expiresAt := request.ProposedAt.UTC().Truncate(time.Millisecond), request.ExpiresAt.UTC().Truncate(time.Millisecond)
	if proposedAt.IsZero() || !expiresAt.After(proposedAt) || expiresAt.Sub(proposedAt) > time.Hour {
		return nil, fmt.Errorf("%w: repair proposal must expire within one hour", ErrAgentWorkflowRepairConflict)
	}
	evidence := struct {
		SchemaVersion                 string                       `json:"schemaVersion"`
		TaskID                        string                       `json:"taskId"`
		Outcome                       AgentWorkflowRepairOutcomeV1 `json:"outcome"`
		OperatorID, TicketRef, Reason string
		Projected                     *AgentWorkflowEvidenceV1 `json:"projected"`
		Temporal                      AgentWorkflowEvidenceV1  `json:"temporal"`
		ProposedAt                    string                   `json:"proposedAt"`
		ExpiresAt                     string                   `json:"expiresAt"`
	}{AgentWorkflowRepairEvidenceVersionV1, request.TaskUUID, request.Outcome, operatorUUID, request.TicketRef, request.Reason, request.Projected, request.Temporal, isoMillisecondsV1(proposedAt), isoMillisecondsV1(expiresAt)}
	// Explicit tags preserve the canonical TypeScript property names and order.
	canonical := struct {
		SchemaVersion string                       `json:"schemaVersion"`
		TaskID        string                       `json:"taskId"`
		Outcome       AgentWorkflowRepairOutcomeV1 `json:"outcome"`
		OperatorID    string                       `json:"operatorId"`
		TicketRef     string                       `json:"ticketRef"`
		Reason        string                       `json:"reason"`
		Projected     *AgentWorkflowEvidenceV1     `json:"projected"`
		Temporal      AgentWorkflowEvidenceV1      `json:"temporal"`
		ProposedAt    string                       `json:"proposedAt"`
		ExpiresAt     string                       `json:"expiresAt"`
	}{evidence.SchemaVersion, evidence.TaskID, evidence.Outcome, evidence.OperatorID, evidence.TicketRef, evidence.Reason, evidence.Projected, evidence.Temporal, evidence.ProposedAt, evidence.ExpiresAt}
	payload, err := json.Marshal(canonical)
	if err != nil {
		return nil, fmt.Errorf("marshal repair evidence: %w", err)
	}
	digest := sha256.Sum256(payload)
	hash := hex.EncodeToString(digest[:])
	return &AgentWorkflowRepairProposalV1{ProposalUUID: "repair:" + hash, TaskUUID: request.TaskUUID, Outcome: request.Outcome,
		Action: AgentWorkflowRepairActionV1, ProposerUUID: operatorUUID, TicketRef: request.TicketRef, Reason: request.Reason,
		Projected: request.Projected, Temporal: request.Temporal, EvidenceSHA256: hash, Status: AgentWorkflowRepairStatusProposed,
		RequiredApprovals: 2, ProposedAt: proposedAt, ExpiresAt: expiresAt}, nil
}

func validRepairOutcomeV1(value AgentWorkflowRepairOutcomeV1) bool {
	return value == AgentWorkflowRepairOutcomeMissing || value == AgentWorkflowRepairOutcomeStale || value == AgentWorkflowRepairOutcomeAhead || value == AgentWorkflowRepairOutcomeConflict
}

func isoMillisecondsV1(value time.Time) string { return value.UTC().Format("2006-01-02T15:04:05.000Z") }
