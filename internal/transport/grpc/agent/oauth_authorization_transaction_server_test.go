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
