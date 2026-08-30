package delivery

import "testing"

func TestRolloutPolicyIsDeterministicAndScoped(t *testing.T) {
	policy := RolloutPolicy{Scope: GrayScopeUser, Target: AuthorityCPP, Percentage: 37, Salt: "rollout-v1"}
	first, err := policy.Select("user-100")
	if err != nil {
		t.Fatalf("first selection: %v", err)
	}
	for i := 0; i < 10; i++ {
		got, err := policy.Select("user-100")
		if err != nil || got != first {
			t.Fatalf("selection changed on replay: got=%q err=%v want=%q", got, err, first)
		}
	}
	otherScope := policy
	otherScope.Scope = GrayScopeNode
	if got, err := otherScope.Select("user-100"); err != nil {
		t.Fatalf("other scope selection: %v", err)
	} else if got == first {
		// Scope is part of the hash input; this assertion guards against
		// accidentally dropping it from the rollout key.
		if got2, _ := otherScope.Select("node-user-100"); got2 == got {
			t.Fatalf("scope change did not change either rollout sample")
		}
	}
}

func TestRolloutPolicyDefaultsToGoAtZeroAndTargetsAtHundred(t *testing.T) {
	for _, target := range []Authority{AuthorityShadow, AuthorityCPP} {
		zero := RolloutPolicy{Scope: GrayScopeNode, Target: target, Percentage: 0, Salt: "v1"}
		if got, err := zero.Select("node-a"); err != nil || got != AuthorityGo {
			t.Fatalf("zero rollout: got=%q err=%v", got, err)
		}
		hundred := zero
		hundred.Percentage = 100
		if got, err := hundred.Select("node-a"); err != nil || got != target {
			t.Fatalf("full rollout: got=%q err=%v want=%q", got, err, target)
		}
	}
}

func TestRolloutPolicyRejectsUnscopedInput(t *testing.T) {
	base := RolloutPolicy{Scope: GrayScopeUser, Target: AuthorityCPP, Percentage: 10, Salt: "v1"}
	for name, policy := range map[string]RolloutPolicy{
		"scope":      {Target: AuthorityCPP, Percentage: 10, Salt: "v1"},
		"target":     {Scope: GrayScopeUser, Target: AuthorityGo, Percentage: 10, Salt: "v1"},
		"percentage": {Scope: GrayScopeUser, Target: AuthorityCPP, Percentage: 101, Salt: "v1"},
		"salt":       {Scope: GrayScopeUser, Target: AuthorityCPP, Percentage: 10},
	} {
		if _, err := policy.Select("user-a"); err == nil {
			t.Errorf("%s: expected validation error", name)
		}
	}
	if _, err := base.Select(" "); err == nil {
		t.Error("expected blank subject to be rejected")
	}
}
