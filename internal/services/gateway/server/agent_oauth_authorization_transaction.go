package gateway

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	agentv1 "github.com/JekYUlll/Dipole/api/gen/go/agent/v1"
	grpccommon "github.com/JekYUlll/Dipole/internal/transport/grpc/common"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrAgentOAuthAuthorizationTransactionInvalid     = errors.New("Agent OAuth authorization transaction is invalid")
	ErrAgentOAuthAuthorizationTransactionDenied      = errors.New("Agent OAuth authorization transaction is unavailable")
	ErrAgentOAuthAuthorizationTransactionUnavailable = errors.New("Agent OAuth authorization transaction service is unavailable")
)

// AgentOAuthAuthorizationTransactionConsumeResult is internal-only. It must
// never be serialized into a browser response, audit record, or log entry.
type AgentOAuthAuthorizationTransactionConsumeResult struct {
	TransactionID      string
	Issuer             string
	RedirectURI        string
	SealedCodeVerifier string
	ExpiresAt          time.Time
}

type agentOAuthAuthorizationTransactionRPC interface {
	ConsumeOAuthAuthorizationTransaction(context.Context, *agentv1.ConsumeOAuthAuthorizationTransactionRequest, ...grpc.CallOption) (*agentv1.ConsumeOAuthAuthorizationTransactionResponse, error)
}

// AgentOAuthAuthorizationTransactionClient owns the Gateway-to-Core consume
// seam. It is intentionally not registered with Gateway HTTP routing yet.
type AgentOAuthAuthorizationTransactionClient struct {
	rpc     agentOAuthAuthorizationTransactionRPC
	timeout time.Duration
}

func NewAgentOAuthAuthorizationTransactionClient(rpc agentOAuthAuthorizationTransactionRPC, timeout time.Duration) (*AgentOAuthAuthorizationTransactionClient, error) {
	if rpc == nil {
		return nil, ErrAgentOAuthAuthorizationTransactionInvalid
	}
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &AgentOAuthAuthorizationTransactionClient{rpc: rpc, timeout: timeout}, nil
}

func (c *AgentOAuthAuthorizationTransactionClient) Consume(ctx context.Context, principalUUID, transactionID, stateSHA256 string) (*AgentOAuthAuthorizationTransactionConsumeResult, error) {
	principalUUID, transactionID, stateSHA256 = strings.TrimSpace(principalUUID), strings.TrimSpace(transactionID), strings.TrimSpace(stateSHA256)
	if !validAgentSubscriptionPublicID(principalUUID, 64) || !validOAuthTransactionID(transactionID) || !isLowerHex(stateSHA256) || len(stateSHA256) != 64 {
		return nil, ErrAgentOAuthAuthorizationTransactionInvalid
	}
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := c.rpc.ConsumeOAuthAuthorizationTransaction(callCtx, &agentv1.ConsumeOAuthAuthorizationTransactionRequest{
		Context: grpccommon.RequestContextFrom(ctx, principalUUID, "dipole-gateway"), TransactionId: transactionID, StateSha256: stateSHA256,
	})
	if err != nil {
		return nil, mapAgentOAuthAuthorizationTransactionRPCError(err)
	}
	result, err := agentOAuthAuthorizationTransactionConsumeResultFromProto(response)
	if err != nil || result.TransactionID != transactionID {
		return nil, ErrAgentOAuthAuthorizationTransactionUnavailable
	}
	return result, nil
}

func agentOAuthAuthorizationTransactionConsumeResultFromProto(response *agentv1.ConsumeOAuthAuthorizationTransactionResponse) (*AgentOAuthAuthorizationTransactionConsumeResult, error) {
	if response == nil || !validOAuthTransactionID(response.GetTransactionId()) || !validOAuthURL(response.GetIssuer()) || !validOAuthURL(response.GetRedirectUri()) ||
		!validOAuthSealedCodeVerifier(response.GetSealedCodeVerifier()) || response.GetExpiresAtUnixMs() <= time.Now().UTC().UnixMilli() {
		return nil, ErrAgentOAuthAuthorizationTransactionUnavailable
	}
	return &AgentOAuthAuthorizationTransactionConsumeResult{
		TransactionID: response.GetTransactionId(), Issuer: response.GetIssuer(), RedirectURI: response.GetRedirectUri(),
		SealedCodeVerifier: response.GetSealedCodeVerifier(), ExpiresAt: time.UnixMilli(response.GetExpiresAtUnixMs()).UTC(),
	}, nil
}

func validOAuthTransactionID(value string) bool {
	return len(value) >= 16 && len(value) <= 64 && validOAuthBase64URL(value)
}

func validOAuthURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

func validOAuthSealedCodeVerifier(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) != 4 || parts[0] != "v1" || len(value) > 1024 {
		return false
	}
	for _, part := range parts[1:] {
		if part == "" || !validOAuthBase64URL(part) {
			return false
		}
	}
	return true
}

func validOAuthBase64URL(value string) bool {
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' || char == '-' {
			continue
		}
		return false
	}
	return value != ""
}

func mapAgentOAuthAuthorizationTransactionRPCError(err error) error {
	switch status.Code(err) {
	case codes.InvalidArgument, codes.FailedPrecondition:
		return ErrAgentOAuthAuthorizationTransactionInvalid
	case codes.Unauthenticated, codes.PermissionDenied, codes.NotFound:
		return ErrAgentOAuthAuthorizationTransactionDenied
	default:
		return ErrAgentOAuthAuthorizationTransactionUnavailable
	}
}
