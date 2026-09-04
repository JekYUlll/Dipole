package gateway

import (
	"context"
	"errors"
	"strings"
	"time"

	agentv1 "github.com/JekYUlll/Dipole/api/gen/go/agent/v1"
	grpccommon "github.com/JekYUlll/Dipole/internal/transport/grpc/common"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrAgentOAuthCallbackHandoffRecordInvalid     = errors.New("Agent OAuth callback handoff record is invalid")
	ErrAgentOAuthCallbackHandoffRecordDenied      = errors.New("Agent OAuth callback handoff record is unavailable")
	ErrAgentOAuthCallbackHandoffRecordUnavailable = errors.New("Agent OAuth callback handoff record service is unavailable")
)

type agentOAuthCallbackHandoffRecordRPC interface {
	RecordOAuthCallbackHandoff(context.Context, *agentv1.RecordOAuthCallbackHandoffRequest, ...grpc.CallOption) (*agentv1.RecordOAuthCallbackHandoffResponse, error)
}

// AgentOAuthCallbackHandoffRecordInput contains only the validated digest and
// Runtime-only envelope. The raw authorization code stays in the callback
// handler and must never enter this client or its logs.
type AgentOAuthCallbackHandoffRecordInput struct {
	HandoffID, TransactionID, StateSHA256, AuthorizationCodeSHA256 string
	SealedAuthorizationCode, RuntimeKeyID                          string
}

type AgentOAuthCallbackHandoffRecordResult struct {
	HandoffID string
	ExpiresAt time.Time
}

// AgentOAuthCallbackHandoffRecordClient is the unmounted Gateway-to-Core seam
// used by a future validated callback handler. It does not add an HTTP route.
type AgentOAuthCallbackHandoffRecordClient struct {
	rpc     agentOAuthCallbackHandoffRecordRPC
	timeout time.Duration
}

func NewAgentOAuthCallbackHandoffRecordClient(rpc agentOAuthCallbackHandoffRecordRPC, timeout time.Duration) (*AgentOAuthCallbackHandoffRecordClient, error) {
	if rpc == nil {
		return nil, ErrAgentOAuthCallbackHandoffRecordInvalid
	}
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &AgentOAuthCallbackHandoffRecordClient{rpc: rpc, timeout: timeout}, nil
}

func (c *AgentOAuthCallbackHandoffRecordClient) Record(ctx context.Context, principalUUID string, input AgentOAuthCallbackHandoffRecordInput) (*AgentOAuthCallbackHandoffRecordResult, error) {
	principalUUID = strings.TrimSpace(principalUUID)
	if c == nil || !validAgentSubscriptionPublicID(principalUUID, 64) || !validAgentOAuthCallbackHandoffRecordInput(input) {
		return nil, ErrAgentOAuthCallbackHandoffRecordInvalid
	}
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := c.rpc.RecordOAuthCallbackHandoff(callCtx, &agentv1.RecordOAuthCallbackHandoffRequest{
		Context:                 grpccommon.RequestContextFrom(ctx, principalUUID, "dipole-gateway"),
		HandoffId:               input.HandoffID,
		TransactionId:           input.TransactionID,
		StateSha256:             input.StateSHA256,
		AuthorizationCodeSha256: input.AuthorizationCodeSHA256,
		SealedAuthorizationCode: input.SealedAuthorizationCode,
		RuntimeKeyId:            input.RuntimeKeyID,
	})
	if err != nil {
		return nil, mapAgentOAuthCallbackHandoffRecordRPCError(err)
	}
	result, err := agentOAuthCallbackHandoffRecordResultFromProto(response)
	if err != nil || result.HandoffID != input.HandoffID {
		return nil, ErrAgentOAuthCallbackHandoffRecordUnavailable
	}
	return result, nil
}

func validAgentOAuthCallbackHandoffRecordInput(input AgentOAuthCallbackHandoffRecordInput) bool {
	return validOAuthTransactionID(input.HandoffID) && validOAuthTransactionID(input.TransactionID) &&
		len(input.StateSHA256) == 64 && isLowerHex(input.StateSHA256) &&
		len(input.AuthorizationCodeSHA256) == 64 && isLowerHex(input.AuthorizationCodeSHA256) &&
		validAgentOAuthCallbackHandoffEnvelope(input.SealedAuthorizationCode) &&
		validAgentOAuthCallbackEnvelopeIdentifier(input.RuntimeKeyID, 128)
}

func validAgentOAuthCallbackHandoffEnvelope(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) != 5 || parts[0] != agentOAuthCallbackEnvelopeVersion || len(value) > 4096 {
		return false
	}
	for _, part := range parts[1:] {
		if len(part) > 2048 || !validOAuthBase64URL(part) {
			return false
		}
	}
	return true
}

func agentOAuthCallbackHandoffRecordResultFromProto(response *agentv1.RecordOAuthCallbackHandoffResponse) (*AgentOAuthCallbackHandoffRecordResult, error) {
	if response == nil || !validOAuthTransactionID(response.GetHandoffId()) || response.GetExpiresAtUnixMs() <= time.Now().UTC().UnixMilli() {
		return nil, ErrAgentOAuthCallbackHandoffRecordUnavailable
	}
	return &AgentOAuthCallbackHandoffRecordResult{
		HandoffID: response.GetHandoffId(), ExpiresAt: time.UnixMilli(response.GetExpiresAtUnixMs()).UTC(),
	}, nil
}

func mapAgentOAuthCallbackHandoffRecordRPCError(err error) error {
	switch status.Code(err) {
	case codes.InvalidArgument, codes.FailedPrecondition:
		return ErrAgentOAuthCallbackHandoffRecordInvalid
	case codes.Unauthenticated, codes.PermissionDenied, codes.NotFound:
		return ErrAgentOAuthCallbackHandoffRecordDenied
	default:
		return ErrAgentOAuthCallbackHandoffRecordUnavailable
	}
}
