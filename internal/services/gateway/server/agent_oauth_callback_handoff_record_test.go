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

type agentOAuthCallbackHandoffRecordRPCStub struct {
	request  *agentv1.RecordOAuthCallbackHandoffRequest
	response *agentv1.RecordOAuthCallbackHandoffResponse
	err      error
}

func (s *agentOAuthCallbackHandoffRecordRPCStub) RecordOAuthCallbackHandoff(_ context.Context, request *agentv1.RecordOAuthCallbackHandoffRequest, _ ...grpc.CallOption) (*agentv1.RecordOAuthCallbackHandoffResponse, error) {
	s.request = request
	return s.response, s.err
}

func TestAgentOAuthCallbackHandoffRecordClientBindsGatewayPrincipalAndCorrelation(t *testing.T) {
	handoffID, transactionID := strings.Repeat("a", 22), strings.Repeat("b", 22)
	stateSHA256, codeSHA256 := strings.Repeat("c", 64), strings.Repeat("d", 64)
	rpc := &agentOAuthCallbackHandoffRecordRPCStub{response: &agentv1.RecordOAuthCallbackHandoffResponse{
		HandoffId: handoffID, ExpiresAtUnixMs: time.Now().UTC().Add(time.Minute).UnixMilli(),
	}}
	client, err := NewAgentOAuthCallbackHandoffRecordClient(rpc, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	ctx := correlation.WithContext(context.Background(), correlation.IDs{RequestID: "REQ-1", TraceID: "TRACE-1"})
	result, err := client.Record(ctx, "U100", AgentOAuthCallbackHandoffRecordInput{
		HandoffID: handoffID, TransactionID: transactionID, StateSHA256: stateSHA256,
		AuthorizationCodeSHA256: codeSHA256, SealedAuthorizationCode: "v1.nonce.ciphertext.tag.wrapped", RuntimeKeyID: "runtime-key-1",
	})
	if err != nil || result == nil || result.HandoffID != handoffID {
		t.Fatalf("record result=%+v err=%v", result, err)
	}
	requestContext := rpc.request.GetContext()
	if requestContext.GetCallerService() != "dipole-gateway" || requestContext.GetPrincipalUserId() != "U100" ||
		requestContext.GetRequestId() != "REQ-1" || requestContext.GetTraceId() != "TRACE-1" ||
		rpc.request.GetTransactionId() != transactionID || rpc.request.GetSealedAuthorizationCode() != "v1.nonce.ciphertext.tag.wrapped" {
		t.Fatalf("unexpected record request: %+v", rpc.request)
	}
}

func TestAgentOAuthCallbackHandoffRecordClientFailsClosed(t *testing.T) {
	handoffID, transactionID := strings.Repeat("a", 22), strings.Repeat("b", 22)
	valid := AgentOAuthCallbackHandoffRecordInput{
		HandoffID: handoffID, TransactionID: transactionID, StateSHA256: strings.Repeat("c", 64),
		AuthorizationCodeSHA256: strings.Repeat("d", 64), SealedAuthorizationCode: "v1.nonce.ciphertext.tag.wrapped", RuntimeKeyID: "runtime-key-1",
	}
	cases := []struct {
		name  string
		input AgentOAuthCallbackHandoffRecordInput
		stub  agentOAuthCallbackHandoffRecordRPCStub
		want  error
	}{
		{name: "denied", input: valid, stub: agentOAuthCallbackHandoffRecordRPCStub{err: status.Error(codes.PermissionDenied, "denied")}, want: ErrAgentOAuthCallbackHandoffRecordDenied},
		{name: "unavailable", input: valid, stub: agentOAuthCallbackHandoffRecordRPCStub{err: status.Error(codes.Unavailable, "down")}, want: ErrAgentOAuthCallbackHandoffRecordUnavailable},
		{name: "expired response", input: valid, stub: agentOAuthCallbackHandoffRecordRPCStub{response: &agentv1.RecordOAuthCallbackHandoffResponse{HandoffId: handoffID, ExpiresAtUnixMs: 1}}, want: ErrAgentOAuthCallbackHandoffRecordUnavailable},
		{name: "mismatched response", input: valid, stub: agentOAuthCallbackHandoffRecordRPCStub{response: &agentv1.RecordOAuthCallbackHandoffResponse{HandoffId: transactionID, ExpiresAtUnixMs: time.Now().UTC().Add(time.Minute).UnixMilli()}}, want: ErrAgentOAuthCallbackHandoffRecordUnavailable},
		{name: "invalid envelope", input: AgentOAuthCallbackHandoffRecordInput{HandoffID: handoffID, TransactionID: transactionID, StateSHA256: strings.Repeat("c", 64), AuthorizationCodeSHA256: strings.Repeat("d", 64), SealedAuthorizationCode: "plaintext", RuntimeKeyID: "runtime-key-1"}, want: ErrAgentOAuthCallbackHandoffRecordInvalid},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			client, err := NewAgentOAuthCallbackHandoffRecordClient(&test.stub, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.Record(context.Background(), "U100", test.input)
			if !errors.Is(err, test.want) {
				t.Fatalf("error=%v want=%v", err, test.want)
			}
			if test.want == ErrAgentOAuthCallbackHandoffRecordInvalid && test.stub.request != nil {
				t.Fatalf("invalid input reached RPC: %+v", test.stub.request)
			}
		})
	}
}
