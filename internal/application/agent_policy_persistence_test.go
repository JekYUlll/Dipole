package application

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestAgentApprovalV1AuthorizesExactUnconsumedBinding(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	scope := AgentResourceScopeV1{ResourceType: "conversation", ResourceID: "G1", Actions: []string{"write"}}
	scopeHash, err := AgentResourceScopeSHA256V1(scope)
	if err != nil {
		t.Fatalf("hash scope: %v", err)
	}
	argumentsHash, nonceHash := strings.Repeat("b", 64), strings.Repeat("c", 64)
	approval := AgentApprovalV1{
		ApprovalUUID:    "APR-1",
		TaskUUID:        "TASK-1",
		CapabilityID:    "message.bulk.send",
		ResourceScope:   scope,
		ScopeSHA256:     scopeHash,
		ArgumentsSHA256: argumentsHash,
		NonceSHA256:     nonceHash,
		Status:          AgentApprovalStatusApproved,
		ApprovedByUUID:  "U100",
		ExpiresAt:       now.Add(time.Hour),
	}
	claim := AgentApprovalClaimV1{
		TaskUUID: "TASK-1", CapabilityID: "message.bulk.send",
		ScopeSHA256: scopeHash, ArgumentsSHA256: argumentsHash, NonceSHA256: nonceHash,
	}
	if err := approval.Authorize(claim, now); err != nil {
		t.Fatalf("authorize exact approval binding: %v", err)
	}
}

func TestAgentApprovalV1RejectsReplayAndBindingDrift(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	scope := AgentResourceScopeV1{ResourceType: "conversation", ResourceID: "G1", Actions: []string{"write"}}
	scopeHash, err := AgentResourceScopeSHA256V1(scope)
	if err != nil {
		t.Fatalf("hash scope: %v", err)
	}
	argumentsHash, nonceHash := strings.Repeat("b", 64), strings.Repeat("c", 64)
	base := AgentApprovalV1{
		ApprovalUUID: "APR-1", TaskUUID: "TASK-1", CapabilityID: "message.bulk.send",
		ResourceScope: scope,
		ScopeSHA256:   scopeHash, ArgumentsSHA256: argumentsHash, NonceSHA256: nonceHash,
		Status: AgentApprovalStatusApproved, ApprovedByUUID: "U100", ExpiresAt: now.Add(time.Hour),
	}
	claim := AgentApprovalClaimV1{
		TaskUUID: "TASK-1", CapabilityID: "message.bulk.send",
		ScopeSHA256: scopeHash, ArgumentsSHA256: argumentsHash, NonceSHA256: nonceHash,
	}

	tests := []struct {
		name     string
		approval AgentApprovalV1
		claim    AgentApprovalClaimV1
		at       time.Time
	}{
		{name: "task drift", approval: base, claim: replaceApprovalClaim(claim, "TASK-2", "", "", "", ""), at: now},
		{name: "capability drift", approval: base, claim: replaceApprovalClaim(claim, "", "message.delete", "", "", ""), at: now},
		{name: "scope drift", approval: base, claim: replaceApprovalClaim(claim, "", "", strings.Repeat("d", 64), "", ""), at: now},
		{name: "arguments drift", approval: base, claim: replaceApprovalClaim(claim, "", "", "", strings.Repeat("e", 64), ""), at: now},
		{name: "nonce drift", approval: base, claim: replaceApprovalClaim(claim, "", "", "", "", strings.Repeat("f", 64)), at: now},
		{name: "expired", approval: base, claim: claim, at: now.Add(time.Hour)},
		{name: "revoked", approval: withApprovalRevoked(base, now), claim: claim, at: now},
		{name: "consumed", approval: withApprovalConsumed(base, now), claim: claim, at: now},
		{name: "pending", approval: withApprovalStatus(base, AgentApprovalStatusPending), claim: claim, at: now},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if err := test.approval.Authorize(test.claim, test.at); !errors.Is(err, ErrAgentApprovalDenied) {
				t.Fatalf("expected approval denial, got %v", err)
			}
		})
	}
}

func TestAgentResourceScopeSHA256V1IsOrderIndependent(t *testing.T) {
	t.Parallel()
	left, err := AgentResourceScopeSHA256V1(AgentResourceScopeV1{ResourceType: " conversation ", ResourceID: " G1 ", Actions: []string{"write", " read "}})
	if err != nil {
		t.Fatalf("hash left scope: %v", err)
	}
	right, err := AgentResourceScopeSHA256V1(AgentResourceScopeV1{ResourceType: "conversation", ResourceID: "G1", Actions: []string{"read", "write"}})
	if err != nil {
		t.Fatalf("hash right scope: %v", err)
	}
	if left != right {
		t.Fatalf("scope action order changed hash: left=%s right=%s", left, right)
	}
	const want = "86df65a26fd77dcbc9f22c9ba232506ed1147f7fd3ceead451952d7f26b36530"
	if left != want {
		t.Fatalf("scope golden hash = %s, want %s", left, want)
	}
}

func TestAgentTaskV1TransitionsAreExplicit(t *testing.T) {
	t.Parallel()
	for _, transition := range [][2]AgentTaskStatusV1{
		{AgentTaskStatusCreated, AgentTaskStatusRunning},
		{AgentTaskStatusRunning, AgentTaskStatusWaitingApproval},
		{AgentTaskStatusWaitingApproval, AgentTaskStatusRunning},
		{AgentTaskStatusRunning, AgentTaskStatusCompleted},
	} {
		if err := ValidateAgentTaskTransitionV1(transition[0], transition[1]); err != nil {
			t.Fatalf("valid transition %s -> %s: %v", transition[0], transition[1], err)
		}
	}
	for _, transition := range [][2]AgentTaskStatusV1{
		{AgentTaskStatusCreated, AgentTaskStatusCompleted},
		{AgentTaskStatusWaitingApproval, AgentTaskStatusCompleted},
		{AgentTaskStatusCompleted, AgentTaskStatusRunning},
	} {
		if err := ValidateAgentTaskTransitionV1(transition[0], transition[1]); !errors.Is(err, ErrAgentPolicyInvalid) {
			t.Fatalf("invalid transition %s -> %s returned %v", transition[0], transition[1], err)
		}
	}
}

func replaceApprovalClaim(claim AgentApprovalClaimV1, task, capability, scope, arguments, nonce string) AgentApprovalClaimV1 {
	if task != "" {
		claim.TaskUUID = task
	}
	if capability != "" {
		claim.CapabilityID = capability
	}
	if scope != "" {
		claim.ScopeSHA256 = scope
	}
	if arguments != "" {
		claim.ArgumentsSHA256 = arguments
	}
	if nonce != "" {
		claim.NonceSHA256 = nonce
	}
	return claim
}

func withApprovalRevoked(approval AgentApprovalV1, at time.Time) AgentApprovalV1 {
	approval.RevokedAt = &at
	return approval
}

func withApprovalConsumed(approval AgentApprovalV1, at time.Time) AgentApprovalV1 {
	approval.ConsumedAt = &at
	approval.Status = AgentApprovalStatusConsumed
	return approval
}

func withApprovalStatus(approval AgentApprovalV1, status AgentApprovalStatusV1) AgentApprovalV1 {
	approval.Status = status
	return approval
}
