package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/application"
)

type persistentAgentMCPToolRoundServiceV1 struct {
	store       application.AgentMCPToolRoundStoreV1
	invocations application.AgentToolInvocationReaderV1
}

var _ application.AgentMCPToolRoundServiceV1 = (*persistentAgentMCPToolRoundServiceV1)(nil)

func NewPersistentAgentMCPToolRoundServiceV1(store application.AgentMCPToolRoundStoreV1, invocations application.AgentToolInvocationReaderV1) (application.AgentMCPToolRoundServiceV1, error) {
	if store == nil || invocations == nil {
		return nil, errors.New("persistent Agent MCP Tool round dependencies are required")
	}
	return &persistentAgentMCPToolRoundServiceV1{store: store, invocations: invocations}, nil
}

func (s *persistentAgentMCPToolRoundServiceV1) Claim(ctx context.Context, claim application.AgentMCPToolRoundClaimV1) (*application.AgentMCPToolRoundClaimResultV1, error) {
	if err := claim.Validate(); err != nil {
		return nil, err
	}
	invocation, err := s.invocations.GetToolInvocation(ctx, claim.InvocationUUID)
	if err != nil {
		return nil, fmt.Errorf("load Agent MCP Tool invocation: %w", err)
	}
	if invocation == nil || invocation.TaskUUID != claim.TaskUUID || invocation.RunUUID != claim.RunUUID ||
		invocation.Status != application.AgentToolInvocationStatusRunning || invocation.ProfileID == "" ||
		application.ValidateAgentMCPToolCommandV1(invocation.ProfileID, invocation.ServerID, invocation.ArgumentsJSON, invocation.ArgumentsSHA256) != nil {
		return nil, application.ErrAgentMCPToolRoundDenied
	}
	created, err := s.store.ClaimMCPToolRound(ctx, claim)
	if err != nil {
		return nil, fmt.Errorf("claim Agent MCP Tool round: %w", err)
	}
	if created {
		return &application.AgentMCPToolRoundClaimResultV1{Outcome: application.AgentMCPToolRoundClaimed}, nil
	}
	existing, err := s.store.GetMCPToolRound(ctx, claim.RoundUUID)
	if err != nil {
		return nil, fmt.Errorf("load Agent MCP Tool round: %w", err)
	}
	if existing == nil || !sameAgentMCPToolRoundClaimV1(existing.AgentMCPToolRoundClaimV1, claim) {
		return nil, application.ErrAgentMCPToolRoundConflict
	}
	switch existing.Status {
	case application.AgentMCPToolRoundStatusCompleted:
		return &application.AgentMCPToolRoundClaimResultV1{
			Outcome: application.AgentMCPToolRoundReplayCompleted, ResultJSON: existing.ResultJSON, ResultSHA256: existing.ResultSHA256,
		}, nil
	case application.AgentMCPToolRoundStatusFailed:
		return &application.AgentMCPToolRoundClaimResultV1{Outcome: application.AgentMCPToolRoundReplayFailed, ErrorCode: existing.ErrorCode}, nil
	case application.AgentMCPToolRoundStatusExecuting:
		return &application.AgentMCPToolRoundClaimResultV1{Outcome: application.AgentMCPToolRoundAmbiguous}, nil
	default:
		return nil, application.ErrAgentMCPToolRoundConflict
	}
}

func (s *persistentAgentMCPToolRoundServiceV1) Finish(ctx context.Context, finish application.AgentMCPToolRoundFinishV1) error {
	if err := finish.Validate(); err != nil {
		return err
	}
	changed, err := s.store.FinishMCPToolRound(ctx, finish)
	if err != nil {
		return fmt.Errorf("finish Agent MCP Tool round: %w", err)
	}
	if !changed {
		return application.ErrAgentMCPToolRoundConflict
	}
	return nil
}

func sameAgentMCPToolRoundClaimV1(left, right application.AgentMCPToolRoundClaimV1) bool {
	return left.RoundUUID == right.RoundUUID && left.InvocationUUID == right.InvocationUUID && left.TaskUUID == right.TaskUUID &&
		left.RunUUID == right.RunUUID && left.RoundNumber == right.RoundNumber && left.RequestSHA256 == right.RequestSHA256
}
