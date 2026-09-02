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

const AgentRuntimePromotionProposalVersionV1 = "dipole.agent.runtime-promotion-proposal.v1"

var (
	ErrAgentRuntimePromotionControlDenied   = errors.New("agent runtime promotion control access denied")
	ErrAgentRuntimePromotionControlConflict = errors.New("agent runtime promotion control conflict")
)

type AgentRuntimePromotionProposalStatusV1 string

const (
	AgentRuntimePromotionProposalProposed AgentRuntimePromotionProposalStatusV1 = "proposed"
	AgentRuntimePromotionProposalApproved AgentRuntimePromotionProposalStatusV1 = "approved"
	AgentRuntimePromotionProposalRejected AgentRuntimePromotionProposalStatusV1 = "rejected"
)

type AgentRuntimePromotionReviewDecisionV1 string

const (
	AgentRuntimePromotionReviewApproved AgentRuntimePromotionReviewDecisionV1 = "approved"
	AgentRuntimePromotionReviewRejected AgentRuntimePromotionReviewDecisionV1 = "rejected"
)

type AgentRuntimePromotionOperatorGrantV1 struct {
	TenantID, UserUUID, GrantedByUUID string
	CanPropose, CanReview, CanRevoke  bool
	ValidFrom                         time.Time
	ExpiresAt, RevokedAt              *time.Time
}

func (g AgentRuntimePromotionOperatorGrantV1) Active(tenantID string, at time.Time) bool {
	return g.TenantID == strings.TrimSpace(tenantID) && validPromotionIdentifierV1(g.UserUUID, 24) &&
		!g.ValidFrom.After(at) && (g.ExpiresAt == nil || g.ExpiresAt.After(at)) &&
		(g.RevokedAt == nil || g.RevokedAt.After(at))
}

type AgentRuntimePromotionProposalV1 struct {
	ProposalUUID, TenantID, RuntimeID, CandidateVersion string
	DefinitionUUID                                      string
	DefinitionVersion                                   uint64
	EvidenceArtifactUUID, EvidenceSHA256                string
	EvalSuiteSHA256                                     string
	ProposerUUID, TicketRef, Reason                     string
	Status                                              AgentRuntimePromotionProposalStatusV1
	GrantUUID                                           string
	ProposedAt, ExpiresAt                               time.Time
	GrantValidFrom, GrantExpiresAt                      time.Time
	DecidedAt                                           *time.Time
}

type AgentRuntimePromotionProposalRequestV1 struct {
	TenantID, RuntimeID, CandidateVersion string
	DefinitionUUID                        string
	DefinitionVersion                     uint64
	EvidenceArtifactUUID, EvidenceSHA256  string
	EvalSuiteSHA256                       string
	TicketRef, Reason                     string
	ProposedAt, ExpiresAt                 time.Time
	GrantValidFrom, GrantExpiresAt        time.Time
}

type AgentRuntimePromotionRevocationV1 struct {
	GrantUUID, TenantID, RevokedByUUID, TicketRef, Reason string
	RevokedAt                                             time.Time
}

type AgentRuntimePromotionControlStoreV1 interface {
	GetRuntimePromotionOperatorGrant(context.Context, string, string) (*AgentRuntimePromotionOperatorGrantV1, error)
	CreateRuntimePromotionProposal(context.Context, AgentRuntimePromotionProposalV1) (bool, error)
	GetRuntimePromotionProposal(context.Context, string) (*AgentRuntimePromotionProposalV1, error)
	ReviewRuntimePromotionProposal(context.Context, string, string, AgentRuntimePromotionReviewDecisionV1, time.Time) (*AgentRuntimePromotionProposalV1, error)
	RevokeRuntimePromotionGrantAudited(context.Context, AgentRuntimePromotionRevocationV1) (*AgentRuntimePromotionGrantV1, error)
}

type AgentRuntimePromotionControlServiceV1 interface {
	Propose(context.Context, string, AgentRuntimePromotionProposalRequestV1) (*AgentRuntimePromotionProposalV1, error)
	Review(context.Context, string, string, AgentRuntimePromotionReviewDecisionV1) (*AgentRuntimePromotionProposalV1, error)
	Get(context.Context, string, string, string) (*AgentRuntimePromotionProposalV1, error)
	Revoke(context.Context, string, string, string, string) (*AgentRuntimePromotionGrantV1, error)
}

type AgentRuntimePromotionEvidenceReviewV1 struct {
	Proposal *AgentRuntimePromotionProposalV1
	Artifact *AgentArtifactV1
	Content  []byte
}

type AgentRuntimePromotionEvidenceReaderV1 interface {
	ReadPromotionEvidence(context.Context, string, string) (*AgentArtifactV1, []byte, error)
}

type AgentRuntimePromotionEvidenceReviewServiceV1 interface {
	Get(context.Context, string, string, string) (*AgentRuntimePromotionEvidenceReviewV1, error)
}

func NewAgentRuntimePromotionProposalV1(operatorUUID string, request AgentRuntimePromotionProposalRequestV1) (*AgentRuntimePromotionProposalV1, error) {
	operatorUUID = strings.TrimSpace(operatorUUID)
	request.TenantID, request.RuntimeID = strings.TrimSpace(request.TenantID), strings.TrimSpace(request.RuntimeID)
	request.CandidateVersion, request.DefinitionUUID = strings.TrimSpace(request.CandidateVersion), strings.TrimSpace(request.DefinitionUUID)
	request.EvidenceArtifactUUID, request.EvidenceSHA256 = strings.TrimSpace(request.EvidenceArtifactUUID), strings.ToLower(strings.TrimSpace(request.EvidenceSHA256))
	request.EvalSuiteSHA256, request.TicketRef, request.Reason = strings.ToLower(strings.TrimSpace(request.EvalSuiteSHA256)), strings.TrimSpace(request.TicketRef), strings.TrimSpace(request.Reason)
	proposedAt, expiresAt := request.ProposedAt.UTC().Truncate(time.Millisecond), request.ExpiresAt.UTC().Truncate(time.Millisecond)
	validFrom, grantExpiresAt := request.GrantValidFrom.UTC().Truncate(time.Millisecond), request.GrantExpiresAt.UTC().Truncate(time.Millisecond)
	if !validPromotionIdentifierV1(operatorUUID, 24) || !validPromotionIdentifierV1(request.TenantID, 64) ||
		!validPromotionIdentifierV1(request.RuntimeID, 64) || !validPromotionIdentifierV1(request.CandidateVersion, 128) ||
		!validPromotionIdentifierV1(request.DefinitionUUID, 64) || request.DefinitionVersion == 0 ||
		!validSHA256V1(request.EvidenceArtifactUUID) || !validSHA256V1(request.EvidenceSHA256) || !validSHA256V1(request.EvalSuiteSHA256) ||
		!validPromotionIdentifierV1(request.TicketRef, 128) || strings.TrimSpace(request.Reason) == "" || len(request.Reason) > 1000 ||
		proposedAt.IsZero() || !expiresAt.After(proposedAt) || expiresAt.Sub(proposedAt) > time.Hour ||
		validFrom.IsZero() || validFrom.Before(proposedAt) || !grantExpiresAt.After(validFrom) || grantExpiresAt.Sub(validFrom) > 30*24*time.Hour {
		return nil, fmt.Errorf("%w: promotion proposal is invalid", ErrAgentRuntimePromotionControlConflict)
	}
	canonical := struct {
		SchemaVersion, TenantID, RuntimeID, CandidateVersion, DefinitionUUID string
		DefinitionVersion                                                    uint64
		EvidenceArtifactID, EvidenceSHA256, EvalSuiteSHA256                  string
		ProposerID, TicketRef, Reason                                        string
		ProposedAt, ExpiresAt, GrantValidFrom, GrantExpiresAt                string
	}{AgentRuntimePromotionProposalVersionV1, request.TenantID, request.RuntimeID, request.CandidateVersion, request.DefinitionUUID,
		request.DefinitionVersion, request.EvidenceArtifactUUID, request.EvidenceSHA256, request.EvalSuiteSHA256,
		operatorUUID, request.TicketRef, request.Reason, isoMillisecondsV1(proposedAt), isoMillisecondsV1(expiresAt), isoMillisecondsV1(validFrom), isoMillisecondsV1(grantExpiresAt)}
	payload, err := json.Marshal(canonical)
	if err != nil {
		return nil, fmt.Errorf("marshal Runtime promotion proposal: %w", err)
	}
	digest := sha256.Sum256(payload)
	return &AgentRuntimePromotionProposalV1{ProposalUUID: hex.EncodeToString(digest[:]), TenantID: request.TenantID,
		RuntimeID: request.RuntimeID, CandidateVersion: request.CandidateVersion, DefinitionUUID: request.DefinitionUUID,
		DefinitionVersion: request.DefinitionVersion, EvidenceArtifactUUID: request.EvidenceArtifactUUID,
		EvidenceSHA256: request.EvidenceSHA256, EvalSuiteSHA256: request.EvalSuiteSHA256, ProposerUUID: operatorUUID,
		TicketRef: request.TicketRef, Reason: request.Reason, Status: AgentRuntimePromotionProposalProposed,
		ProposedAt: proposedAt, ExpiresAt: expiresAt, GrantValidFrom: validFrom, GrantExpiresAt: grantExpiresAt}, nil
}

func (p AgentRuntimePromotionProposalV1) Grant(reviewerUUID string) AgentRuntimePromotionGrantV1 {
	digest := sha256.Sum256([]byte(strings.Join([]string{p.ProposalUUID, strings.TrimSpace(reviewerUUID), AgentRuntimePromotionPolicyVersionV2}, "\n")))
	return AgentRuntimePromotionGrantV1{GrantUUID: hex.EncodeToString(digest[:]), TenantID: p.TenantID, RuntimeID: p.RuntimeID,
		CandidateVersion: p.CandidateVersion, DefinitionUUID: p.DefinitionUUID, DefinitionVersion: p.DefinitionVersion,
		PolicyVersion: AgentRuntimePromotionPolicyVersionV2, EvidenceSHA256: p.EvidenceSHA256, EvalSuiteSHA256: p.EvalSuiteSHA256,
		GrantedByUUID: p.ProposerUUID, ReviewedByUUID: strings.TrimSpace(reviewerUUID), ValidFrom: p.GrantValidFrom, ExpiresAt: p.GrantExpiresAt}
}
