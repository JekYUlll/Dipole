package application

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrAgentOAuthCallbackHandoffInvalid = errors.New("agent OAuth callback handoff is invalid")

type AgentOAuthCallbackHandoffStatusV1 string

const (
	AgentOAuthCallbackHandoffRecordedV1  AgentOAuthCallbackHandoffStatusV1 = "callback_recorded"
	AgentOAuthCallbackHandoffClaimedV1   AgentOAuthCallbackHandoffStatusV1 = "exchange_claimed"
	AgentOAuthCallbackHandoffExchangedV1 AgentOAuthCallbackHandoffStatusV1 = "exchanged"
)

// AgentOAuthCallbackHandoffV1 is durable recovery metadata for a callback.
// SealedAuthorizationCode is opaque to Core and must only be opened by the
// Runtime key boundary identified by RuntimeKeyID.
type AgentOAuthCallbackHandoffV1 struct {
	HandoffUUID, TransactionUUID, OwnerUserUUID string
	Issuer, RedirectURI                         string
	AuthorizationCodeSHA256                     string
	SealedAuthorizationCode, RuntimeKeyID       string
	Status                                      AgentOAuthCallbackHandoffStatusV1
	LeaseOwner                                  string
	ExpiresAt, LeaseExpiresAt, CompletedAt      time.Time
	Attempts                                    uint32
}

type AgentOAuthCallbackHandoffStoreV1 interface {
	CreateAgentOAuthCallbackHandoff(context.Context, AgentOAuthCallbackHandoffV1) (bool, error)
	GetAgentOAuthCallbackHandoff(context.Context, string) (*AgentOAuthCallbackHandoffV1, error)
	ClaimAgentOAuthCallbackHandoff(context.Context, string, string, time.Time, time.Time) (bool, error)
	CompleteAgentOAuthCallbackHandoff(context.Context, string, string, time.Time) (bool, error)
	ReleaseAgentOAuthCallbackHandoff(context.Context, string, string, time.Time) (bool, error)
}

func (v AgentOAuthCallbackHandoffV1) Validate() error {
	if !validAgentOAuthTransactionIdentifier(v.HandoffUUID) || !validAgentOAuthTransactionIdentifier(v.TransactionUUID) ||
		!validAgentOAuthIdentifier(v.OwnerUserUUID, 64) || !validAgentOAuthURL(v.Issuer) || !validAgentOAuthURL(v.RedirectURI) ||
		!validSHA256V1(v.AuthorizationCodeSHA256) || !validAgentOAuthSealedVerifier(v.SealedAuthorizationCode) ||
		!validAgentOAuthIdentifier(v.RuntimeKeyID, 128) || v.ExpiresAt.IsZero() {
		return ErrAgentOAuthCallbackHandoffInvalid
	}
	switch v.Status {
	case AgentOAuthCallbackHandoffRecordedV1:
		if v.LeaseOwner != "" || !v.LeaseExpiresAt.IsZero() || !v.CompletedAt.IsZero() {
			return ErrAgentOAuthCallbackHandoffInvalid
		}
	case AgentOAuthCallbackHandoffClaimedV1:
		if !validAgentOAuthIdentifier(v.LeaseOwner, 128) || v.LeaseExpiresAt.IsZero() || !v.LeaseExpiresAt.After(time.Time{}) || !v.CompletedAt.IsZero() {
			return ErrAgentOAuthCallbackHandoffInvalid
		}
	case AgentOAuthCallbackHandoffExchangedV1:
		if v.LeaseOwner != "" || !v.LeaseExpiresAt.IsZero() || v.CompletedAt.IsZero() || v.CompletedAt.After(v.ExpiresAt) {
			return ErrAgentOAuthCallbackHandoffInvalid
		}
	default:
		return ErrAgentOAuthCallbackHandoffInvalid
	}
	return nil
}

func validAgentOAuthCallbackHandoffLease(handoffUUID, leaseOwner string, now, leaseExpiresAt time.Time) bool {
	return validAgentOAuthTransactionIdentifier(handoffUUID) && validAgentOAuthIdentifier(leaseOwner, 128) &&
		!now.IsZero() && !leaseExpiresAt.IsZero() && leaseExpiresAt.After(now)
}

func validAgentOAuthCallbackHandoffCompletion(handoffUUID, leaseOwner string, now time.Time) bool {
	return validAgentOAuthTransactionIdentifier(handoffUUID) && validAgentOAuthIdentifier(leaseOwner, 128) && !now.IsZero()
}

func validAgentOAuthCallbackHandoffRelease(handoffUUID, leaseOwner string) bool {
	return validAgentOAuthTransactionIdentifier(handoffUUID) && validAgentOAuthIdentifier(leaseOwner, 128) &&
		strings.TrimSpace(leaseOwner) == leaseOwner
}
