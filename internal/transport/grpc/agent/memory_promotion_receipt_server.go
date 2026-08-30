package agentgrpc

import (
	"context"
	"errors"

	agentv1 "github.com/JekYUlll/Dipole/api/gen/go/agent/v1"
	"github.com/JekYUlll/Dipole/internal/application"
)

// MemoryPromotionReceiptServer exposes only the reviewed receipt commit seam.
// Remaining Agent Capability methods stay unimplemented in the standalone Core.
type MemoryPromotionReceiptServer struct {
	agentv1.UnimplementedAgentCapabilityServiceServer
	commits application.AgentMemoryPromotionReceiptCommitServiceV1
}

func NewMemoryPromotionReceiptServer(commits application.AgentMemoryPromotionReceiptCommitServiceV1) (*MemoryPromotionReceiptServer, error) {
	if commits == nil {
		return nil, errors.New("Agent Memory promotion receipt commit service is required")
	}
	return &MemoryPromotionReceiptServer{commits: commits}, nil
}

func (s *MemoryPromotionReceiptServer) CommitMemoryPromotionReceipt(ctx context.Context, request *agentv1.CommitMemoryPromotionReceiptRequest) (*agentv1.CommitMemoryPromotionReceiptResponse, error) {
	if s == nil {
		return nil, errors.New("Agent Memory promotion receipt server is unavailable")
	}
	return commitMemoryPromotionReceiptV1(ctx, request, s.commits)
}
