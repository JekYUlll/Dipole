package agentapplication

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/JekYUlll/Dipole/internal/application"
)

type agentMCPToolRoundStoreStub struct {
	created  bool
	existing *application.AgentMCPToolRoundV1
	finished bool
	claims   int
}

type agentToolAuditStoreStub struct {
	invocation *application.AgentToolInvocationV1
}

func (s *agentToolAuditStoreStub) GetToolInvocation(context.Context, string) (*application.AgentToolInvocationV1, error) {
	return s.invocation, nil
}

func (s *agentMCPToolRoundStoreStub) ClaimMCPToolRound(context.Context, application.AgentMCPToolRoundClaimV1) (bool, error) {
	s.claims++
	return s.created, nil
}

func (s *agentMCPToolRoundStoreStub) GetMCPToolRound(context.Context, string) (*application.AgentMCPToolRoundV1, error) {
	return s.existing, nil
}

func TestPersistentAgentMCPToolRoundTerminalInvocationOnlyReplaysExistingReceipt(t *testing.T) {
	claim := testAgentMCPToolRoundClaim()
	invocation := testExternalAgentToolInvocation()
	invocation.Status = application.AgentToolInvocationStatusCompleted
	store := &agentMCPToolRoundStoreStub{existing: testAgentMCPToolRound(claim, application.AgentMCPToolRoundStatusCompleted)}
	service, _ := NewPersistentAgentMCPToolRoundServiceV1(store, &agentToolAuditStoreStub{invocation: invocation})
	result, err := service.Claim(context.Background(), claim)
	if err != nil || result.Outcome != application.AgentMCPToolRoundReplayCompleted || store.claims != 0 {
		t.Fatalf("terminal replay=%+v claims=%d err=%v", result, store.claims, err)
	}

	store.existing = nil
	if _, err := service.Claim(context.Background(), claim); !errors.Is(err, application.ErrAgentMCPToolRoundDenied) || store.claims != 0 {
		t.Fatalf("missing terminal receipt error=%v claims=%d", err, store.claims)
	}
}

func (s *agentMCPToolRoundStoreStub) FinishMCPToolRound(context.Context, application.AgentMCPToolRoundFinishV1) (bool, error) {
	return s.finished, nil
}

func TestPersistentAgentMCPToolRoundClaimOutcomes(t *testing.T) {
	claim := testAgentMCPToolRoundClaim()
	invocation := testExternalAgentToolInvocation()
	for name, fixture := range map[string]struct {
		created  bool
		existing *application.AgentMCPToolRoundV1
		want     application.AgentMCPToolRoundClaimOutcomeV1
	}{
		"new":       {created: true, want: application.AgentMCPToolRoundClaimed},
		"completed": {existing: testAgentMCPToolRound(claim, application.AgentMCPToolRoundStatusCompleted), want: application.AgentMCPToolRoundReplayCompleted},
		"failed":    {existing: testAgentMCPToolRound(claim, application.AgentMCPToolRoundStatusFailed), want: application.AgentMCPToolRoundReplayFailed},
		"executing": {existing: testAgentMCPToolRound(claim, application.AgentMCPToolRoundStatusExecuting), want: application.AgentMCPToolRoundAmbiguous},
	} {
		t.Run(name, func(t *testing.T) {
			store := &agentMCPToolRoundStoreStub{created: fixture.created, existing: fixture.existing}
			service, _ := NewPersistentAgentMCPToolRoundServiceV1(store, &agentToolAuditStoreStub{invocation: invocation})
			result, err := service.Claim(context.Background(), claim)
			if err != nil || result.Outcome != fixture.want {
				t.Fatalf("claim outcome=%+v err=%v", result, err)
			}
		})
	}
}

func TestPersistentAgentMCPToolRoundRejectsUnsafeClaimAndFinish(t *testing.T) {
	claim := testAgentMCPToolRoundClaim()
	store := &agentMCPToolRoundStoreStub{existing: testAgentMCPToolRound(claim, application.AgentMCPToolRoundStatusExecuting)}
	store.existing.RequestSHA256 = strings.Repeat("d", 64)
	service, _ := NewPersistentAgentMCPToolRoundServiceV1(store, &agentToolAuditStoreStub{invocation: testExternalAgentToolInvocation()})
	if _, err := service.Claim(context.Background(), claim); !errors.Is(err, application.ErrAgentMCPToolRoundConflict) {
		t.Fatalf("request drift error=%v", err)
	}
	resultJSON := `{"content":[]}`
	finish := application.AgentMCPToolRoundFinishV1{
		RoundUUID: claim.RoundUUID, OwnerTokenSHA256: claim.OwnerTokenSHA256, Status: application.AgentMCPToolRoundStatusCompleted,
		ResultJSON: resultJSON, ResultSHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(resultJSON))),
	}
	if err := service.Finish(context.Background(), finish); !errors.Is(err, application.ErrAgentMCPToolRoundConflict) {
		t.Fatalf("non-owner or terminal finish error=%v", err)
	}
}

func testAgentMCPToolRoundClaim() application.AgentMCPToolRoundClaimV1 {
	return application.AgentMCPToolRoundClaimV1{
		RoundUUID: strings.Repeat("a", 64), InvocationUUID: "INV-EXT-1", TaskUUID: "TASK-1", RunUUID: "RUN-1",
		RoundNumber: 0, RequestSHA256: strings.Repeat("b", 64), OwnerTokenSHA256: strings.Repeat("c", 64),
	}
}

func testExternalAgentToolInvocation() *application.AgentToolInvocationV1 {
	arguments := `{"calendarId":"CAL-1"}`
	return &application.AgentToolInvocationV1{
		InvocationUUID: "INV-EXT-1", TaskUUID: "TASK-1", RunUUID: "RUN-1", Status: application.AgentToolInvocationStatusRunning,
		ProfileID: "calendar-prod", ServerID: "calendar.example", ArgumentsJSON: arguments,
		ArgumentsSHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(arguments))),
	}
}

func testAgentMCPToolRound(claim application.AgentMCPToolRoundClaimV1, status application.AgentMCPToolRoundStatusV1) *application.AgentMCPToolRoundV1 {
	round := &application.AgentMCPToolRoundV1{AgentMCPToolRoundClaimV1: claim, Status: status}
	if status == application.AgentMCPToolRoundStatusCompleted {
		round.ResultJSON = `{"content":[]}`
		round.ResultSHA256 = fmt.Sprintf("%x", sha256.Sum256([]byte(round.ResultJSON)))
	}
	if status == application.AgentMCPToolRoundStatusFailed {
		round.ErrorCode = "transport_unavailable"
	}
	return round
}
