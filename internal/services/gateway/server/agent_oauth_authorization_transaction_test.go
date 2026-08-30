package gateway

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	agentv1 "github.com/JekYUlll/Dipole/api/gen/go/agent/v1"
	"github.com/JekYUlll/Dipole/internal/platform/correlation"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type agentOAuthAuthorizationTransactionRPCStub struct {
	request  *agentv1.ConsumeOAuthAuthorizationTransactionRequest
	response *agentv1.ConsumeOAuthAuthorizationTransactionResponse
	err      error
}

func (s *agentOAuthAuthorizationTransactionRPCStub) ConsumeOAuthAuthorizationTransaction(_ context.Context, request *agentv1.ConsumeOAuthAuthorizationTransactionRequest, _ ...grpc.CallOption) (*agentv1.ConsumeOAuthAuthorizationTransactionResponse, error) {
	s.request = request
	return s.response, s.err
}

func TestAgentOAuthAuthorizationTransactionClientBindsGatewayPrincipalAndCorrelation(t *testing.T) {
	transactionID, stateSHA256 := strings.Repeat("a", 22), strings.Repeat("b", 64)
	rpc := &agentOAuthAuthorizationTransactionRPCStub{response: &agentv1.ConsumeOAuthAuthorizationTransactionResponse{
		TransactionId: transactionID, Issuer: "https://auth.example.com/tenant", RedirectUri: "https://dipole.example.com/oauth/callback",
		SealedCodeVerifier: "v1.abc.def.ghi", ExpiresAtUnixMs: time.Now().UTC().Add(time.Minute).UnixMilli(),
	}}
	client, err := NewAgentOAuthAuthorizationTransactionClient(rpc, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	ctx := correlation.WithContext(context.Background(), correlation.IDs{RequestID: "REQ-1", TraceID: "TRACE-1"})
	result, err := client.Consume(ctx, "U100", transactionID, stateSHA256)
	if err != nil || result == nil || result.SealedCodeVerifier != "v1.abc.def.ghi" {
		t.Fatalf("consume result=%+v err=%v", result, err)
	}
	requestContext := rpc.request.GetContext()
	if requestContext.GetCallerService() != "dipole-gateway" || requestContext.GetPrincipalUserId() != "U100" ||
		requestContext.GetRequestId() != "REQ-1" || requestContext.GetTraceId() != "TRACE-1" ||
		rpc.request.GetTransactionId() != transactionID || rpc.request.GetStateSha256() != stateSHA256 {
		t.Fatalf("unexpected consume request: %+v", rpc.request)
	}
}

func TestAgentOAuthAuthorizationTransactionClientFailsClosedForInvalidResponseAndErrors(t *testing.T) {
	transactionID, stateSHA256 := strings.Repeat("a", 22), strings.Repeat("b", 64)
	cases := []struct {
		name string
		stub agentOAuthAuthorizationTransactionRPCStub
		want error
	}{
		{name: "denied", stub: agentOAuthAuthorizationTransactionRPCStub{err: status.Error(codes.NotFound, "missing")}, want: ErrAgentOAuthAuthorizationTransactionDenied},
		{name: "unavailable", stub: agentOAuthAuthorizationTransactionRPCStub{err: status.Error(codes.Unavailable, "down")}, want: ErrAgentOAuthAuthorizationTransactionUnavailable},
		{name: "expired response", stub: agentOAuthAuthorizationTransactionRPCStub{response: &agentv1.ConsumeOAuthAuthorizationTransactionResponse{TransactionId: transactionID, Issuer: "https://auth.example.com", RedirectUri: "https://dipole.example.com/callback", SealedCodeVerifier: "v1.abc.def.ghi", ExpiresAtUnixMs: 1}}, want: ErrAgentOAuthAuthorizationTransactionUnavailable},
		{name: "invalid input", stub: agentOAuthAuthorizationTransactionRPCStub{}, want: ErrAgentOAuthAuthorizationTransactionInvalid},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			client, err := NewAgentOAuthAuthorizationTransactionClient(&test.stub, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			inputState := stateSHA256
			if test.name == "invalid input" {
				inputState = "bad"
			}
			_, err = client.Consume(context.Background(), "U100", transactionID, inputState)
			if !errors.Is(err, test.want) {
				t.Fatalf("error=%v want=%v", err, test.want)
			}
		})
	}
}

func TestAgentOAuthAuthorizationTransactionClientRejectsNonCanonicalTransactionMaterial(t *testing.T) {
	rpc := &agentOAuthAuthorizationTransactionRPCStub{}
	client, _ := NewAgentOAuthAuthorizationTransactionClient(rpc, time.Second)
	_, err := client.Consume(context.Background(), "U100", "transaction:with:colon", strings.Repeat("b", 64))
	if !errors.Is(err, ErrAgentOAuthAuthorizationTransactionInvalid) || rpc.request != nil {
		t.Fatalf("error=%v request=%+v", err, rpc.request)
	}
	if validOAuthSealedCodeVerifier("v1.abc.def.gh:i") {
		t.Fatal("sealed verifier must use base64url components")
	}
}
