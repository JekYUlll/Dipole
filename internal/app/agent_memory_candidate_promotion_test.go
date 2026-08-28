package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

func TestAgentMemoryCandidatePromotionRequiresAcceptedExactReview(t *testing.T) {
	now := time.Date(2026, 8, 29, 1, 0, 0, 0, time.UTC)
	candidate := promotionCandidate(now)
	review := promotionReview(candidate)
	store := &candidatePromotionStore{candidate: &candidate, review: &review}
	service, err := NewPersistentAgentMemoryCandidatePromotionServiceV1(store, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	memory, err := service.Promote(context.Background(), application.AgentMemoryCandidatePromotionRequestV1{
		TenantID: "dipole", PrincipalUUID: "U100", CandidateUUID: candidate.CandidateUUID,
		CandidateSHA256: candidate.CandidateSHA256, ReviewUUID: review.ReviewUUID,
	})
	if err != nil || memory == nil {
		t.Fatalf("promote accepted candidate memory=%+v err=%v", memory, err)
	}
	if memory.MemoryType != application.AgentMemoryTypeObservational || memory.Content != candidate.Summary || memory.Provenance.SourceID != candidate.CandidateUUID || store.promotions != 1 {
		t.Fatalf("unexpected promoted memory=%+v promotions=%d", memory, store.promotions)
	}
}

func TestAgentMemoryCandidatePromotionFailsClosedOnPendingDriftAndStaleEvidence(t *testing.T) {
	now := time.Date(2026, 8, 29, 1, 0, 0, 0, time.UTC)
	candidate := promotionCandidate(now)
	review := promotionReview(candidate)
	cases := []struct {
		name string
		edit func(*application.AgentMemoryCandidateV1, *application.AgentMemoryCandidateReviewV1)
	}{
		{name: "pending", edit: func(c *application.AgentMemoryCandidateV1, _ *application.AgentMemoryCandidateReviewV1) {
			c.Status = application.AgentMemoryCandidateStatusPending
		}},
		{name: "hash drift", edit: func(c *application.AgentMemoryCandidateV1, _ *application.AgentMemoryCandidateReviewV1) {
			c.CandidateSHA256 = "b" + c.CandidateSHA256[1:]
		}},
		{name: "review drift", edit: func(_ *application.AgentMemoryCandidateV1, r *application.AgentMemoryCandidateReviewV1) {
			r.Decision = application.AgentMemoryCandidateReviewDecisionRejected
		}},
		{name: "stale", edit: func(c *application.AgentMemoryCandidateV1, _ *application.AgentMemoryCandidateReviewV1) {
			c.ObservedAt = now.Add(-31 * 24 * time.Hour)
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			currentCandidate, currentReview := candidate, review
			test.edit(&currentCandidate, &currentReview)
			store := &candidatePromotionStore{candidate: &currentCandidate, review: &currentReview}
			candidateService, _ := NewPersistentAgentMemoryCandidatePromotionServiceV1(store, func() time.Time { return now })
			_, err := candidateService.Promote(context.Background(), application.AgentMemoryCandidatePromotionRequestV1{
				TenantID: "dipole", PrincipalUUID: "U100", CandidateUUID: candidate.CandidateUUID,
				CandidateSHA256: candidate.CandidateSHA256, ReviewUUID: review.ReviewUUID,
			})
			if !errors.Is(err, application.ErrAgentMemoryCandidateConflict) || store.promotions != 0 {
				t.Fatalf("err=%v promotions=%d", err, store.promotions)
			}
		})
	}
}

func TestAgentMemoryCandidatePromotionIsIdempotent(t *testing.T) {
	now := time.Date(2026, 8, 29, 1, 0, 0, 0, time.UTC)
	candidate := promotionCandidate(now)
	review := promotionReview(candidate)
	store := &candidatePromotionStore{candidate: &candidate, review: &review}
	service, _ := NewPersistentAgentMemoryCandidatePromotionServiceV1(store, func() time.Time { return now })
	request := application.AgentMemoryCandidatePromotionRequestV1{TenantID: "dipole", PrincipalUUID: "U100", CandidateUUID: candidate.CandidateUUID, CandidateSHA256: candidate.CandidateSHA256, ReviewUUID: review.ReviewUUID}
	first, firstErr := service.Promote(context.Background(), request)
	second, secondErr := service.Promote(context.Background(), request)
	if firstErr != nil || secondErr != nil || first == nil || second == nil || first.MemoryUUID != second.MemoryUUID || store.promotions != 1 {
		t.Fatalf("first=%+v/%v second=%+v/%v promotions=%d", first, firstErr, second, secondErr, store.promotions)
	}
}

type candidatePromotionStore struct {
	candidate  *application.AgentMemoryCandidateV1
	review     *application.AgentMemoryCandidateReviewV1
	memory     *application.AgentMemoryV1
	promotions int
}

func (s *candidatePromotionStore) GetCandidateForPromotion(context.Context, string, string, string) (*application.AgentMemoryCandidateV1, error) {
	if s.candidate == nil {
		return nil, nil
	}
	copy := *s.candidate
	return &copy, nil
}

func (s *candidatePromotionStore) GetCandidateReview(context.Context, string, string) (*application.AgentMemoryCandidateReviewV1, error) {
	if s.review == nil {
		return nil, nil
	}
	copy := *s.review
	return &copy, nil
}

func (s *candidatePromotionStore) PromoteCandidate(_ context.Context, _ application.AgentMemoryCandidateV1, _ application.AgentMemoryCandidateReviewV1, memory application.AgentMemoryV1) (*application.AgentMemoryV1, error) {
	if s.memory != nil {
		copy := *s.memory
		return &copy, nil
	}
	s.promotions++
	s.memory = &memory
	copy := memory
	return &copy, nil
}

func promotionCandidate(now time.Time) application.AgentMemoryCandidateV1 {
	return application.AgentMemoryCandidateV1{
		CandidateUUID: "OBS-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		TenantID:      "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", ResourceType: "conversation", ResourceID: "group:G1",
		CandidateType: application.AgentMemoryCandidateTypeMessage, SourceID: "M-1", EvidenceIDs: []string{"M-1"}, Summary: "API v2 Friday", PolicyVersion: "observation-v1",
		CandidateSHA256: strings.Repeat("a", 64), Status: application.AgentMemoryCandidateStatusAccepted, ObservedAt: now.Add(-time.Hour),
	}
}

func promotionReview(candidate application.AgentMemoryCandidateV1) application.AgentMemoryCandidateReviewV1 {
	return application.AgentMemoryCandidateReviewV1{ReviewUUID: "REVIEW-1", CandidateUUID: candidate.CandidateUUID, CandidateSHA256: candidate.CandidateSHA256, ReviewerUUID: "U100", Decision: application.AgentMemoryCandidateReviewDecisionAccepted, Reason: "owner confirmed", ReviewSHA256: strings.Repeat("c", 64), ReviewedAt: candidate.ObservedAt.Add(time.Minute)}
}
