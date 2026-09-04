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
	complete       bool
	release        bool
	completed      []string
	released       []string
}

type oauthCallbackHandoffRecorderStub struct {
	record *application.AgentOAuthCallbackHandoffV1
	input  application.AgentOAuthCallbackHandoffRecordRequestV1
	err    error
}

func (s *oauthCallbackHandoffRecorderStub) RecordAgentOAuthCallbackHandoff(_ context.Context, input application.AgentOAuthCallbackHandoffRecordRequestV1, _ time.Time) (*application.AgentOAuthCallbackHandoffV1, bool, error) {
	s.input = input
	return s.record, s.record != nil, s.err
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
func (s *oauthCallbackHandoffStoreStub) CompleteAgentOAuthCallbackHandoff(_ context.Context, handoffID, leaseOwner string, _ time.Time) (bool, error) {
	s.completed = []string{handoffID, leaseOwner}
	return s.complete, nil
}
func (s *oauthCallbackHandoffStoreStub) ReleaseAgentOAuthCallbackHandoff(_ context.Context, handoffID, leaseOwner string, _ time.Time) (bool, error) {
	s.released = []string{handoffID, leaseOwner}
	return s.release, nil
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

func TestRecordOAuthCallbackHandoffRequiresGatewayAndTrustedOwner(t *testing.T) {
	transactionID, handoffID := strings.Repeat("a", 22), strings.Repeat("b", 22)
	recorder := &oauthCallbackHandoffRecorderStub{record: &application.AgentOAuthCallbackHandoffV1{HandoffUUID: handoffID, TransactionUUID: transactionID, OwnerUserUUID: "U100", ExpiresAt: time.Now().UTC().Add(time.Hour)}}
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{})
	if _, err := server.WithOAuthCallbackHandoffRecorder(recorder); err != nil {
		t.Fatal(err)
	}
	request := &agentv1.RecordOAuthCallbackHandoffRequest{Context: grpccommon.RequestContext("U100", "dipole-gateway"), HandoffId: handoffID, TransactionId: transactionID,
		StateSha256: strings.Repeat("c", 64), AuthorizationCodeSha256: strings.Repeat("d", 64), SealedAuthorizationCode: "v1.abc.def.ghi", RuntimeKeyId: "runtime-key-1"}
	response, err := server.RecordOAuthCallbackHandoff(context.Background(), request)
	if err != nil || response.GetHandoffId() != handoffID || recorder.input.OwnerUserUUID != "U100" || recorder.input.HandoffUUID != handoffID {
		t.Fatalf("response=%v err=%v input=%+v", response, err, recorder.input)
	}
	request.Context.CallerService = "dipole-agent"
	if _, err = server.RecordOAuthCallbackHandoff(context.Background(), request); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected denied, got %v", err)
	}
}

func TestRecordOAuthCallbackHandoffFailsClosedWithoutRecorder(t *testing.T) {
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{})
	_, err := server.RecordOAuthCallbackHandoff(context.Background(), &agentv1.RecordOAuthCallbackHandoffRequest{Context: grpccommon.RequestContext("U100", "dipole-gateway")})
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

func TestOAuthCallbackHandoffTerminalRPCsRequireAgentRuntimeLease(t *testing.T) {
	store := &oauthCallbackHandoffStoreStub{complete: true, release: true}
	server, err := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = server.WithOAuthCallbackHandoffs(store); err != nil {
		t.Fatal(err)
	}
	handoffID := strings.Repeat("a", 22)
	requestContext := grpccommon.RequestContext("U-untrusted", "dipole-agent")
	completed, err := server.CompleteOAuthCallbackHandoff(context.Background(), &agentv1.CompleteOAuthCallbackHandoffRequest{Context: requestContext, HandoffId: handoffID, LeaseOwner: "runtime-1"})
	if err != nil || completed.GetHandoffId() != handoffID || strings.Join(store.completed, ":") != handoffID+":runtime-1" {
		t.Fatalf("complete=%v err=%v record=%v", completed, err, store.completed)
	}
	released, err := server.ReleaseOAuthCallbackHandoff(context.Background(), &agentv1.ReleaseOAuthCallbackHandoffRequest{Context: requestContext, HandoffId: handoffID, LeaseOwner: "runtime-1"})
	if err != nil || released.GetHandoffId() != handoffID || strings.Join(store.released, ":") != handoffID+":runtime-1" {
		t.Fatalf("release=%v err=%v record=%v", released, err, store.released)
	}
	requestContext.CallerService = "dipole-gateway"
	if _, err = server.CompleteOAuthCallbackHandoff(context.Background(), &agentv1.CompleteOAuthCallbackHandoffRequest{Context: requestContext, HandoffId: handoffID, LeaseOwner: "runtime-1"}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected denied, got %v", err)
	}
}

func TestOAuthCallbackHandoffTerminalRPCsFailClosedWithoutStore(t *testing.T) {
	server, _ := NewServer(&capabilityStub{}, resolverStub{}, &admissionStub{})
	requestContext := grpccommon.RequestContext("", "dipole-agent")
	handoffID := strings.Repeat("a", 22)
	_, err := server.CompleteOAuthCallbackHandoff(context.Background(), &agentv1.CompleteOAuthCallbackHandoffRequest{Context: requestContext, HandoffId: handoffID, LeaseOwner: "runtime-1"})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("expected unavailable, got %v", err)
	}
	_, err = server.ReleaseOAuthCallbackHandoff(context.Background(), &agentv1.ReleaseOAuthCallbackHandoffRequest{Context: requestContext, HandoffId: handoffID, LeaseOwner: "runtime-1"})
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
