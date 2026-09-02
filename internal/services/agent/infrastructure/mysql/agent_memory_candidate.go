package agentmysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/application"
	mysqlData "github.com/JekYUlll/Dipole/internal/platform/mysql"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
)

var _ application.AgentMemoryCandidatePromotionStoreV1 = (*AgentMemoryRepository)(nil)

func (r *AgentMemoryRepository) GetCandidateForPromotion(ctx context.Context, tenantID, principalUUID, candidateUUID string) (*application.AgentMemoryCandidateV1, error) {
	row, err := r.queries.GetAgentMemoryCandidateForPromotion(ctx, generated.GetAgentMemoryCandidateForPromotionParams{TenantID: tenantID, PrincipalUuid: principalUUID, CandidateUuid: candidateUUID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get Agent Memory candidate: %w", err)
	}
	candidate, err := mapAgentMemoryCandidate(row)
	if err != nil {
		return nil, err
	}
	return &candidate, nil
}

func (r *AgentMemoryRepository) GetCandidateReview(ctx context.Context, candidateUUID, reviewUUID string) (*application.AgentMemoryCandidateReviewV1, error) {
	row, err := r.queries.GetAgentMemoryCandidateReviewForPromotion(ctx, generated.GetAgentMemoryCandidateReviewForPromotionParams{CandidateUuid: candidateUUID, ReviewUuid: reviewUUID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get Agent Memory candidate review: %w", err)
	}
	review := mapAgentMemoryCandidateReview(row)
	return &review, nil
}

func (r *AgentMemoryRepository) PromoteCandidate(ctx context.Context, candidate application.AgentMemoryCandidateV1, review application.AgentMemoryCandidateReviewV1, memory application.AgentMemoryV1) (*application.AgentMemoryV1, error) {
	if r.store == nil || candidate.Validate() != nil || review.Validate() != nil || memory.Validate() != nil {
		return nil, application.ErrAgentMemoryCandidateInvalid
	}
	var stored *application.AgentMemoryV1
	err := r.store.WithinTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted}, func(queries *generated.Queries) error {
		row, err := queries.GetAgentMemoryCandidateForPromotion(ctx, generated.GetAgentMemoryCandidateForPromotionParams{TenantID: candidate.TenantID, PrincipalUuid: candidate.PrincipalUUID, CandidateUuid: candidate.CandidateUUID})
		if errors.Is(err, sql.ErrNoRows) {
			return application.ErrAgentMemoryCandidateConflict
		}
		if err != nil {
			return fmt.Errorf("lock Agent Memory candidate: %w", err)
		}
		locked, err := mapAgentMemoryCandidate(row)
		if err != nil || !sameAgentMemoryCandidateV1(locked, candidate) || locked.Status != application.AgentMemoryCandidateStatusAccepted {
			return application.ErrAgentMemoryCandidateConflict
		}
		reviewRow, err := queries.GetAgentMemoryCandidateReviewForPromotion(ctx, generated.GetAgentMemoryCandidateReviewForPromotionParams{CandidateUuid: candidate.CandidateUUID, ReviewUuid: review.ReviewUUID})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return application.ErrAgentMemoryCandidateConflict
			}
			return fmt.Errorf("get Agent Memory candidate review in transaction: %w", err)
		}
		lockedReview := mapAgentMemoryCandidateReview(reviewRow)
		if !sameAgentMemoryCandidateReviewV1(lockedReview, review) || lockedReview.Decision != application.AgentMemoryCandidateReviewDecisionAccepted {
			return application.ErrAgentMemoryCandidateConflict
		}
		if locked.PromotedMemoryUUID != "" {
			if locked.PromotedMemoryUUID != memory.MemoryUUID {
				return application.ErrAgentMemoryCandidateConflict
			}
			row, getErr := queries.GetOwnedAgentMemory(ctx, generated.GetOwnedAgentMemoryParams{TenantID: candidate.TenantID, PrincipalUuid: candidate.PrincipalUUID, MemoryUuid: locked.PromotedMemoryUUID})
			if getErr != nil {
				return fmt.Errorf("get promoted Agent Memory: %w", getErr)
			}
			value := mapAgentMemory(row)
			if !samePromotedAgentMemoryV1(value, memory) {
				return application.ErrAgentMemoryCandidateConflict
			}
			stored = &value
			return nil
		}
		if err := insertAgentMemoryV1(ctx, queries, memory); err != nil && !isExactAgentMemoryDuplicate(ctx, queries, memory, err) {
			return fmt.Errorf("promote Agent Memory candidate: %w", err)
		}
		rows, err := queries.PromoteAgentMemoryCandidate(ctx, generated.PromoteAgentMemoryCandidateParams{
			PromotedMemoryUuid: sql.NullString{String: memory.MemoryUUID, Valid: true}, PromotedAt: sql.NullTime{Time: memory.ValidFrom, Valid: true},
			TenantID: candidate.TenantID, PrincipalUuid: candidate.PrincipalUUID, CandidateUuid: candidate.CandidateUUID, CandidateSha256: candidate.CandidateSHA256,
		})
		if err != nil {
			return fmt.Errorf("record Agent Memory candidate promotion: %w", err)
		}
		if rows != 1 {
			return application.ErrAgentMemoryCandidateConflict
		}
		copy := memory
		stored = &copy
		return nil
	})
	if err != nil {
		return nil, err
	}
	return stored, nil
}

func mapAgentMemoryCandidate(row generated.AgentMemoryCandidate) (application.AgentMemoryCandidateV1, error) {
	var evidence []string
	if err := json.Unmarshal(row.EvidenceIdsJson, &evidence); err != nil {
		return application.AgentMemoryCandidateV1{}, fmt.Errorf("decode Agent Memory candidate evidence: %w", err)
	}
	return application.AgentMemoryCandidateV1{
		CandidateUUID: row.CandidateUuid, TenantID: row.TenantID, PrincipalUUID: row.PrincipalUuid, AgentUUID: row.AgentUuid,
		ResourceType: row.ResourceType, ResourceID: row.ResourceID, CandidateType: row.CandidateType, SourceID: row.SourceID,
		EvidenceIDs: evidence, Summary: row.Summary, PolicyVersion: row.PolicyVersion, CandidateSHA256: row.CandidateSha256, Status: row.Status,
		ObservedAt: row.ObservedAt, PromotedMemoryUUID: row.PromotedMemoryUuid.String, PromotedAt: timePointer(row.PromotedAt),
	}, nil
}

func mapAgentMemoryCandidateReview(row generated.AgentMemoryCandidateReview) application.AgentMemoryCandidateReviewV1 {
	return application.AgentMemoryCandidateReviewV1{ReviewUUID: row.ReviewUuid, CandidateUUID: row.CandidateUuid, CandidateSHA256: row.CandidateSha256, ReviewerUUID: row.ReviewerUuid, Decision: row.Decision, Reason: row.Reason, ReviewSHA256: row.ReviewSha256, ReviewedAt: row.ReviewedAt}
}

func sameAgentMemoryCandidateV1(left, right application.AgentMemoryCandidateV1) bool {
	if left.CandidateUUID != right.CandidateUUID || left.TenantID != right.TenantID || left.PrincipalUUID != right.PrincipalUUID || left.AgentUUID != right.AgentUUID || left.ResourceType != right.ResourceType || left.ResourceID != right.ResourceID || left.CandidateType != right.CandidateType || left.SourceID != right.SourceID || left.Summary != right.Summary || left.PolicyVersion != right.PolicyVersion || left.CandidateSHA256 != right.CandidateSHA256 || left.Status != right.Status || !left.ObservedAt.Equal(right.ObservedAt) || left.PromotedMemoryUUID != right.PromotedMemoryUUID {
		return false
	}
	if len(left.EvidenceIDs) != len(right.EvidenceIDs) {
		return false
	}
	for i := range left.EvidenceIDs {
		if left.EvidenceIDs[i] != right.EvidenceIDs[i] {
			return false
		}
	}
	return true
}

func sameAgentMemoryCandidateReviewV1(left, right application.AgentMemoryCandidateReviewV1) bool {
	return left.ReviewUUID == right.ReviewUUID && left.CandidateUUID == right.CandidateUUID && left.CandidateSHA256 == right.CandidateSHA256 && left.ReviewerUUID == right.ReviewerUUID && left.Decision == right.Decision && left.Reason == right.Reason && left.ReviewSHA256 == right.ReviewSHA256 && left.ReviewedAt.Equal(right.ReviewedAt)
}

func samePromotedAgentMemoryV1(left, right application.AgentMemoryV1) bool {
	// ValidFrom is assigned by the first durable commit. A retry can have a later
	// wall-clock value while still representing the same candidate promotion.
	return left.MemoryUUID == right.MemoryUUID && left.TenantID == right.TenantID && left.PrincipalUUID == right.PrincipalUUID && left.AgentUUID == right.AgentUUID && left.MemoryType == right.MemoryType && left.Status == right.Status && left.ResourceType == right.ResourceType && left.ResourceID == right.ResourceID && left.Content == right.Content && left.CompactContent == right.CompactContent && left.Priority == right.Priority && left.Provenance == right.Provenance
}

func isExactAgentMemoryDuplicate(ctx context.Context, queries *generated.Queries, memory application.AgentMemoryV1, err error) bool {
	if !mysqlData.IsDuplicateKey(err) {
		return false
	}
	row, getErr := queries.GetOwnedAgentMemory(ctx, generated.GetOwnedAgentMemoryParams{TenantID: memory.TenantID, PrincipalUuid: memory.PrincipalUUID, MemoryUuid: memory.MemoryUUID})
	return getErr == nil && samePromotedAgentMemoryV1(mapAgentMemory(row), memory)
}
