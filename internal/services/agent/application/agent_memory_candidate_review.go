package agentapplication

import (
	"context"
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
)

func (s *PersistentAgentMemoryCandidatePromotionServiceV1) Review(ctx context.Context, request application.AgentMemoryCandidateReviewRequestV1) (*application.AgentMemoryCandidateCatalogItemV1, error) {
	request.TenantID, request.PrincipalUUID, request.CandidateUUID = strings.TrimSpace(request.TenantID), strings.TrimSpace(request.PrincipalUUID), strings.TrimSpace(request.CandidateUUID)
	request.CandidateSHA256, request.Decision, request.Reason = strings.TrimSpace(request.CandidateSHA256), strings.TrimSpace(request.Decision), strings.TrimSpace(request.Reason)
	if request.TenantID == "" || request.PrincipalUUID == "" || request.CandidateUUID == "" || request.CandidateSHA256 == "" {
		return nil, application.ErrAgentMemoryCandidateInvalid
	}
	candidate, err := s.store.GetCandidateForPromotion(ctx, request.TenantID, request.PrincipalUUID, request.CandidateUUID)
	if err != nil {
		return nil, err
	}
	if candidate == nil || candidate.Validate() != nil || candidate.TenantID != request.TenantID || candidate.PrincipalUUID != request.PrincipalUUID || candidate.CandidateUUID != request.CandidateUUID || candidate.CandidateSHA256 != request.CandidateSHA256 {
		return nil, application.ErrAgentMemoryCandidateConflict
	}
	now := s.now().UTC()
	if now.IsZero() || candidate.ObservedAt.After(now) || now.Sub(candidate.ObservedAt) > agentMemoryCandidateMaxAgeV1 {
		return nil, application.ErrAgentMemoryCandidateConflict
	}
	review, err := application.BuildAgentMemoryCandidateReviewV1(candidate.CandidateUUID, candidate.CandidateSHA256, request.PrincipalUUID, request.Decision, request.Reason, now)
	if err != nil {
		return nil, err
	}
	return s.store.ReviewCandidate(ctx, *candidate, review)
}
