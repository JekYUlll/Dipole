package app

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

const agentMemoryCandidateMaxAgeV1 = 30 * 24 * time.Hour

type PersistentAgentMemoryCandidatePromotionServiceV1 struct {
	store application.AgentMemoryCandidatePromotionStoreV1
	now   func() time.Time
}

func NewPersistentAgentMemoryCandidatePromotionServiceV1(store application.AgentMemoryCandidatePromotionStoreV1, now func() time.Time) (*PersistentAgentMemoryCandidatePromotionServiceV1, error) {
	if store == nil {
		return nil, errors.New("Agent Memory candidate promotion store is required")
	}
	if now == nil {
		now = time.Now
	}
	return &PersistentAgentMemoryCandidatePromotionServiceV1{store: store, now: now}, nil
}

func (s *PersistentAgentMemoryCandidatePromotionServiceV1) Promote(ctx context.Context, request application.AgentMemoryCandidatePromotionRequestV1) (*application.AgentMemoryV1, error) {
	request.TenantID, request.PrincipalUUID, request.CandidateUUID = strings.TrimSpace(request.TenantID), strings.TrimSpace(request.PrincipalUUID), strings.TrimSpace(request.CandidateUUID)
	request.CandidateSHA256, request.ReviewUUID = strings.TrimSpace(request.CandidateSHA256), strings.TrimSpace(request.ReviewUUID)
	if request.TenantID == "" || request.PrincipalUUID == "" || request.CandidateUUID == "" || request.CandidateSHA256 == "" || request.ReviewUUID == "" {
		return nil, application.ErrAgentMemoryCandidateInvalid
	}
	candidate, err := s.store.GetCandidateForPromotion(ctx, request.TenantID, request.PrincipalUUID, request.CandidateUUID)
	if err != nil {
		return nil, fmt.Errorf("get Agent Memory candidate: %w", err)
	}
	if candidate == nil || candidate.Validate() != nil || candidate.TenantID != request.TenantID || candidate.PrincipalUUID != request.PrincipalUUID || candidate.CandidateUUID != request.CandidateUUID || candidate.CandidateSHA256 != request.CandidateSHA256 || candidate.Status != application.AgentMemoryCandidateStatusAccepted {
		return nil, application.ErrAgentMemoryCandidateConflict
	}
	review, err := s.store.GetCandidateReview(ctx, candidate.CandidateUUID, request.ReviewUUID)
	if err != nil {
		return nil, fmt.Errorf("get Agent Memory candidate review: %w", err)
	}
	if review == nil || review.Validate() != nil || review.ReviewUUID != request.ReviewUUID || review.CandidateUUID != candidate.CandidateUUID || review.CandidateSHA256 != candidate.CandidateSHA256 || review.Decision != application.AgentMemoryCandidateReviewDecisionAccepted || review.ReviewerUUID != candidate.PrincipalUUID {
		return nil, application.ErrAgentMemoryCandidateConflict
	}
	now := s.now().UTC()
	if now.IsZero() || candidate.ObservedAt.After(now) || now.Sub(candidate.ObservedAt) > agentMemoryCandidateMaxAgeV1 {
		return nil, application.ErrAgentMemoryCandidateConflict
	}
	memory := application.AgentMemoryV1{
		MemoryUUID: stableAgentMemoryCandidateUUIDV1(*candidate), TenantID: candidate.TenantID, PrincipalUUID: candidate.PrincipalUUID, AgentUUID: candidate.AgentUUID,
		MemoryType: application.AgentMemoryTypeObservational, Status: application.AgentMemoryStatusActive,
		ResourceType: candidate.ResourceType, ResourceID: candidate.ResourceID, Content: candidate.Summary, CompactContent: candidate.Summary,
		Priority: 60, Provenance: application.AgentMemoryProvenanceV1{SourceType: "memory_candidate", SourceID: candidate.CandidateUUID, Sequence: review.ReviewUUID}, ValidFrom: now,
	}
	if memory.Validate() != nil {
		return nil, application.ErrAgentMemoryCandidateInvalid
	}
	stored, err := s.store.PromoteCandidate(ctx, *candidate, *review, memory)
	if err != nil {
		return nil, err
	}
	if stored == nil || stored.MemoryUUID != memory.MemoryUUID || stored.TenantID != memory.TenantID || stored.PrincipalUUID != memory.PrincipalUUID || stored.Content != memory.Content || stored.Validate() != nil {
		return nil, application.ErrAgentMemoryCandidateConflict
	}
	return stored, nil
}

func stableAgentMemoryCandidateUUIDV1(candidate application.AgentMemoryCandidateV1) string {
	canonical := fmt.Sprintf("%s|%s|%s|%s", candidate.TenantID, candidate.CandidateUUID, candidate.CandidateSHA256, candidate.PolicyVersion)
	digest := sha256.Sum256([]byte(canonical))
	return fmt.Sprintf("MEM-CAND-%x", digest[:16])
}
