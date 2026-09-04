package application

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestAgentOAuthTokenLifecycleWriteRequestValidation(t *testing.T) {
	t.Parallel()
	expiresAt := time.Now().UTC().Add(time.Hour)
	valid := AgentOAuthTokenLifecycleWriteRequestV1{
		HandoffUUID: "handoff_123456789", LeaseOwner: "runtime-1", State: AgentOAuthTokenLifecycleActiveV1,
		SealedTokenBundle: "v1.nonce.ciphertext.tag.wrapped", TokenBundleSHA256: strings.Repeat("a", 64), AccessTokenExpiresAt: expiresAt,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("active request rejected: %v", err)
	}
	if err := (AgentOAuthTokenLifecycleWriteRequestV1{HandoffUUID: valid.HandoffUUID, LeaseOwner: valid.LeaseOwner, State: AgentOAuthTokenLifecycleRevokedV1, RevocationReason: "invalid_grant"}).Validate(); err != nil {
		t.Fatalf("revoked request rejected: %v", err)
	}
	if err := (AgentOAuthTokenLifecycleWriteRequestV1{HandoffUUID: valid.HandoffUUID, LeaseOwner: valid.LeaseOwner, State: AgentOAuthTokenLifecycleExpiredV1}).Validate(); err != nil {
		t.Fatalf("expired request rejected: %v", err)
	}

	for _, invalid := range []AgentOAuthTokenLifecycleWriteRequestV1{
		{HandoffUUID: "short", LeaseOwner: valid.LeaseOwner, State: valid.State, SealedTokenBundle: valid.SealedTokenBundle, TokenBundleSHA256: valid.TokenBundleSHA256, AccessTokenExpiresAt: expiresAt},
		{HandoffUUID: valid.HandoffUUID, LeaseOwner: " runtime", State: valid.State, SealedTokenBundle: valid.SealedTokenBundle, TokenBundleSHA256: valid.TokenBundleSHA256, AccessTokenExpiresAt: expiresAt},
		{HandoffUUID: valid.HandoffUUID, LeaseOwner: valid.LeaseOwner, State: valid.State, SealedTokenBundle: "plaintext", TokenBundleSHA256: valid.TokenBundleSHA256, AccessTokenExpiresAt: expiresAt},
		{HandoffUUID: valid.HandoffUUID, LeaseOwner: valid.LeaseOwner, State: AgentOAuthTokenLifecycleRevokedV1, SealedTokenBundle: valid.SealedTokenBundle, RevocationReason: "invalid_grant"},
		{HandoffUUID: valid.HandoffUUID, LeaseOwner: valid.LeaseOwner, State: AgentOAuthTokenLifecycleExpiredV1, RevocationReason: "expired"},
	} {
		if err := invalid.Validate(); !errors.Is(err, ErrAgentOAuthTokenLifecycleInvalid) {
			t.Fatalf("expected invalid lifecycle request, got %v", err)
		}
	}
}
