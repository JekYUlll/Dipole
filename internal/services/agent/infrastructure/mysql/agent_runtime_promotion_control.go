package agentmysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	mysqlData "github.com/JekYUlll/Dipole/internal/platform/mysql"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
)

type AgentRuntimePromotionControlRepository struct{ store *mysqlData.Store }

var _ application.AgentRuntimePromotionControlStoreV1 = (*AgentRuntimePromotionControlRepository)(nil)

func NewAgentRuntimePromotionControlRepository(store *mysqlData.Store) (*AgentRuntimePromotionControlRepository, error) {
	if store == nil {
		return nil, errors.New("Agent Runtime promotion control transaction Store is required")
	}
	return &AgentRuntimePromotionControlRepository{store: store}, nil
}

func (r *AgentRuntimePromotionControlRepository) GetRuntimePromotionOperatorGrant(ctx context.Context, tenantID, userUUID string) (*application.AgentRuntimePromotionOperatorGrantV1, error) {
	return getRuntimePromotionOperatorGrantV1(ctx, r.store.Queries(), tenantID, userUUID)
}

func getRuntimePromotionOperatorGrantV1(ctx context.Context, queries *generated.Queries, tenantID, userUUID string) (*application.AgentRuntimePromotionOperatorGrantV1, error) {
	row, err := queries.GetAgentRuntimePromotionOperatorGrant(ctx, generated.GetAgentRuntimePromotionOperatorGrantParams{TenantID: strings.TrimSpace(tenantID), UserUuid: strings.TrimSpace(userUUID)})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get Runtime promotion operator grant: %w", err)
	}
	return &application.AgentRuntimePromotionOperatorGrantV1{TenantID: row.TenantID, UserUUID: row.UserUuid, GrantedByUUID: row.GrantedByUuid,
		CanPropose: row.CanPropose, CanReview: row.CanReview, CanRevoke: row.CanRevoke, ValidFrom: row.ValidFrom,
		ExpiresAt: timePointer(row.ExpiresAt), RevokedAt: timePointer(row.RevokedAt)}, nil
}

func (r *AgentRuntimePromotionControlRepository) CreateRuntimePromotionProposal(ctx context.Context, proposal application.AgentRuntimePromotionProposalV1) (bool, error) {
	if proposal.Status != application.AgentRuntimePromotionProposalProposed || proposal.GrantUUID != "" || proposal.DecidedAt != nil {
		return false, fmt.Errorf("validate Runtime promotion proposal: %w", application.ErrAgentRuntimePromotionControlConflict)
	}
	rows, err := r.store.Queries().InsertAgentRuntimePromotionProposal(ctx, generated.InsertAgentRuntimePromotionProposalParams{
		ProposalUuid: proposal.ProposalUUID, TenantID: proposal.TenantID, RuntimeID: proposal.RuntimeID, CandidateVersion: proposal.CandidateVersion,
		DefinitionUuid: proposal.DefinitionUUID, DefinitionVersion: proposal.DefinitionVersion, EvidenceArtifactUuid: proposal.EvidenceArtifactUUID,
		EvidenceSha256: proposal.EvidenceSHA256, EvalSuiteSha256: proposal.EvalSuiteSHA256, ProposerUuid: proposal.ProposerUUID,
		TicketRef: proposal.TicketRef, Reason: proposal.Reason, ProposedAt: proposal.ProposedAt, ExpiresAt: proposal.ExpiresAt,
		GrantValidFrom: proposal.GrantValidFrom, GrantExpiresAt: proposal.GrantExpiresAt,
	})
	if err != nil {
		return false, fmt.Errorf("create Runtime promotion proposal: %w", err)
	}
	if rows > 0 {
		return true, nil
	}
	existing, err := r.GetRuntimePromotionProposal(ctx, proposal.ProposalUUID)
	if err != nil {
		return false, err
	}
	if existing == nil || !sameRuntimePromotionProposalV1(*existing, proposal) {
		return false, fmt.Errorf("%w: proposal UUID=%s", application.ErrAgentRuntimePromotionControlConflict, proposal.ProposalUUID)
	}
	return false, nil
}

func (r *AgentRuntimePromotionControlRepository) GetRuntimePromotionProposal(ctx context.Context, proposalUUID string) (*application.AgentRuntimePromotionProposalV1, error) {
	row, err := r.store.Queries().GetAgentRuntimePromotionProposal(ctx, strings.TrimSpace(proposalUUID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get Runtime promotion proposal: %w", err)
	}
	return mapRuntimePromotionProposalV1(row), nil
}

func mapRuntimePromotionProposalV1(row generated.AgentRuntimePromotionProposal) *application.AgentRuntimePromotionProposalV1 {
	return &application.AgentRuntimePromotionProposalV1{ProposalUUID: row.ProposalUuid, TenantID: row.TenantID, RuntimeID: row.RuntimeID,
		CandidateVersion: row.CandidateVersion, DefinitionUUID: row.DefinitionUuid, DefinitionVersion: row.DefinitionVersion,
		EvidenceArtifactUUID: row.EvidenceArtifactUuid, EvidenceSHA256: row.EvidenceSha256, EvalSuiteSHA256: row.EvalSuiteSha256,
		ProposerUUID: row.ProposerUuid, TicketRef: row.TicketRef, Reason: row.Reason, Status: application.AgentRuntimePromotionProposalStatusV1(row.Status),
		GrantUUID: row.GrantUuid.String, ProposedAt: row.ProposedAt, ExpiresAt: row.ExpiresAt, GrantValidFrom: row.GrantValidFrom,
		GrantExpiresAt: row.GrantExpiresAt, DecidedAt: timePointer(row.DecidedAt)}
}

func sameRuntimePromotionProposalV1(left, right application.AgentRuntimePromotionProposalV1) bool {
	return left.ProposalUUID == right.ProposalUUID && left.TenantID == right.TenantID && left.RuntimeID == right.RuntimeID &&
		left.CandidateVersion == right.CandidateVersion && left.DefinitionUUID == right.DefinitionUUID && left.DefinitionVersion == right.DefinitionVersion &&
		left.EvidenceArtifactUUID == right.EvidenceArtifactUUID && left.EvidenceSHA256 == right.EvidenceSHA256 && left.EvalSuiteSHA256 == right.EvalSuiteSHA256 &&
		left.ProposerUUID == right.ProposerUUID && left.TicketRef == right.TicketRef && left.Reason == right.Reason && left.Status == right.Status &&
		left.GrantUUID == right.GrantUUID && left.ProposedAt.Equal(right.ProposedAt) && left.ExpiresAt.Equal(right.ExpiresAt) &&
		left.GrantValidFrom.Equal(right.GrantValidFrom) && left.GrantExpiresAt.Equal(right.GrantExpiresAt)
}

func (r *AgentRuntimePromotionControlRepository) ReviewRuntimePromotionProposal(ctx context.Context, proposalUUID, reviewerUUID string, decision application.AgentRuntimePromotionReviewDecisionV1, decidedAt time.Time) (*application.AgentRuntimePromotionProposalV1, error) {
	proposalUUID, reviewerUUID = strings.TrimSpace(proposalUUID), strings.TrimSpace(reviewerUUID)
	if proposalUUID == "" || reviewerUUID == "" || decidedAt.IsZero() || (decision != application.AgentRuntimePromotionReviewApproved && decision != application.AgentRuntimePromotionReviewRejected) {
		return nil, fmt.Errorf("validate Runtime promotion review: %w", application.ErrAgentRuntimePromotionControlConflict)
	}
	err := r.store.WithinTx(ctx, nil, func(q *generated.Queries) error {
		row, err := q.GetAgentRuntimePromotionProposalForUpdate(ctx, proposalUUID)
		if errors.Is(err, sql.ErrNoRows) {
			return application.ErrAgentRuntimePromotionControlDenied
		}
		if err != nil {
			return fmt.Errorf("lock Runtime promotion proposal: %w", err)
		}
		proposal := mapRuntimePromotionProposalV1(row)
		operator, err := getRuntimePromotionOperatorGrantV1(ctx, q, proposal.TenantID, reviewerUUID)
		if err != nil {
			return err
		}
		if operator == nil || !operator.Active(proposal.TenantID, decidedAt) || !operator.CanReview || proposal.ProposerUUID == reviewerUUID {
			return application.ErrAgentRuntimePromotionControlDenied
		}
		if proposal.Status != application.AgentRuntimePromotionProposalProposed {
			review, lookupErr := q.GetAgentRuntimePromotionReview(ctx, proposalUUID)
			if lookupErr == nil && review.ReviewerUuid == reviewerUUID && string(review.Decision) == string(decision) {
				return nil
			}
			return application.ErrAgentRuntimePromotionControlConflict
		}
		if !proposal.ExpiresAt.After(decidedAt) {
			return application.ErrAgentRuntimePromotionControlConflict
		}
		rows, err := q.InsertAgentRuntimePromotionReview(ctx, generated.InsertAgentRuntimePromotionReviewParams{ProposalUuid: proposalUUID, ReviewerUuid: reviewerUUID, Decision: generated.AgentRuntimePromotionReviewsDecision(decision), DecidedAt: decidedAt})
		if err != nil {
			return fmt.Errorf("record Runtime promotion review: %w", err)
		}
		if rows == 0 {
			return application.ErrAgentRuntimePromotionControlConflict
		}
		if decision == application.AgentRuntimePromotionReviewRejected {
			rows, err = q.RejectAgentRuntimePromotionProposal(ctx, generated.RejectAgentRuntimePromotionProposalParams{DecidedAt: sql.NullTime{Time: decidedAt, Valid: true}, ProposalUuid: proposalUUID, ExpiresAt: decidedAt})
			if err != nil || rows != 1 {
				return fmt.Errorf("reject Runtime promotion proposal: rows=%d: %w", rows, err)
			}
			return nil
		}
		grant := proposal.Grant(reviewerUUID)
		if err := grant.Validate(); err != nil {
			return err
		}
		rows, err = q.InsertAgentRuntimePromotionGrant(ctx, generated.InsertAgentRuntimePromotionGrantParams{GrantUuid: grant.GrantUUID, TenantID: grant.TenantID,
			RuntimeID: grant.RuntimeID, CandidateVersion: grant.CandidateVersion, DefinitionUuid: grant.DefinitionUUID, DefinitionVersion: grant.DefinitionVersion,
			PolicyVersion: grant.PolicyVersion, EvidenceSha256: grant.EvidenceSHA256, EvalSuiteSha256: grant.EvalSuiteSHA256,
			GrantedByUuid: grant.GrantedByUUID, ReviewedByUuid: grant.ReviewedByUUID, ValidFrom: grant.ValidFrom, ExpiresAt: grant.ExpiresAt})
		if err != nil || rows != 1 {
			return fmt.Errorf("create reviewed Runtime promotion grant: rows=%d: %w", rows, err)
		}
		rows, err = q.ApproveAgentRuntimePromotionProposal(ctx, generated.ApproveAgentRuntimePromotionProposalParams{GrantUuid: sql.NullString{String: grant.GrantUUID, Valid: true}, DecidedAt: sql.NullTime{Time: decidedAt, Valid: true}, ProposalUuid: proposalUUID, ExpiresAt: decidedAt})
		if err != nil || rows != 1 {
			return fmt.Errorf("approve Runtime promotion proposal: rows=%d: %w", rows, err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.GetRuntimePromotionProposal(ctx, proposalUUID)
}

func (r *AgentRuntimePromotionControlRepository) RevokeRuntimePromotionGrantAudited(ctx context.Context, revocation application.AgentRuntimePromotionRevocationV1) (*application.AgentRuntimePromotionGrantV1, error) {
	revocation.GrantUUID, revocation.RevokedByUUID = strings.TrimSpace(revocation.GrantUUID), strings.TrimSpace(revocation.RevokedByUUID)
	revocation.TicketRef, revocation.Reason = strings.TrimSpace(revocation.TicketRef), strings.TrimSpace(revocation.Reason)
	if revocation.GrantUUID == "" || revocation.RevokedByUUID == "" || revocation.TicketRef == "" || revocation.Reason == "" || revocation.RevokedAt.IsZero() {
		return nil, application.ErrAgentRuntimePromotionControlConflict
	}
	var result *application.AgentRuntimePromotionGrantV1
	err := r.store.WithinTx(ctx, nil, func(q *generated.Queries) error {
		row, err := q.GetAgentRuntimePromotionGrantForUpdate(ctx, revocation.GrantUUID)
		if errors.Is(err, sql.ErrNoRows) {
			return application.ErrAgentRuntimePromotionControlDenied
		}
		if err != nil {
			return fmt.Errorf("lock Runtime promotion grant: %w", err)
		}
		grant, err := mapAgentRuntimePromotionGrant(row)
		if err != nil {
			return err
		}
		operator, err := getRuntimePromotionOperatorGrantV1(ctx, q, grant.TenantID, revocation.RevokedByUUID)
		if err != nil {
			return err
		}
		if operator == nil || !operator.Active(grant.TenantID, revocation.RevokedAt) || !operator.CanRevoke {
			return application.ErrAgentRuntimePromotionControlDenied
		}
		revocation.TenantID = grant.TenantID
		if grant.RevokedAt != nil {
			existing, lookupErr := q.GetAgentRuntimePromotionRevocation(ctx, grant.GrantUUID)
			if lookupErr == nil && existing.RevokedByUuid == revocation.RevokedByUUID && existing.TicketRef == revocation.TicketRef && existing.Reason == revocation.Reason {
				result = grant
				return nil
			}
			return application.ErrAgentRuntimePromotionControlConflict
		}
		rows, err := q.InsertAgentRuntimePromotionRevocation(ctx, generated.InsertAgentRuntimePromotionRevocationParams{GrantUuid: grant.GrantUUID, TenantID: grant.TenantID,
			RevokedByUuid: revocation.RevokedByUUID, TicketRef: revocation.TicketRef, Reason: revocation.Reason, RevokedAt: revocation.RevokedAt})
		if err != nil || rows != 1 {
			return fmt.Errorf("record Runtime promotion revocation: rows=%d: %w", rows, err)
		}
		rows, err = q.RevokeAgentRuntimePromotionGrant(ctx, generated.RevokeAgentRuntimePromotionGrantParams{RevokedAt: sql.NullTime{Time: revocation.RevokedAt, Valid: true}, GrantUuid: grant.GrantUUID})
		if err != nil || rows != 1 {
			return fmt.Errorf("revoke Runtime promotion grant: rows=%d: %w", rows, err)
		}
		grant.RevokedAt = &revocation.RevokedAt
		result = grant
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
