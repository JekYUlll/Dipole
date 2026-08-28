package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

type persistentAgentMCPToolInvocationTerminalServiceV1 struct {
	rounds      application.AgentMCPToolRoundStoreV1
	invocations application.AgentToolInvocationReaderV1
	audits      application.AgentToolInvocationAuditServiceV1
	now         func() time.Time
}

var _ application.AgentMCPToolInvocationTerminalServiceV1 = (*persistentAgentMCPToolInvocationTerminalServiceV1)(nil)

func newPersistentAgentMCPToolInvocationTerminalServiceV1(
	rounds application.AgentMCPToolRoundStoreV1,
	invocations application.AgentToolInvocationReaderV1,
	audits application.AgentToolInvocationAuditServiceV1,
	now func() time.Time,
) (application.AgentMCPToolInvocationTerminalServiceV1, error) {
	if rounds == nil || invocations == nil || audits == nil || now == nil {
		return nil, errors.New("persistent Agent MCP Tool terminal dependencies are required")
	}
	return &persistentAgentMCPToolInvocationTerminalServiceV1{
		rounds: rounds, invocations: invocations, audits: audits, now: now,
	}, nil
}

func NewPersistentAgentMCPToolInvocationTerminalServiceV1(
	rounds application.AgentMCPToolRoundStoreV1,
	invocations application.AgentToolInvocationReaderV1,
	audits application.AgentToolInvocationAuditServiceV1,
) (application.AgentMCPToolInvocationTerminalServiceV1, error) {
	return newPersistentAgentMCPToolInvocationTerminalServiceV1(rounds, invocations, audits, time.Now)
}

func (s *persistentAgentMCPToolInvocationTerminalServiceV1) FinishFromRound(
	ctx context.Context,
	request application.AgentMCPToolInvocationTerminalRequestV1,
) (*application.AgentToolInvocationV1, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	invocation, err := s.invocations.GetToolInvocation(ctx, request.InvocationUUID)
	if err != nil {
		return nil, fmt.Errorf("load Agent MCP Tool invocation terminal: %w", err)
	}
	if !trustedReadMCPInvocationTerminalV1(invocation, request) {
		return nil, application.ErrAgentMCPToolRoundDenied
	}
	round, err := s.rounds.GetMCPToolRound(ctx, request.RoundUUID)
	if err != nil {
		return nil, fmt.Errorf("load Agent MCP Tool terminal round: %w", err)
	}
	if !trustedMCPToolTerminalRoundV1(round, request) {
		return nil, application.ErrAgentMCPToolRoundDenied
	}
	if invocation.Status != application.AgentToolInvocationStatusRunning {
		if !sameInvocationAndRoundTerminalV1(invocation, round) {
			return nil, application.ErrAgentMCPToolRoundConflict
		}
		return invocation, nil
	}

	finish := application.AgentToolInvocationFinishV1{
		InvocationUUID: invocation.InvocationUUID,
		TaskUUID:       invocation.TaskUUID,
		RunUUID:        invocation.RunUUID,
		LatencyMS:      elapsedMillisecondsV1(invocation.StartedAt, s.now().UTC()),
	}
	switch round.Status {
	case application.AgentMCPToolRoundStatusCompleted:
		finish.Status = application.AgentToolInvocationStatusCompleted
		finish.ResultSHA256 = round.ResultSHA256
		finish.ResultBytes = uint64(len(round.ResultJSON))
	case application.AgentMCPToolRoundStatusFailed:
		finish.Status = application.AgentToolInvocationStatusFailed
		finish.ErrorCode = round.ErrorCode
	default:
		return nil, application.ErrAgentMCPToolRoundDenied
	}
	if err := s.audits.Finish(ctx, finish); err != nil {
		return nil, err
	}
	terminal, err := s.invocations.GetToolInvocation(ctx, request.InvocationUUID)
	if err != nil {
		return nil, fmt.Errorf("reload Agent MCP Tool invocation terminal: %w", err)
	}
	if !sameInvocationAndRoundTerminalV1(terminal, round) {
		return nil, application.ErrAgentMCPToolRoundConflict
	}
	return terminal, nil
}

func trustedReadMCPInvocationTerminalV1(invocation *application.AgentToolInvocationV1, request application.AgentMCPToolInvocationTerminalRequestV1) bool {
	if invocation == nil || invocation.InvocationUUID != request.InvocationUUID || invocation.TaskUUID != request.TaskUUID || invocation.RunUUID != request.RunUUID ||
		invocation.Transport != application.AgentToolTransportMCP || invocation.StartedAt.IsZero() ||
		application.ValidateAgentMCPToolCommandV1(invocation.ProfileID, invocation.ServerID, invocation.ArgumentsJSON, invocation.ArgumentsSHA256) != nil {
		return false
	}
	descriptor, ok := application.AgentCapabilityDescriptorByIDV1(invocation.CapabilityID)
	if !ok || descriptor.Risk != application.AgentCapabilityRiskRead {
		return false
	}
	return invocation.Status == application.AgentToolInvocationStatusRunning ||
		invocation.Status == application.AgentToolInvocationStatusCompleted || invocation.Status == application.AgentToolInvocationStatusFailed
}

func trustedMCPToolTerminalRoundV1(round *application.AgentMCPToolRoundV1, request application.AgentMCPToolInvocationTerminalRequestV1) bool {
	if round == nil || round.RoundUUID != request.RoundUUID || round.InvocationUUID != request.InvocationUUID ||
		round.TaskUUID != request.TaskUUID || round.RunUUID != request.RunUUID {
		return false
	}
	finish := application.AgentMCPToolRoundFinishV1{
		RoundUUID: round.RoundUUID, OwnerTokenSHA256: round.OwnerTokenSHA256, Status: round.Status,
		ResultJSON: round.ResultJSON, ResultSHA256: round.ResultSHA256, ErrorCode: round.ErrorCode,
	}
	if finish.Validate() != nil {
		return false
	}
	if round.Status == application.AgentMCPToolRoundStatusCompleted {
		var result map[string]any
		if json.Unmarshal([]byte(round.ResultJSON), &result) != nil || result["resultType"] == "input_required" {
			return false
		}
	}
	return true
}

func sameInvocationAndRoundTerminalV1(invocation *application.AgentToolInvocationV1, round *application.AgentMCPToolRoundV1) bool {
	if invocation == nil || round == nil || invocation.ActionReference != nil {
		return false
	}
	switch round.Status {
	case application.AgentMCPToolRoundStatusCompleted:
		return invocation.Status == application.AgentToolInvocationStatusCompleted && invocation.ResultSHA256 == round.ResultSHA256 &&
			invocation.ResultBytes == uint64(len(round.ResultJSON)) && invocation.ErrorCode == ""
	case application.AgentMCPToolRoundStatusFailed:
		return invocation.Status == application.AgentToolInvocationStatusFailed && invocation.ResultSHA256 == "" &&
			invocation.ResultBytes == 0 && invocation.ErrorCode == round.ErrorCode
	default:
		return false
	}
}

func elapsedMillisecondsV1(startedAt, finishedAt time.Time) uint64 {
	if startedAt.IsZero() || !finishedAt.After(startedAt) {
		return 0
	}
	return uint64(finishedAt.Sub(startedAt) / time.Millisecond)
}
