package application

import (
	"strings"
	"testing"
	"time"
)

func TestAgentRuntimePromotionGrantValidation(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)
	grant := validRuntimePromotionGrantV1(now)
	if err := grant.Validate(); err != nil || !grant.Active(now) {
		t.Fatalf("valid promotion grant: active=%v err=%v", grant.Active(now), err)
	}
	for name, mutate := range map[string]func(*AgentRuntimePromotionGrantV1){
		"policy":        func(g *AgentRuntimePromotionGrantV1) { g.PolicyVersion = "v1" },
		"evidence":      func(g *AgentRuntimePromotionGrantV1) { g.EvidenceSHA256 = "invalid" },
		"suite":         func(g *AgentRuntimePromotionGrantV1) { g.EvalSuiteSHA256 = strings.Repeat("A", 64) },
		"single signer": func(g *AgentRuntimePromotionGrantV1) { g.ReviewedByUUID = g.GrantedByUUID },
		"window":        func(g *AgentRuntimePromotionGrantV1) { g.ExpiresAt = g.ValidFrom },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := grant
			mutate(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Fatal("invalid promotion grant passed validation")
			}
		})
	}
	revokedAt := now
	grant.RevokedAt = &revokedAt
	if grant.Active(now) {
		t.Fatal("revoked promotion grant is active")
	}
}

func validRuntimePromotionGrantV1(now time.Time) AgentRuntimePromotionGrantV1 {
	return AgentRuntimePromotionGrantV1{
		GrantUUID: "PROMOTE-1", TenantID: "dipole", RuntimeID: "dipole-agent", CandidateVersion: "runtime-v7",
		DefinitionUUID: "DEF-UAI", DefinitionVersion: 7, PolicyVersion: AgentRuntimePromotionPolicyVersionV2,
		EvidenceSHA256: strings.Repeat("a", 64), EvalSuiteSHA256: strings.Repeat("b", 64),
		GrantedByUUID: "OPERATOR-1", ReviewedByUUID: "OPERATOR-2", ValidFrom: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour),
	}
}
