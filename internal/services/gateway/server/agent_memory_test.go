package gateway

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	agentv1 "github.com/JekYUlll/Dipole/api/gen/go/agent/v1"
	"google.golang.org/grpc"
)

type gatewayAgentMemoryRPCStub struct {
	listRequest       *agentv1.ListOwnedMemoriesRequest
	revokeRequest     *agentv1.RevokeOwnedMemoryRequest
	correctRequest    *agentv1.CorrectOwnedMemoryRequest
	listResponse      *agentv1.ListOwnedMemoriesResponse
	revokeResponse    *agentv1.AgentOwnedMemory
	correctResponse   *agentv1.CorrectOwnedMemoryResponse
	promoteRequest    *agentv1.PromoteMemoryCandidateRequest
	promoteResponse   *agentv1.AgentOwnedMemory
	candidateRequest  *agentv1.ListOwnedMemoryCandidatesRequest
	candidateResponse *agentv1.ListOwnedMemoryCandidatesResponse
	reviewRequest     *agentv1.ReviewMemoryCandidateRequest
	reviewResponse    *agentv1.AgentMemoryCandidateSummary
}

func (s *gatewayAgentMemoryRPCStub) ListOwnedMemoryCandidates(_ context.Context, request *agentv1.ListOwnedMemoryCandidatesRequest, _ ...grpc.CallOption) (*agentv1.ListOwnedMemoryCandidatesResponse, error) {
	s.candidateRequest = request
	return s.candidateResponse, nil
}

func (s *gatewayAgentMemoryRPCStub) ListOwnedMemories(_ context.Context, request *agentv1.ListOwnedMemoriesRequest, _ ...grpc.CallOption) (*agentv1.ListOwnedMemoriesResponse, error) {
	s.listRequest = request
	return s.listResponse, nil
}

func (s *gatewayAgentMemoryRPCStub) RevokeOwnedMemory(_ context.Context, request *agentv1.RevokeOwnedMemoryRequest, _ ...grpc.CallOption) (*agentv1.AgentOwnedMemory, error) {
	s.revokeRequest = request
	return s.revokeResponse, nil
}

func (s *gatewayAgentMemoryRPCStub) CorrectOwnedMemory(_ context.Context, request *agentv1.CorrectOwnedMemoryRequest, _ ...grpc.CallOption) (*agentv1.CorrectOwnedMemoryResponse, error) {
	s.correctRequest = request
	return s.correctResponse, nil
}

func (s *gatewayAgentMemoryRPCStub) PromoteMemoryCandidate(_ context.Context, request *agentv1.PromoteMemoryCandidateRequest, _ ...grpc.CallOption) (*agentv1.AgentOwnedMemory, error) {
	s.promoteRequest = request
	return s.promoteResponse, nil
}

func (s *gatewayAgentMemoryRPCStub) ReviewMemoryCandidate(_ context.Context, request *agentv1.ReviewMemoryCandidateRequest, _ ...grpc.CallOption) (*agentv1.AgentMemoryCandidateSummary, error) {
	s.reviewRequest = request
	return s.reviewResponse, nil
}

func TestAgentMemoryControlClientBindsPrincipalAndCanonicalCursor(t *testing.T) {
	now := time.UnixMilli(1_700_000_000_000).UTC()
	active := gatewayAgentMemoryProtoFixture(now)
	rpc := &gatewayAgentMemoryRPCStub{listResponse: &agentv1.ListOwnedMemoriesResponse{
		Memories: []*agentv1.AgentOwnedMemory{active}, NextCreatedAtUnixMs: now.UnixMilli(), NextMemoryId: "MEM-1",
	}}
	client, err := NewAgentMemoryControlClient(rpc, "dipole", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	page, err := client.List(context.Background(), "U100", "", 20)
	if err != nil || len(page.Memories) != 1 || page.NextCursor == "" || rpc.listRequest.GetContext().GetPrincipalUserId() != "U100" || rpc.listRequest.GetTenantId() != "dipole" {
		t.Fatalf("list page=%+v request=%+v err=%v", page, rpc.listRequest, err)
	}
	createdAt, memoryID, err := decodeAgentMemoryCursor(page.NextCursor)
	if err != nil || !createdAt.Equal(now) || memoryID != "MEM-1" {
		t.Fatalf("cursor created=%s memory=%s err=%v", createdAt, memoryID, err)
	}
	if _, err = client.List(context.Background(), "U100", page.NextCursor+"=", 20); !errors.Is(err, ErrAgentMemoryInvalid) {
		t.Fatalf("non-canonical cursor error = %v", err)
	}
}

func TestAgentMemoryControlClientRequiresAuditedRevokeResponse(t *testing.T) {
	now := time.UnixMilli(1_700_000_000_000).UTC()
	revoked := gatewayAgentMemoryProtoFixture(now)
	revoked.Status, revoked.RevokedAtUnixMs, revoked.RevokedById, revoked.RevokeReason = "revoked", now.UnixMilli(), "U100", "outdated"
	rpc := &gatewayAgentMemoryRPCStub{revokeResponse: revoked}
	client, _ := NewAgentMemoryControlClient(rpc, "dipole", time.Second)
	item, err := client.Revoke(context.Background(), "U100", "MEM-1", "outdated")
	if err != nil || item.Status != "revoked" || rpc.revokeRequest.GetContext().GetPrincipalUserId() != "U100" {
		t.Fatalf("revoke item=%+v request=%+v err=%v", item, rpc.revokeRequest, err)
	}
	rpc.revokeResponse.RevokedById = "U999"
	if _, err = client.Revoke(context.Background(), "U100", "MEM-1", "outdated"); !errors.Is(err, ErrAgentMemoryUnavailable) {
		t.Fatalf("forged audit response error = %v", err)
	}
}

func TestAgentMemoryControlClientRequiresCanonicalCorrectionLineage(t *testing.T) {
	now := time.UnixMilli(1_700_000_000_000).UTC()
	previous := gatewayAgentMemoryProtoFixture(now)
	previous.Status, previous.RevokedAtUnixMs, previous.RevokedById, previous.RevokeReason = "revoked", now.UnixMilli(), "U100", "superseded by MEM-2"
	corrected := gatewayAgentMemoryProtoFixture(now)
	corrected.MemoryId, corrected.MemoryVersion = "MEM-2", 2
	corrected.MemoryRootId, corrected.SupersedesMemoryId = "MEM-1", "MEM-1"
	corrected.CorrectedById, corrected.CorrectionReason = "U100", "fix owner"
	corrected.Content, corrected.CompactContent = "Owner is Bob", "Owner: Bob"
	corrected.Provenance = &agentv1.AgentMemoryProvenance{SourceType: "owner_correction", SourceId: "MEM-1", Sequence: "2"}
	rpc := &gatewayAgentMemoryRPCStub{correctResponse: &agentv1.CorrectOwnedMemoryResponse{Previous: previous, Corrected: corrected}}
	client, _ := NewAgentMemoryControlClient(rpc, "dipole", time.Second)
	result, err := client.Correct(context.Background(), "U100", "MEM-1", 1, "Owner is Bob", "Owner: Bob", "fix owner")
	if err != nil || result.Corrected.MemoryVersion != 2 || rpc.correctRequest.GetContext().GetPrincipalUserId() != "U100" {
		t.Fatalf("correct result=%+v request=%+v err=%v", result, rpc.correctRequest, err)
	}
	rpc.correctResponse.Corrected.MemoryRootId = "MEM-FORGED"
	if _, err = client.Correct(context.Background(), "U100", "MEM-1", 1, "Owner is Bob", "Owner: Bob", "fix owner"); !errors.Is(err, ErrAgentMemoryUnavailable) {
		t.Fatalf("forged correction lineage error = %v", err)
	}
}

func TestAgentMemoryControlClientPromotesCandidateWithBoundPrincipal(t *testing.T) {
	now := time.UnixMilli(1_700_000_000_000).UTC()
	rpc := &gatewayAgentMemoryRPCStub{promoteResponse: gatewayAgentMemoryProtoFixture(now)}
	rpc.promoteResponse.MemoryId = "MEM-CAND-1"
	rpc.promoteResponse.MemoryRootId = "MEM-CAND-1"
	rpc.promoteResponse.Provenance.SourceType = "memory_candidate"
	rpc.promoteResponse.Provenance.SourceId = "CAND-1"
	rpc.promoteResponse.Provenance.Sequence = "REV-1"
	client, _ := NewAgentMemoryControlClient(rpc, "dipole", time.Second)
	item, err := client.PromoteCandidate(context.Background(), "U100", "CAND-1", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "REV-1", "semantic")
	if err != nil || item.MemoryID != "MEM-CAND-1" || rpc.promoteRequest.GetContext().GetPrincipalUserId() != "U100" || rpc.promoteRequest.GetCandidateSha256() == "" || rpc.promoteRequest.GetTargetMemoryType() != "semantic" {
		t.Fatalf("promotion item=%+v request=%+v err=%v", item, rpc.promoteRequest, err)
	}
}

func TestAgentMemoryControlClientListsOwnerCandidatesWithoutEvidence(t *testing.T) {
	now := time.UnixMilli(1_700_000_000_000).UTC()
	rpc := &gatewayAgentMemoryRPCStub{candidateResponse: &agentv1.ListOwnedMemoryCandidatesResponse{
		Candidates: []*agentv1.AgentMemoryCandidateSummary{{
			CandidateId: "CAND-1", CandidateSha256: strings.Repeat("a", 64), Summary: "Database migration may slip", Status: "accepted", ReviewId: "REV-1", ObservedAtUnixMs: now.UnixMilli(),
		}},
		NextCursor: "CAND-1",
	}}
	client, err := NewAgentMemoryControlClient(rpc, "dipole", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	page, err := client.ListCandidates(context.Background(), "U100", "", 20)
	if err != nil || len(page.Candidates) != 1 || page.Candidates[0].ReviewID != "REV-1" || page.NextCursor != "CAND-1" || rpc.candidateRequest.GetContext().GetPrincipalUserId() != "U100" {
		t.Fatalf("candidate page=%+v request=%+v err=%v", page, rpc.candidateRequest, err)
	}
	rpc.candidateResponse.Candidates[0].Status = "accepted"
	rpc.candidateResponse.Candidates[0].ReviewId = ""
	if _, err = client.ListCandidates(context.Background(), "U100", "", 20); !errors.Is(err, ErrAgentMemoryUnavailable) {
		t.Fatalf("forged accepted candidate error=%v", err)
	}
}

func TestAgentMemoryControlClientReviewsCandidateWithBoundPrincipal(t *testing.T) {
	now := time.UnixMilli(1_700_000_000_000).UTC()
	rpc := &gatewayAgentMemoryRPCStub{reviewResponse: &agentv1.AgentMemoryCandidateSummary{
		CandidateId: "CAND-1", CandidateSha256: strings.Repeat("a", 64), Summary: "API v2 Friday", Status: "accepted", ReviewId: "REV-1", ObservedAtUnixMs: now.UnixMilli(),
	}}
	client, _ := NewAgentMemoryControlClient(rpc, "dipole", time.Second)
	item, err := client.ReviewCandidate(context.Background(), "U100", "CAND-1", strings.Repeat("a", 64), "accepted", "owner confirmed")
	if err != nil || item.ReviewID != "REV-1" || rpc.reviewRequest.GetContext().GetPrincipalUserId() != "U100" || rpc.reviewRequest.GetDecision() != "accepted" {
		t.Fatalf("review item=%+v request=%+v err=%v", item, rpc.reviewRequest, err)
	}
	rpc.reviewResponse.Status = "rejected"
	if _, err = client.ReviewCandidate(context.Background(), "U100", "CAND-1", strings.Repeat("a", 64), "accepted", "owner confirmed"); !errors.Is(err, ErrAgentMemoryUnavailable) {
		t.Fatalf("forged review status error=%v", err)
	}
}

func gatewayAgentMemoryProtoFixture(now time.Time) *agentv1.AgentOwnedMemory {
	return &agentv1.AgentOwnedMemory{
		MemoryId: "MEM-1", AgentId: "UAI", MemoryType: "semantic", Status: "active",
		ResourceType: "conversation", ResourceId: "group:G1", Content: "Owner is Alice", CompactContent: "Owner: Alice", Priority: 80,
		Provenance:      &agentv1.AgentMemoryProvenance{SourceType: "message", SourceId: "MSG-1", Sequence: "42"},
		ValidFromUnixMs: now.Add(-time.Hour).UnixMilli(), CreatedAtUnixMs: now.UnixMilli(),
		MemoryRootId: "MEM-1", MemoryVersion: 1,
	}
}
