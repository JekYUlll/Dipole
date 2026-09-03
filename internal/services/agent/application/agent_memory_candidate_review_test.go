package agentapplication

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

func TestAgentMemoryCandidateReviewAcceptsPendingOwnerDecision(t *testing.T) {
	now := time.Date(2026, 8, 29, 1, 0, 0, 0, time.UTC)
	candidate := promotionCandidate(now)
	candidate.Status = application.AgentMemoryCandidateStatusPending
	store := &candidatePromotionStore{candidate: &candidate}
	service, err := NewPersistentAgentMemoryCandidatePromotionServiceV1(store, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	item, err := service.Review(context.Background(), application.AgentMemoryCandidateReviewRequestV1{
		TenantID: "dipole", PrincipalUUID: "U100", CandidateUUID: candidate.CandidateUUID,
		CandidateSHA256: candidate.CandidateSHA256, Decision: "accepted", Reason: "owner confirmed",
	})
	if err != nil || item == nil || item.Candidate.Status != application.AgentMemoryCandidateStatusAccepted || item.ReviewUUID == "" || item.ReviewUUID != store.review.ReviewUUID {
		t.Fatalf("review item=%+v err=%v", item, err)
	}
}

func TestAgentMemoryCandidateReviewRejectsSecretReasonAndHashDrift(t *testing.T) {
	now := time.Date(2026, 8, 29, 1, 0, 0, 0, time.UTC)
	candidate := promotionCandidate(now)
	candidate.Status = application.AgentMemoryCandidateStatusPending
	store := &candidatePromotionStore{candidate: &candidate}
	service, _ := NewPersistentAgentMemoryCandidatePromotionServiceV1(store, func() time.Time { return now })
	_, err := service.Review(context.Background(), application.AgentMemoryCandidateReviewRequestV1{
		TenantID: "dipole", PrincipalUUID: "U100", CandidateUUID: candidate.CandidateUUID,
		CandidateSHA256: candidate.CandidateSHA256, Decision: "accepted", Reason: "token=secret",
	})
	if !errors.Is(err, application.ErrAgentMemoryCandidateInvalid) || store.review != nil {
		t.Fatalf("secret reason err=%v review=%+v", err, store.review)
	}
	_, err = service.Review(context.Background(), application.AgentMemoryCandidateReviewRequestV1{
		TenantID: "dipole", PrincipalUUID: "U100", CandidateUUID: candidate.CandidateUUID,
		CandidateSHA256: "b" + candidate.CandidateSHA256[1:], Decision: "accepted", Reason: "owner confirmed",
	})
	if !errors.Is(err, application.ErrAgentMemoryCandidateConflict) {
		t.Fatalf("hash drift err=%v", err)
	}
}

func TestAgentMemoryCandidateReviewIsIdempotentForExactReplay(t *testing.T) {
	now := time.Date(2026, 8, 29, 1, 0, 0, 0, time.UTC)
	candidate := promotionCandidate(now)
	candidate.Status = application.AgentMemoryCandidateStatusPending
	store := &candidatePromotionStore{candidate: &candidate}
	service, _ := NewPersistentAgentMemoryCandidatePromotionServiceV1(store, func() time.Time { return now })
	request := application.AgentMemoryCandidateReviewRequestV1{
		TenantID: "dipole", PrincipalUUID: "U100", CandidateUUID: candidate.CandidateUUID,
		CandidateSHA256: candidate.CandidateSHA256, Decision: "rejected", Reason: "owner declined",
	}
	first, firstErr := service.Review(context.Background(), request)
	second, secondErr := service.Review(context.Background(), request)
	if firstErr != nil || secondErr != nil || first.ReviewUUID != second.ReviewUUID || first.Candidate.Status != application.AgentMemoryCandidateStatusRejected {
		t.Fatalf("first=%+v/%v second=%+v/%v", first, firstErr, second, secondErr)
	}
}
