package application

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrAgentOAuthAuthorizationTransactionInvalid = errors.New("agent OAuth authorization transaction is invalid")

// AgentOAuthAuthorizationTransactionV1 holds only sealed verifier material.
// The state digest and owner binding are used by a later callback service to
// execute a single conditional consume before asking the Runtime to decrypt.
type AgentOAuthAuthorizationTransactionV1 struct {
	TransactionUUID, OwnerUserUUID   string
	Issuer, RedirectURI              string
	StateSHA256, SealedCodeVerifier  string
	ExpiresAt, CreatedAt, ConsumedAt time.Time
}

type AgentOAuthAuthorizationTransactionStoreV1 interface {
	CreateAgentOAuthAuthorizationTransaction(context.Context, AgentOAuthAuthorizationTransactionV1) (bool, error)
	GetAgentOAuthAuthorizationTransaction(context.Context, string) (*AgentOAuthAuthorizationTransactionV1, error)
	ConsumeAgentOAuthAuthorizationTransaction(context.Context, string, string, string, time.Time) (bool, error)
}

func (v AgentOAuthAuthorizationTransactionV1) Validate() error {
	if !validAgentOAuthTransactionIdentifier(v.TransactionUUID) || !validAgentOAuthIdentifier(v.OwnerUserUUID, 64) ||
		!validAgentOAuthURL(v.Issuer) || !validAgentOAuthURL(v.RedirectURI) || !validSHA256V1(v.StateSHA256) ||
		!validAgentOAuthSealedVerifier(v.SealedCodeVerifier) || v.ExpiresAt.IsZero() ||
		(!v.CreatedAt.IsZero() && !v.ExpiresAt.After(v.CreatedAt)) ||
		(!v.ConsumedAt.IsZero() && (!v.ConsumedAt.Before(v.ExpiresAt) || v.ConsumedAt.Before(v.CreatedAt))) {
		return ErrAgentOAuthAuthorizationTransactionInvalid
	}
	return nil
}

func validAgentOAuthIdentifier(value string, limit int) bool {
	return value != "" && value == strings.TrimSpace(value) && len(value) <= limit
}

func validAgentOAuthTransactionIdentifier(value string) bool {
	return len(value) >= 16 && validAgentOAuthIdentifier(value, 64) &&
		strings.Trim(value, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-") == ""
}

func validAgentOAuthURL(value string) bool {
	return strings.HasPrefix(value, "https://") && value == strings.TrimSpace(value) && len(value) <= 2048 &&
		!strings.ContainsAny(value, "?#@")
}

func validAgentOAuthSealedVerifier(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) != 4 || parts[0] != "v1" || len(value) > 1024 {
		return false
	}
	for _, part := range parts[1:] {
		if part == "" || strings.Trim(part, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-") != "" {
			return false
		}
	}
	return true
}
