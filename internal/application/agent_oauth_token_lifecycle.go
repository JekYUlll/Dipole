package application

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrAgentOAuthTokenLifecycleInvalid = errors.New("agent OAuth token lifecycle is invalid")

type AgentOAuthTokenLifecycleStateV1 string

const (
	AgentOAuthTokenLifecycleActiveV1    AgentOAuthTokenLifecycleStateV1 = "active"
	AgentOAuthTokenLifecycleRefreshedV1 AgentOAuthTokenLifecycleStateV1 = "refreshed"
	AgentOAuthTokenLifecycleRevokedV1   AgentOAuthTokenLifecycleStateV1 = "revoked"
	AgentOAuthTokenLifecycleExpiredV1   AgentOAuthTokenLifecycleStateV1 = "expired"
)

// AgentOAuthTokenLifecycleWriteRequestV1 carries only Runtime-key encrypted
// token material. Core persists the transition but never receives plaintext.
type AgentOAuthTokenLifecycleWriteRequestV1 struct {
	HandoffUUID, LeaseOwner              string
	State                                AgentOAuthTokenLifecycleStateV1
	SealedTokenBundle, TokenBundleSHA256 string
	AccessTokenExpiresAt                 time.Time
	Scope, RevocationReason              string
}

// AgentOAuthTokenLifecycleV1 is the durable Core record. It intentionally has
// no plaintext token field; Runtime owns decrypting its envelope.
type AgentOAuthTokenLifecycleV1 struct {
	HandoffUUID, RuntimeKeyID            string
	State                                AgentOAuthTokenLifecycleStateV1
	SealedTokenBundle, TokenBundleSHA256 string
	AccessTokenExpiresAt                 time.Time
	Scope, RevocationReason              string
	RefreshCount                         uint32
}

func (v AgentOAuthTokenLifecycleWriteRequestV1) Validate() error {
	if !validAgentOAuthTransactionIdentifier(v.HandoffUUID) || !validAgentOAuthIdentifier(v.LeaseOwner, 128) ||
		strings.TrimSpace(v.LeaseOwner) != v.LeaseOwner {
		return ErrAgentOAuthTokenLifecycleInvalid
	}
	if len(v.Scope) > 2048 || strings.TrimSpace(v.Scope) != v.Scope || strings.ContainsAny(v.Scope, "\r\n\x00") || len(v.RevocationReason) > 512 || strings.TrimSpace(v.RevocationReason) != v.RevocationReason || strings.ContainsAny(v.RevocationReason, "\r\n\x00") {
		return ErrAgentOAuthTokenLifecycleInvalid
	}
	switch v.State {
	case AgentOAuthTokenLifecycleActiveV1, AgentOAuthTokenLifecycleRefreshedV1:
		if !validAgentOAuthSealedTokenBundle(v.SealedTokenBundle) || !validSHA256V1(v.TokenBundleSHA256) || v.AccessTokenExpiresAt.IsZero() || v.RevocationReason != "" {
			return ErrAgentOAuthTokenLifecycleInvalid
		}
	case AgentOAuthTokenLifecycleRevokedV1:
		if v.SealedTokenBundle != "" || v.TokenBundleSHA256 != "" || !v.AccessTokenExpiresAt.IsZero() || v.RevocationReason == "" {
			return ErrAgentOAuthTokenLifecycleInvalid
		}
	case AgentOAuthTokenLifecycleExpiredV1:
		if v.SealedTokenBundle != "" || v.TokenBundleSHA256 != "" || !v.AccessTokenExpiresAt.IsZero() || v.RevocationReason != "" {
			return ErrAgentOAuthTokenLifecycleInvalid
		}
	default:
		return ErrAgentOAuthTokenLifecycleInvalid
	}
	return nil
}

func validAgentOAuthSealedTokenBundle(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) != 5 || parts[0] != "v1" || len(value) > 16384 {
		return false
	}
	for _, part := range parts[1:] {
		if part == "" || strings.Trim(part, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-") != "" {
			return false
		}
	}
	return true
}

// AgentOAuthTokenLifecycleStoreV1 is Core-owned. Implementations must bind a
// write to a currently claimed handoff and its Runtime lease atomically.
type AgentOAuthTokenLifecycleStoreV1 interface {
	PersistAgentOAuthTokenLifecycle(context.Context, AgentOAuthTokenLifecycleWriteRequestV1, time.Time) (bool, error)
}
