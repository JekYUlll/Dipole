package agentgrpc

import (
	"context"
	"errors"

	agentv1 "github.com/JekYUlll/Dipole/api/gen/go/agent/v1"
	"github.com/JekYUlll/Dipole/internal/application"
)

// RestrictedServer exposes independently opt-in Agent seams without requiring
// the full interactive capability composition in standalone Core.
type RestrictedServer struct {
	agentv1.UnimplementedAgentCapabilityServiceServer
	commits               application.AgentMemoryPromotionReceiptCommitServiceV1
	oauthTransactions     application.AgentOAuthAuthorizationTransactionStoreV1
	oauthCallbackHandoffs application.AgentOAuthCallbackHandoffStoreV1
}

// MemoryPromotionReceiptServer remains an alias for callers that compose only
// the receipt seam. RestrictedServer also supports the separately gated OAuth
// transaction seam.
type MemoryPromotionReceiptServer = RestrictedServer

func NewMemoryPromotionReceiptServer(commits application.AgentMemoryPromotionReceiptCommitServiceV1) (*RestrictedServer, error) {
	if commits == nil {
		return nil, errors.New("Agent Memory promotion receipt commit service is required")
	}
	return &RestrictedServer{commits: commits}, nil
}

func NewOAuthAuthorizationTransactionServer(transactions application.AgentOAuthAuthorizationTransactionStoreV1) (*RestrictedServer, error) {
	if transactions == nil {
		return nil, errors.New("Agent OAuth authorization transaction store is required")
	}
	return &RestrictedServer{oauthTransactions: transactions}, nil
}

func (s *RestrictedServer) WithMemoryPromotionReceiptCommits(commits application.AgentMemoryPromotionReceiptCommitServiceV1) (*RestrictedServer, error) {
	if s == nil || commits == nil {
		return nil, errors.New("Agent Memory promotion receipt commit service is required")
	}
	s.commits = commits
	return s, nil
}

func (s *RestrictedServer) WithOAuthAuthorizationTransactions(transactions application.AgentOAuthAuthorizationTransactionStoreV1) (*RestrictedServer, error) {
	if s == nil || transactions == nil {
		return nil, errors.New("Agent OAuth authorization transaction store is required")
	}
	s.oauthTransactions = transactions
	return s, nil
}

func (s *RestrictedServer) WithOAuthCallbackHandoffs(handoffs application.AgentOAuthCallbackHandoffStoreV1) (*RestrictedServer, error) {
	if s == nil || handoffs == nil {
		return nil, errors.New("Agent OAuth callback handoff store is required")
	}
	s.oauthCallbackHandoffs = handoffs
	return s, nil
}

func (s *RestrictedServer) CommitMemoryPromotionReceipt(ctx context.Context, request *agentv1.CommitMemoryPromotionReceiptRequest) (*agentv1.CommitMemoryPromotionReceiptResponse, error) {
	if s == nil {
		return nil, errors.New("Agent Memory promotion receipt server is unavailable")
	}
	return commitMemoryPromotionReceiptV1(ctx, request, s.commits)
}

func (s *RestrictedServer) ConsumeOAuthAuthorizationTransaction(ctx context.Context, request *agentv1.ConsumeOAuthAuthorizationTransactionRequest) (*agentv1.ConsumeOAuthAuthorizationTransactionResponse, error) {
	if s == nil {
		return nil, errors.New("Agent OAuth authorization transaction server is unavailable")
	}
	return consumeOAuthAuthorizationTransactionV1(ctx, request, s.oauthTransactions)
}

func (s *RestrictedServer) ClaimOAuthCallbackHandoff(ctx context.Context, request *agentv1.ClaimOAuthCallbackHandoffRequest) (*agentv1.ClaimOAuthCallbackHandoffResponse, error) {
	if s == nil {
		return nil, errors.New("Agent OAuth callback handoff server is unavailable")
	}
	return claimOAuthCallbackHandoffV1(ctx, request, s.oauthCallbackHandoffs)
}

func (s *RestrictedServer) CompleteOAuthCallbackHandoff(ctx context.Context, request *agentv1.CompleteOAuthCallbackHandoffRequest) (*agentv1.CompleteOAuthCallbackHandoffResponse, error) {
	if s == nil {
		return nil, errors.New("Agent OAuth callback handoff server is unavailable")
	}
	return completeOAuthCallbackHandoffV1(ctx, request, s.oauthCallbackHandoffs)
}

func (s *RestrictedServer) ReleaseOAuthCallbackHandoff(ctx context.Context, request *agentv1.ReleaseOAuthCallbackHandoffRequest) (*agentv1.ReleaseOAuthCallbackHandoffResponse, error) {
	if s == nil {
		return nil, errors.New("Agent OAuth callback handoff server is unavailable")
	}
	return releaseOAuthCallbackHandoffV1(ctx, request, s.oauthCallbackHandoffs)
}
