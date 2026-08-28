package application

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

func TestAgentMCPToolRoundContracts(t *testing.T) {
	claim := AgentMCPToolRoundClaimV1{
		RoundUUID: strings.Repeat("a", 64), InvocationUUID: "INV-1", TaskUUID: "TASK-1", RunUUID: "RUN-1",
		RoundNumber: 0, RequestSHA256: strings.Repeat("b", 64), OwnerTokenSHA256: strings.Repeat("c", 64),
	}
	if err := claim.Validate(); err != nil {
		t.Fatalf("valid claim: %v", err)
	}
	for name, mutate := range map[string]func(*AgentMCPToolRoundClaimV1){
		"bad round id":     func(v *AgentMCPToolRoundClaimV1) { v.RoundUUID = "short" },
		"drifting task id": func(v *AgentMCPToolRoundClaimV1) { v.TaskUUID = " TASK-1" },
		"bad request hash": func(v *AgentMCPToolRoundClaimV1) { v.RequestSHA256 = strings.Repeat("g", 64) },
		"too many rounds":  func(v *AgentMCPToolRoundClaimV1) { v.RoundNumber = 2 },
	} {
		candidate := claim
		mutate(&candidate)
		if candidate.Validate() == nil {
			t.Fatalf("%s should fail", name)
		}
	}

	resultJSON := `{"content":[{"text":"created","type":"text"}]}`
	completed := AgentMCPToolRoundFinishV1{
		RoundUUID: claim.RoundUUID, OwnerTokenSHA256: claim.OwnerTokenSHA256, Status: AgentMCPToolRoundStatusCompleted,
		ResultJSON:   resultJSON,
		ResultSHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(resultJSON))),
	}
	if err := completed.Validate(); err != nil {
		t.Fatalf("valid completion: %v", err)
	}
	failed := AgentMCPToolRoundFinishV1{
		RoundUUID: claim.RoundUUID, OwnerTokenSHA256: claim.OwnerTokenSHA256,
		Status: AgentMCPToolRoundStatusFailed, ErrorCode: "transport_unavailable",
	}
	if err := failed.Validate(); err != nil {
		t.Fatalf("valid failure: %v", err)
	}
	completed.ResultJSON = strings.Repeat("x", 128*1024+1)
	if completed.Validate() == nil {
		t.Fatal("oversized result should fail")
	}
	completed.ResultJSON = `[]`
	completed.ResultSHA256 = fmt.Sprintf("%x", sha256.Sum256([]byte(completed.ResultJSON)))
	if completed.Validate() == nil {
		t.Fatal("non-object result should fail")
	}
}

func TestAgentMCPToolRoundClaimOutcomePreventsUnsafeRetry(t *testing.T) {
	for _, outcome := range []AgentMCPToolRoundClaimOutcomeV1{
		AgentMCPToolRoundClaimed, AgentMCPToolRoundReplayCompleted, AgentMCPToolRoundReplayFailed, AgentMCPToolRoundAmbiguous,
	} {
		if !outcome.Valid() {
			t.Fatalf("valid outcome rejected: %s", outcome)
		}
	}
	if AgentMCPToolRoundClaimOutcomeV1("retry").Valid() {
		t.Fatal("unsafe retry outcome must not exist")
	}
}
