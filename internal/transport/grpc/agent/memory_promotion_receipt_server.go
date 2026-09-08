package agentgrpc

import (
	"context"
	"errors"
	"strings"

	agentv1 "github.com/JekYUlll/Dipole/api/gen/go/agent/v1"
	"github.com/JekYUlll/Dipole/internal/application"
	grpccommon "github.com/JekYUlll/Dipole/internal/transport/grpc/common"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RestrictedServer exposes independently opt-in Agent seams without requiring
// the full interactive capability composition in standalone Core.
type RestrictedServer struct {
	agentv1.UnimplementedAgentCapabilityServiceServer
	commits               application.AgentMemoryPromotionReceiptCommitServiceV1
	memories              application.AgentMemoryContextResolverV1
	oauthTransactions     application.AgentOAuthAuthorizationTransactionStoreV1
	oauthCallbackHandoffs application.AgentOAuthCallbackHandoffStoreV1
	oauthCallbackRecorder application.AgentOAuthCallbackHandoffRecorderV1
	oauthTokenLifecycles  application.AgentOAuthTokenLifecycleStoreV1
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

func NewOAuthCallbackHandoffServer(handoffs application.AgentOAuthCallbackHandoffStoreV1) (*RestrictedServer, error) {
	if handoffs == nil {
		return nil, errors.New("Agent OAuth callback handoff store is required")
	}
	return &RestrictedServer{oauthCallbackHandoffs: handoffs}, nil
}

func (s *RestrictedServer) WithMemoryPromotionReceiptCommits(commits application.AgentMemoryPromotionReceiptCommitServiceV1) (*RestrictedServer, error) {
	if s == nil || commits == nil {
		return nil, errors.New("Agent Memory promotion receipt commit service is required")
	}
	s.commits = commits
	return s, nil
}

// WithMemories enables the read-only Context Memory seam for a restricted Core
// deployment. The caller identity and Task/Run binding remain enforced by the
// same RPC contract used by the full Agent capability server.
func (s *RestrictedServer) WithMemories(memories application.AgentMemoryContextResolverV1) (*RestrictedServer, error) {
	if s == nil || memories == nil {
		return nil, errors.New("Agent Memory resolver is required")
	}
	s.memories = memories
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

func (s *RestrictedServer) WithOAuthCallbackHandoffRecorder(recorder application.AgentOAuthCallbackHandoffRecorderV1) (*RestrictedServer, error) {
	if s == nil || recorder == nil {
		return nil, errors.New("Agent OAuth callback handoff recorder is required")
	}
	s.oauthCallbackRecorder = recorder
	return s, nil
}

func (s *RestrictedServer) WithOAuthTokenLifecycles(store application.AgentOAuthTokenLifecycleStoreV1) (*RestrictedServer, error) {
	if s == nil || store == nil {
		return nil, errors.New("Agent OAuth token lifecycle store is required")
	}
	s.oauthTokenLifecycles = store
	return s, nil
}

func (s *RestrictedServer) CommitMemoryPromotionReceipt(ctx context.Context, request *agentv1.CommitMemoryPromotionReceiptRequest) (*agentv1.CommitMemoryPromotionReceiptResponse, error) {
	if s == nil {
		return nil, errors.New("Agent Memory promotion receipt server is unavailable")
	}
	return commitMemoryPromotionReceiptV1(ctx, request, s.commits)
}

func (s *RestrictedServer) ListContextMemories(ctx context.Context, request *agentv1.ListContextMemoriesRequest) (*agentv1.ListContextMemoriesResponse, error) {
	caller, err := authenticatedAgentArtifactCallerV1(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	if caller != "dipole-agent" || strings.TrimSpace(request.GetContext().GetPrincipalUserId()) != "" {
		return nil, status.Error(codes.PermissionDenied, "only the authenticated Agent runtime may list Context Memories")
	}
	if s == nil || s.memories == nil {
		return nil, status.Error(codes.Unavailable, "Agent Memory resolver is unavailable")
	}
	items, err := s.memories.ResolveContextMemories(grpccommon.Correlation(ctx, request.GetContext()), request.GetTaskId(), request.GetRunId(), request.GetResourceType(), request.GetResourceId(), int(request.GetLimit()))
	if err != nil {
		switch {
		case errors.Is(err, application.ErrAgentMemoryDenied):
			return nil, status.Error(codes.PermissionDenied, "Agent Memory scope denied")
		case errors.Is(err, application.ErrAgentMemoryInvalid), errors.Is(err, application.ErrAgentExecutionPolicyDenied):
			return nil, status.Error(codes.FailedPrecondition, "Agent Memory request is invalid")
		default:
			return nil, status.Error(codes.Internal, "Agent Memory lookup failed")
		}
	}
	response := &agentv1.ListContextMemoriesResponse{Memories: make([]*agentv1.AgentContextMemory, 0, len(items))}
	for _, item := range items {
		response.Memories = append(response.Memories, &agentv1.AgentContextMemory{
			MemoryId: item.MemoryUUID, MemoryType: string(item.MemoryType), Content: item.Content,
			CompactContent: item.CompactContent, Priority: item.Priority,
			Provenance: &agentv1.AgentMemoryProvenance{SourceType: item.Provenance.SourceType, SourceId: item.Provenance.SourceID, Uri: item.Provenance.URI, Sequence: item.Provenance.Sequence},
		})
	}
	return response, nil
}

func (s *RestrictedServer) ConsumeOAuthAuthorizationTransaction(ctx context.Context, request *agentv1.ConsumeOAuthAuthorizationTransactionRequest) (*agentv1.ConsumeOAuthAuthorizationTransactionResponse, error) {
	if s == nil {
		return nil, errors.New("Agent OAuth authorization transaction server is unavailable")
	}
	return consumeOAuthAuthorizationTransactionV1(ctx, request, s.oauthTransactions)
}

func (s *RestrictedServer) RecordOAuthCallbackHandoff(ctx context.Context, request *agentv1.RecordOAuthCallbackHandoffRequest) (*agentv1.RecordOAuthCallbackHandoffResponse, error) {
	if s == nil {
		return nil, errors.New("Agent OAuth callback handoff server is unavailable")
	}
	return recordOAuthCallbackHandoffV1(ctx, request, s.oauthCallbackRecorder)
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

func (s *RestrictedServer) PersistOAuthTokenLifecycle(ctx context.Context, request *agentv1.PersistOAuthTokenLifecycleRequest) (*agentv1.PersistOAuthTokenLifecycleResponse, error) {
	if s == nil {
		return nil, errors.New("Agent OAuth token lifecycle server is unavailable")
	}
	return persistOAuthTokenLifecycleV1(ctx, request, s.oauthCallbackHandoffs, s.oauthTokenLifecycles)
}
