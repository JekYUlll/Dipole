package application

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestAgentOAuthCallbackHandoffValidation(t *testing.T) {
	now := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	valid := AgentOAuthCallbackHandoffV1{
		HandoffUUID: strings.Repeat("a", 22), TransactionUUID: strings.Repeat("b", 22), OwnerUserUUID: "U100",
		Issuer: "https://auth.example.com/tenant", RedirectURI: "https://dipole.example.com/oauth/callback",
		AuthorizationCodeSHA256: strings.Repeat("c", 64), SealedAuthorizationCode: "v1.abc.def.ghi", RuntimeKeyID: "oauth-runtime-2026-08",
		Status: AgentOAuthCallbackHandoffRecordedV1, ExpiresAt: now.Add(10 * time.Minute),
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("validate recorded: %v", err)
	}
	claimed := valid
	claimed.Status, claimed.LeaseOwner, claimed.LeaseExpiresAt = AgentOAuthCallbackHandoffClaimedV1, "agent-runtime-1", now.Add(time.Minute)
	if err := claimed.Validate(); err != nil {
		t.Fatalf("validate claimed: %v", err)
	}
	exchanged := valid
	exchanged.Status, exchanged.CompletedAt = AgentOAuthCallbackHandoffExchangedV1, now.Add(time.Minute)
	if err := exchanged.Validate(); err != nil {
		t.Fatalf("validate exchanged: %v", err)
	}
	for name, mutate := range map[string]func(*AgentOAuthCallbackHandoffV1){
		"plain-looking code": func(v *AgentOAuthCallbackHandoffV1) { v.SealedAuthorizationCode = "authorization-code" },
		"recorded lease":     func(v *AgentOAuthCallbackHandoffV1) { v.LeaseOwner = "agent-runtime-1" },
		"unknown status":     func(v *AgentOAuthCallbackHandoffV1) { v.Status = "retryable_failure" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			mutate(&candidate)
			if !errors.Is(candidate.Validate(), ErrAgentOAuthCallbackHandoffInvalid) {
				t.Fatal("expected invalid handoff")
			}
		})
	}
}
