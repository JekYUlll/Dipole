package application

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestAgentOAuthAuthorizationTransactionValidation(t *testing.T) {
	now := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	valid := AgentOAuthAuthorizationTransactionV1{
		TransactionUUID: strings.Repeat("a", 22), OwnerUserUUID: "U100", Issuer: "https://auth.example.com/tenant",
		RedirectURI: "https://dipole.example.com/oauth/callback", StateSHA256: strings.Repeat("b", 64),
		SealedCodeVerifier: "v1.abc.def.ghi", CreatedAt: now, ExpiresAt: now.Add(10 * time.Minute),
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	for name, mutate := range map[string]func(*AgentOAuthAuthorizationTransactionV1){
		"state digest":          func(v *AgentOAuthAuthorizationTransactionV1) { v.StateSHA256 = "bad" },
		"issuer query":          func(v *AgentOAuthAuthorizationTransactionV1) { v.Issuer += "?x=1" },
		"sealed verifier":       func(v *AgentOAuthAuthorizationTransactionV1) { v.SealedCodeVerifier = "v1.bad" },
		"expired":               func(v *AgentOAuthAuthorizationTransactionV1) { v.ExpiresAt = now },
		"consumed after expiry": func(v *AgentOAuthAuthorizationTransactionV1) { v.ConsumedAt = v.ExpiresAt },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			mutate(&candidate)
			if !errors.Is(candidate.Validate(), ErrAgentOAuthAuthorizationTransactionInvalid) {
				t.Fatal("expected invalid transaction")
			}
		})
	}
}
