package application

import (
	"context"
	"errors"
	"strings"
	"time"
)

const (
	AgentMemoryCandidateStatusPending          = "pending"
	AgentMemoryCandidateStatusAccepted         = "accepted"
	AgentMemoryCandidateStatusRejected         = "rejected"
	AgentMemoryCandidateTypeMessage            = "message"
	AgentMemoryCandidateTypeReflection         = "reflection"
	AgentMemoryCandidateReviewDecisionAccepted = "accepted"
	AgentMemoryCandidateReviewDecisionRejected = "rejected"
)

var (
	ErrAgentMemoryCandidateInvalid  = errors.New("Agent Memory candidate is invalid")
	ErrAgentMemoryCandidateConflict = errors.New("Agent Memory candidate promotion conflicts")
)

type AgentMemoryCandidateV1 struct {
	CandidateUUID      string
	TenantID           string
	PrincipalUUID      string
	AgentUUID          string
	ResourceType       string
	ResourceID         string
	CandidateType      string
	SourceID           string
	EvidenceIDs        []string
	Summary            string
	PolicyVersion      string
	CandidateSHA256    string
	Status             string
	ObservedAt         time.Time
	PromotedMemoryUUID string
	PromotedAt         *time.Time
}

type AgentMemoryCandidateReviewV1 struct {
	ReviewUUID      string
	CandidateUUID   string
	CandidateSHA256 string
	ReviewerUUID    string
	Decision        string
	Reason          string
	ReviewSHA256    string
	ReviewedAt      time.Time
}

type AgentMemoryCandidatePromotionRequestV1 struct {
	TenantID         string
	PrincipalUUID    string
	CandidateUUID    string
	CandidateSHA256  string
	ReviewUUID       string
	TargetMemoryType AgentMemoryTypeV1
}

type AgentMemoryCandidatePromotionStoreV1 interface {
	GetCandidateForPromotion(ctx context.Context, tenantID, principalUUID, candidateUUID string) (*AgentMemoryCandidateV1, error)
	GetCandidateReview(ctx context.Context, candidateUUID, reviewUUID string) (*AgentMemoryCandidateReviewV1, error)
	PromoteCandidate(ctx context.Context, candidate AgentMemoryCandidateV1, review AgentMemoryCandidateReviewV1, memory AgentMemoryV1) (*AgentMemoryV1, error)
}

type AgentMemoryCandidateCatalogItemV1 struct {
	Candidate  AgentMemoryCandidateV1
	ReviewUUID string
	ReviewedAt *time.Time
}

type AgentMemoryCandidateCatalogStoreV1 interface {
	ListOwnedCandidates(ctx context.Context, tenantID, principalUUID, afterCandidateUUID string, limit int) ([]AgentMemoryCandidateCatalogItemV1, error)
}

type AgentMemoryCandidateCatalogRequestV1 struct {
	TenantID           string
	PrincipalUUID      string
	AfterCandidateUUID string
	Limit              int
}

type AgentMemoryCandidateCatalogPageV1 struct {
	Items      []AgentMemoryCandidateCatalogItemV1
	NextCursor string
}

type AgentMemoryCandidateCatalogServiceV1 interface {
	ListOwnedCandidates(ctx context.Context, request AgentMemoryCandidateCatalogRequestV1) (*AgentMemoryCandidateCatalogPageV1, error)
}

type AgentMemoryCandidatePromotionServiceV1 interface {
	Promote(ctx context.Context, request AgentMemoryCandidatePromotionRequestV1) (*AgentMemoryV1, error)
}

func (candidate AgentMemoryCandidateV1) Validate() error {
	if anyBlank(candidate.CandidateUUID, candidate.TenantID, candidate.PrincipalUUID, candidate.AgentUUID, candidate.ResourceType, candidate.ResourceID, candidate.SourceID, candidate.Summary, candidate.PolicyVersion, candidate.CandidateSHA256, candidate.Status) ||
		len(candidate.CandidateUUID) > 72 || len(candidate.TenantID) > 64 || len(candidate.PrincipalUUID) > 64 || len(candidate.AgentUUID) > 24 || len(candidate.ResourceType) > 64 || len(candidate.ResourceID) > 128 || len(candidate.Summary) > 4096 || len(candidate.PolicyVersion) > 64 || len(candidate.CandidateSHA256) != 64 || !isHex(candidate.CandidateSHA256) || candidate.ObservedAt.IsZero() ||
		(candidate.CandidateType != AgentMemoryCandidateTypeMessage && candidate.CandidateType != AgentMemoryCandidateTypeReflection) ||
		(candidate.Status != AgentMemoryCandidateStatusPending && candidate.Status != AgentMemoryCandidateStatusAccepted && candidate.Status != AgentMemoryCandidateStatusRejected) || len(candidate.EvidenceIDs) == 0 || len(candidate.EvidenceIDs) > 128 {
		return ErrAgentMemoryCandidateInvalid
	}
	seen := make(map[string]struct{}, len(candidate.EvidenceIDs))
	for _, evidence := range candidate.EvidenceIDs {
		if strings.TrimSpace(evidence) == "" || len(evidence) > 128 {
			return ErrAgentMemoryCandidateInvalid
		}
		if _, exists := seen[evidence]; exists {
			return ErrAgentMemoryCandidateInvalid
		}
		seen[evidence] = struct{}{}
	}
	return nil
}

func (review AgentMemoryCandidateReviewV1) Validate() error {
	if anyBlank(review.ReviewUUID, review.CandidateUUID, review.CandidateSHA256, review.ReviewerUUID, review.Decision, review.Reason, review.ReviewSHA256) || len(review.ReviewUUID) > 72 || len(review.CandidateUUID) > 72 || len(review.ReviewerUUID) > 64 || len(review.Reason) > 1000 || len(review.CandidateSHA256) != 64 || !isHex(review.CandidateSHA256) || len(review.ReviewSHA256) != 64 || !isHex(review.ReviewSHA256) || review.ReviewedAt.IsZero() || (review.Decision != AgentMemoryCandidateReviewDecisionAccepted && review.Decision != AgentMemoryCandidateReviewDecisionRejected) {
		return ErrAgentMemoryCandidateInvalid
	}
	return nil
}

func isHex(value string) bool {
	for _, char := range value {
		if !(char >= '0' && char <= '9') && !(char >= 'a' && char <= 'f') {
			return false
		}
	}
	return true
}
