package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

type AgentMemoryCandidateReviewRequestV1 struct {
	TenantID        string
	PrincipalUUID   string
	CandidateUUID   string
	CandidateSHA256 string
	Decision        string
	Reason          string
}

type AgentMemoryCandidatePromotionStoreV1 interface {
	GetCandidateForPromotion(ctx context.Context, tenantID, principalUUID, candidateUUID string) (*AgentMemoryCandidateV1, error)
	GetCandidateReview(ctx context.Context, candidateUUID, reviewUUID string) (*AgentMemoryCandidateReviewV1, error)
	PromoteCandidate(ctx context.Context, candidate AgentMemoryCandidateV1, review AgentMemoryCandidateReviewV1, memory AgentMemoryV1) (*AgentMemoryV1, error)
	ReviewCandidate(ctx context.Context, candidate AgentMemoryCandidateV1, review AgentMemoryCandidateReviewV1) (*AgentMemoryCandidateCatalogItemV1, error)
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
	Review(ctx context.Context, request AgentMemoryCandidateReviewRequestV1) (*AgentMemoryCandidateCatalogItemV1, error)
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

func NormalizeAgentMemoryCandidateReviewReason(reason string) (string, error) {
	normalized := strings.TrimSpace(reason)
	if normalized == "" || len([]rune(normalized)) > 1000 || strings.IndexFunc(normalized, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
		return "", ErrAgentMemoryCandidateInvalid
	}
	lower := strings.ToLower(normalized)
	for _, marker := range []string{"password", "passwd", "token", "secret", "authorization", "bearer", "api_key", "api-key", "api key"} {
		if index := strings.Index(lower, marker); index >= 0 {
			rest := strings.TrimLeft(normalized[index+len(marker):], " \t")
			if strings.HasPrefix(rest, "=") || strings.HasPrefix(rest, ":") {
				return "", ErrAgentMemoryCandidateInvalid
			}
		}
	}
	return normalized, nil
}

func BuildAgentMemoryCandidateReviewV1(candidateUUID, candidateSHA256, reviewerUUID, decision, reason string, reviewedAt time.Time) (AgentMemoryCandidateReviewV1, error) {
	normalized, err := NormalizeAgentMemoryCandidateReviewReason(reason)
	if err != nil {
		return AgentMemoryCandidateReviewV1{}, err
	}
	candidateUUID, candidateSHA256, reviewerUUID, decision = strings.TrimSpace(candidateUUID), strings.TrimSpace(candidateSHA256), strings.TrimSpace(reviewerUUID), strings.TrimSpace(decision)
	if anyBlank(candidateUUID, candidateSHA256, reviewerUUID) || len(candidateSHA256) != 64 || !isHex(candidateSHA256) || (decision != AgentMemoryCandidateReviewDecisionAccepted && decision != AgentMemoryCandidateReviewDecisionRejected) || reviewedAt.IsZero() {
		return AgentMemoryCandidateReviewV1{}, ErrAgentMemoryCandidateInvalid
	}
	reviewedAt = reviewedAt.UTC().Truncate(time.Millisecond)
	reviewedAtText := reviewedAt.Format("2006-01-02T15:04:05.000Z")
	canonical, err := marshalAgentMemoryCandidateReviewCanonicalV1(candidateUUID, candidateSHA256, reviewerUUID, decision, normalized, reviewedAtText)
	if err != nil {
		return AgentMemoryCandidateReviewV1{}, err
	}
	digest := sha256.Sum256(canonical)
	hash := hex.EncodeToString(digest[:])
	review := AgentMemoryCandidateReviewV1{
		ReviewUUID: "REVIEW-" + hash, CandidateUUID: candidateUUID, CandidateSHA256: candidateSHA256,
		ReviewerUUID: reviewerUUID, Decision: decision, Reason: normalized, ReviewSHA256: hash, ReviewedAt: reviewedAt,
	}
	if err := review.Validate(); err != nil {
		return AgentMemoryCandidateReviewV1{}, err
	}
	return review, nil
}

func marshalAgentMemoryCandidateReviewCanonicalV1(candidateUUID, candidateSHA256, reviewerUUID, decision, reason, reviewedAt string) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(struct {
		SchemaVersion   string `json:"schemaVersion"`
		CandidateID     string `json:"candidateId"`
		CandidateSHA256 string `json:"candidateSha256"`
		ReviewerID      string `json:"reviewerId"`
		Decision        string `json:"decision"`
		Reason          string `json:"reason"`
		ReviewedAt      string `json:"reviewedAt"`
	}{
		SchemaVersion: "dipole.agent.memory-candidate-review.v1", CandidateID: candidateUUID, CandidateSHA256: candidateSHA256,
		ReviewerID: reviewerUUID, Decision: decision, Reason: reason, ReviewedAt: reviewedAt,
	}); err != nil {
		return nil, err
	}
	canonical := bytes.TrimRight(buffer.Bytes(), "\n")
	return canonical, nil
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
