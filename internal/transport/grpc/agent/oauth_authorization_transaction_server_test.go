package agentgrpc

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	agentv1 "github.com/JekYUlll/Dipole/api/gen/go/agent/v1"
	"github.com/JekYUlll/Dipole/internal/application"
	grpccommon "github.com/JekYUlll/Dipole/internal/transport/grpc/common"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type oauthTransactionStoreStub struct {
	record   *application.AgentOAuthAuthorizationTransactionV1
	consumed bool
	claim    []string
}

type oauthCallbackHandoffStoreStub struct {
	record         *application.AgentOAuthCallbackHandoffV1
	claimed        bool
	claim          []string
	leaseExpiresAt time.Time
}

func (s *oauthCallbackHandoffStoreStub) CreateAgentOAuthCallbackHandoff(context.Context, application.AgentOAuthCallbackHandoffV1) (bool, error) {
	return false, errors.New("unused")
}
func (s *oauthCallbackHandoffStoreStub) GetAgentOAuthCallbackHandoff(context.Context, string) (*application.AgentOAuthCallbackHandoffV1, error) {
	return s.record, nil
}
func (s *oauthCallbackHandoffStoreStub) ClaimAgentOAuthCallbackHandoff(_ context.Context, id, owner string, _ time.Time, leaseExpiresAt time.Time) (bool, error) {
	s.claim, s.leaseExpiresAt = []string{id, owner}, leaseExpiresAt
	if s.claimed && s.record != nil {
		s.record.Status, s.record.LeaseOwner, s.record.LeaseExpiresAt = application.AgentOAuthCallbackHandoffClaimedV1, owner, leaseExpiresAt
	}
	return s.claimed, nil
}
func (*oauthCallbackHandoffStoreStub) CompleteAgentOAuthCallbackHandoff(context.Context, string, string, time.Time) (bool, error) {
	return false, errors.New("unused")
}
func (*oauthCallbackHandoffStoreStub) ReleaseAgentOAuthCallbackHandoff(context.Context, string, string, time.Time) (bool, error) {
	return false, errors.New("unused")
}

type memoryPromotionReceiptCommitStub struct{}

func (memoryPromotionReceiptCommitStub) CommitMemoryPromotionReceipt(context.Context, application.AgentMemoryPromotionReceiptCommitRequestV1) (*application.AgentMemoryV1, error) {
	return nil, nil
}

func (s *oauthTransactionStoreStub) CreateAgentOAuthAuthorizationTransaction(context.Context, application.AgentOAuthAuthorizationTransactionV1) (bool, error) {
	return false, errors.New("unused")
}
func (s *oauthTransactionStoreStub) GetAgentOAuthAuthorizationTransaction(context.Context, string) (*application.AgentOAuthAuthorizationTransactionV1, error) {
	return s.record, nil
}
func (s *oauthTransactionStoreStub) ConsumeAgentOAuthAuthorizationTransaction(_ context.Context, id, owner, state string, _ time.Time) (bool, error) {
	s.claim = []string{id, owner, state}
	return s.consumed, nil
}

func TestConsumeOAuthAuthorizationTransactionRequiresGatewayOwnerAndSingleConsume(t *testing.T) {
	now := time.Now().UTC().Add(5 * time.Minute)
	store := &oauthTransactionStoreStub{record: &application.AgentOAuthAuthorizationTransactionV1{
		TransactionUUID: strings.Repeat("a", 22), OwnerUserUUID: "U100", Issuer: "https://auth.example.com/tenant",
		RedirectURI: "https://dipole.example.com/oauth/callback", StateSHA256: strings.Repeat("b", 64),
		SealedCodeVerifier: "v1.abc.def.ghi", CreatedAt: time.Now().UTC(), ExpiresAt: now,
	}, consumed: true}
	server, err := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = server.WithOAuthAuthorizationTransactions(store); err != nil {
		t.Fatal(err)
	}
	request := &agentv1.ConsumeOAuthAuthorizationTransactionRequest{Context: grpccommon.RequestContext("U100", "dipole-gateway"), TransactionId: strings.Repeat("a", 22), StateSha256: strings.Repeat("b", 64)}
	response, err := server.ConsumeOAuthAuthorizationTransaction(context.Background(), request)
	if err != nil || response.GetSealedCodeVerifier() != "v1.abc.def.ghi" || len(store.claim) != 3 {
		t.Fatalf("response=%v err=%v claim=%v", response, err, store.claim)
	}
	request.Context.CallerService = "dipole-agent"
	if _, err = server.ConsumeOAuthAuthorizationTransaction(context.Background(), request); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected denied, got %v", err)
	}
	request.Context.CallerService = "dipole-gateway"
	request.Context.PrincipalUserId = "U200"
	if _, err = server.ConsumeOAuthAuthorizationTransaction(context.Background(), request); status.Code(err) != codes.NotFound {
		t.Fatalf("expected unavailable, got %v", err)
	}
}

func TestConsumeOAuthAuthorizationTransactionFailsClosedWithoutStore(t *testing.T) {
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{})
	_, err := server.ConsumeOAuthAuthorizationTransaction(context.Background(), &agentv1.ConsumeOAuthAuthorizationTransactionRequest{
		Context: grpccommon.RequestContext("U100", "dipole-gateway"), TransactionId: strings.Repeat("a", 22), StateSha256: strings.Repeat("b", 64),
	})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("expected unavailable, got %v", err)
	}
}

func TestClaimOAuthCallbackHandoffRequiresAgentRuntimeAndReturnsOpaqueRecord(t *testing.T) {
	store := &oauthCallbackHandoffStoreStub{claimed: true, record: &application.AgentOAuthCallbackHandoffV1{
		HandoffUUID: strings.Repeat("a", 22), TransactionUUID: strings.Repeat("b", 22), OwnerUserUUID: "U100",
		Issuer: "https://auth.example.com/tenant", RedirectURI: "https://dipole.example.com/oauth/callback",
		AuthorizationCodeSHA256: strings.Repeat("c", 64), SealedAuthorizationCode: "v1.nonce.ciphertext.tag.wrapped-dek", RuntimeKeyID: "runtime-key-1",
		Status: application.AgentOAuthCallbackHandoffRecordedV1, ExpiresAt: time.Now().UTC().Add(5 * time.Minute),
	}}
	server, err := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = server.WithOAuthCallbackHandoffs(store); err != nil {
		t.Fatal(err)
	}
	request := &agentv1.ClaimOAuthCallbackHandoffRequest{Context: grpccommon.RequestContext("U-untrusted", "dipole-agent"), HandoffId: store.record.HandoffUUID, LeaseOwner: "runtime-1"}
	response, err := server.ClaimOAuthCallbackHandoff(context.Background(), request)
	if err != nil || response.GetSealedAuthorizationCode() != store.record.SealedAuthorizationCode || response.GetLeaseExpiresAtUnixMs() == 0 || len(store.claim) != 2 {
		t.Fatalf("response=%v err=%v claim=%v", response, err, store.claim)
	}
	request.Context.CallerService = "dipole-gateway"
	if _, err = server.ClaimOAuthCallbackHandoff(context.Background(), request); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected denied, got %v", err)
	}
}

func TestClaimOAuthCallbackHandoffFailsClosedWithoutStore(t *testing.T) {
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{})
	_, err := server.ClaimOAuthCallbackHandoff(context.Background(), &agentv1.ClaimOAuthCallbackHandoffRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), HandoffId: strings.Repeat("a", 22), LeaseOwner: "runtime-1",
	})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("expected unavailable, got %v", err)
	}
}

func TestRestrictedServerComposesOAuthWithReceiptBoundary(t *testing.T) {
	store := &oauthTransactionStoreStub{}
	server, err := NewOAuthAuthorizationTransactionServer(store)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = server.WithMemoryPromotionReceiptCommits(&memoryPromotionReceiptCommitStub{}); err != nil {
		t.Fatalf("compose receipt boundary: %v", err)
	}
	if server.oauthTransactions != store || server.commits == nil {
		t.Fatal("restricted server did not retain both independently gated seams")
	}
}
