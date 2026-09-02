package agentapplication

import (
	"context"
	"errors"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

type PersistentAgentMemoryPromotionReceiptCommitServiceV1 struct {
	resolver   application.AgentInvocationResolverV1
	promotions application.AgentMemoryCandidatePromotionServiceV1
	now        func() time.Time
}

var _ application.AgentMemoryPromotionReceiptCommitServiceV1 = (*PersistentAgentMemoryPromotionReceiptCommitServiceV1)(nil)

func NewPersistentAgentMemoryPromotionReceiptCommitServiceV1(resolver application.AgentInvocationResolverV1, promotions application.AgentMemoryCandidatePromotionServiceV1, now func() time.Time) (*PersistentAgentMemoryPromotionReceiptCommitServiceV1, error) {
	if resolver == nil || promotions == nil || now == nil {
		return nil, errors.New("Agent Memory promotion receipt commit dependencies are required")
	}
	return &PersistentAgentMemoryPromotionReceiptCommitServiceV1{resolver: resolver, promotions: promotions, now: now}, nil
}

func (s *PersistentAgentMemoryPromotionReceiptCommitServiceV1) CommitMemoryPromotionReceipt(ctx context.Context, request application.AgentMemoryPromotionReceiptCommitRequestV1) (*application.AgentMemoryV1, error) {
	now := s.now().UTC()
	if err := request.ValidateAt(now); err != nil {
		return nil, err
	}
	invocation, err := s.resolver.Resolve(ctx, request.TaskUUID, request.RunUUID)
	if err != nil {
		return nil, err
	}
	if err := request.VerifyInvocation(invocation); err != nil {
		return nil, err
	}
	return s.promotions.Promote(ctx, application.AgentMemoryCandidatePromotionRequestV1{
		TenantID: invocation.TenantID, PrincipalUUID: invocation.PrincipalUUID,
		CandidateUUID: request.CandidateUUID, CandidateSHA256: request.CandidateSHA256,
		ReviewUUID: request.ReviewUUID, TargetMemoryType: request.TargetMemoryType,
	})
}
