package app

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

type agentMCPToolTerminalAuditStub struct {
	invocation *application.AgentToolInvocationV1
	finishes   []application.AgentToolInvocationFinishV1
}

func (s *agentMCPToolTerminalAuditStub) Begin(context.Context, application.AgentToolInvocationBeginV1) (*application.AgentToolInvocationV1, error) {
	return nil, errors.New("unexpected begin")
}

func (s *agentMCPToolTerminalAuditStub) ResolveCommand(context.Context, string, string, string) (*application.AgentMCPToolCommandV1, error) {
	return nil, errors.New("unexpected resolve")
}

func (s *agentMCPToolTerminalAuditStub) Finish(_ context.Context, finish application.AgentToolInvocationFinishV1) error {
	s.finishes = append(s.finishes, finish)
	s.invocation.Status = finish.Status
	s.invocation.ResultSHA256 = finish.ResultSHA256
	s.invocation.ResultBytes = finish.ResultBytes
	s.invocation.LatencyMS = finish.LatencyMS
	s.invocation.ErrorCode = finish.ErrorCode
	return nil
}

func (s *agentMCPToolTerminalAuditStub) GetToolInvocation(context.Context, string) (*application.AgentToolInvocationV1, error) {
	return s.invocation, nil
}

func TestPersistentAgentMCPToolTerminalDerivesCompletedFinishAndReplays(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 1, 250_000_000, time.UTC)
	claim := testAgentMCPToolRoundClaim()
	round := testAgentMCPToolRound(claim, application.AgentMCPToolRoundStatusCompleted)
	invocation := terminalReadInvocation(now.Add(-1250 * time.Millisecond))
	audit := &agentMCPToolTerminalAuditStub{invocation: invocation}
	service, err := newPersistentAgentMCPToolInvocationTerminalServiceV1(
		&agentMCPToolRoundStoreStub{existing: round}, audit, audit, func() time.Time { return now },
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.FinishFromRound(context.Background(), terminalRequest(claim))
	if err != nil || result.Status != application.AgentToolInvocationStatusCompleted || len(audit.finishes) != 1 {
		t.Fatalf("finish result=%+v finishes=%d err=%v", result, len(audit.finishes), err)
	}
	finish := audit.finishes[0]
	if finish.ResultSHA256 != round.ResultSHA256 || finish.ResultBytes != uint64(len(round.ResultJSON)) || finish.LatencyMS != 1250 {
		t.Fatalf("derived finish=%+v", finish)
	}

	result, err = service.FinishFromRound(context.Background(), terminalRequest(claim))
	if err != nil || result.Status != application.AgentToolInvocationStatusCompleted || len(audit.finishes) != 1 {
		t.Fatalf("replay result=%+v finishes=%d err=%v", result, len(audit.finishes), err)
	}
}

func TestPersistentAgentMCPToolTerminalDerivesFailedFinish(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	claim := testAgentMCPToolRoundClaim()
	round := testAgentMCPToolRound(claim, application.AgentMCPToolRoundStatusFailed)
	invocation := terminalReadInvocation(now)
	audit := &agentMCPToolTerminalAuditStub{invocation: invocation}
	service, _ := newPersistentAgentMCPToolInvocationTerminalServiceV1(
		&agentMCPToolRoundStoreStub{existing: round}, audit, audit, func() time.Time { return now },
	)

	result, err := service.FinishFromRound(context.Background(), terminalRequest(claim))
	if err != nil || result.Status != application.AgentToolInvocationStatusFailed || len(audit.finishes) != 1 || audit.finishes[0].ErrorCode != round.ErrorCode {
		t.Fatalf("failed result=%+v finishes=%+v err=%v", result, audit.finishes, err)
	}
}

func TestPersistentAgentMCPToolTerminalRejectsUntrustedReceiptState(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	claim := testAgentMCPToolRoundClaim()
	for name, mutate := range map[string]func(*application.AgentMCPToolRoundV1, *application.AgentToolInvocationV1){
		"executing": func(round *application.AgentMCPToolRoundV1, _ *application.AgentToolInvocationV1) {
			round.Status = application.AgentMCPToolRoundStatusExecuting
		},
		"binding drift": func(round *application.AgentMCPToolRoundV1, _ *application.AgentToolInvocationV1) {
			round.RunUUID = "RUN-OTHER"
		},
		"input required": func(round *application.AgentMCPToolRoundV1, _ *application.AgentToolInvocationV1) {
			round.ResultJSON = `{"resultType":"input_required"}`
			round.ResultSHA256 = fmt.Sprintf("%x", sha256.Sum256([]byte(round.ResultJSON)))
		},
		"write capability": func(_ *application.AgentMCPToolRoundV1, invocation *application.AgentToolInvocationV1) {
			invocation.CapabilityID = application.AgentCapabilitySystemMessageSend
		},
	} {
		t.Run(name, func(t *testing.T) {
			round := testAgentMCPToolRound(claim, application.AgentMCPToolRoundStatusCompleted)
			invocation := terminalReadInvocation(now)
			mutate(round, invocation)
			audit := &agentMCPToolTerminalAuditStub{invocation: invocation}
			service, _ := newPersistentAgentMCPToolInvocationTerminalServiceV1(
				&agentMCPToolRoundStoreStub{existing: round}, audit, audit, func() time.Time { return now },
			)
			if _, err := service.FinishFromRound(context.Background(), terminalRequest(claim)); !errors.Is(err, application.ErrAgentMCPToolRoundDenied) || len(audit.finishes) != 0 {
				t.Fatalf("error=%v finishes=%d", err, len(audit.finishes))
			}
		})
	}
}

func terminalReadInvocation(startedAt time.Time) *application.AgentToolInvocationV1 {
	invocation := testExternalAgentToolInvocation()
	invocation.CapabilityID = application.AgentCapabilityConversationRead
	invocation.Transport = application.AgentToolTransportMCP
	invocation.StartedAt = startedAt
	return invocation
}

func terminalRequest(claim application.AgentMCPToolRoundClaimV1) application.AgentMCPToolInvocationTerminalRequestV1 {
	return application.AgentMCPToolInvocationTerminalRequestV1{
		TaskUUID: claim.TaskUUID, RunUUID: claim.RunUUID, InvocationUUID: claim.InvocationUUID, RoundUUID: claim.RoundUUID,
	}
}
