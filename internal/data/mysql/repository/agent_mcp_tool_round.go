package repository

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
)

type AgentMCPToolRoundRepository struct{ queries generated.Querier }

var _ application.AgentMCPToolRoundStoreV1 = (*AgentMCPToolRoundRepository)(nil)

func NewAgentMCPToolRoundRepository(queries generated.Querier) (*AgentMCPToolRoundRepository, error) {
	if queries == nil {
		return nil, errors.New("Agent MCP Tool round queries are required")
	}
	return &AgentMCPToolRoundRepository{queries: queries}, nil
}

func (r *AgentMCPToolRoundRepository) ClaimMCPToolRound(ctx context.Context, claim application.AgentMCPToolRoundClaimV1) (bool, error) {
	if err := claim.Validate(); err != nil {
		return false, err
	}
	rows, err := r.queries.InsertAgentMCPToolRound(ctx, generated.InsertAgentMCPToolRoundParams{
		RoundUuid: claim.RoundUUID, InvocationUuid: claim.InvocationUUID, TaskUuid: claim.TaskUUID, RunUuid: claim.RunUUID,
		RoundNumber: claim.RoundNumber, RequestSha256: claim.RequestSHA256, OwnerTokenSha256: claim.OwnerTokenSHA256,
	})
	if err != nil {
		return false, fmt.Errorf("insert Agent MCP Tool round: %w", err)
	}
	return rows == 1, nil
}

func (r *AgentMCPToolRoundRepository) GetMCPToolRound(ctx context.Context, roundUUID string) (*application.AgentMCPToolRoundV1, error) {
	row, err := r.queries.GetAgentMCPToolRound(ctx, roundUUID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get Agent MCP Tool round: %w", err)
	}
	resultJSON := row.ResultJson
	if len(resultJSON) > 0 {
		compacted := make([]byte, 0, len(resultJSON))
		buffer := bytes.NewBuffer(compacted)
		if err := json.Compact(buffer, resultJSON); err != nil {
			return nil, fmt.Errorf("compact Agent MCP Tool round result: %w", err)
		}
		resultJSON = buffer.Bytes()
	}
	return &application.AgentMCPToolRoundV1{
		AgentMCPToolRoundClaimV1: application.AgentMCPToolRoundClaimV1{
			RoundUUID: row.RoundUuid, InvocationUUID: row.InvocationUuid, TaskUUID: row.TaskUuid, RunUUID: row.RunUuid,
			RoundNumber: row.RoundNumber, RequestSHA256: row.RequestSha256, OwnerTokenSHA256: row.OwnerTokenSha256,
		},
		Status: application.AgentMCPToolRoundStatusV1(row.Status), ResultJSON: string(resultJSON),
		ResultSHA256: row.ResultSha256.String, ErrorCode: row.ErrorCode.String,
	}, nil
}

func (r *AgentMCPToolRoundRepository) FinishMCPToolRound(ctx context.Context, finish application.AgentMCPToolRoundFinishV1) (bool, error) {
	if err := finish.Validate(); err != nil {
		return false, err
	}
	var resultJSON json.RawMessage
	if finish.ResultJSON != "" {
		resultJSON = json.RawMessage(finish.ResultJSON)
	}
	rows, err := r.queries.FinishAgentMCPToolRound(ctx, generated.FinishAgentMCPToolRoundParams{
		Status: string(finish.Status), ResultJson: resultJSON, ResultSha256: nullableString(finish.ResultSHA256),
		ResultBytes: nullableUint64(uint64(len(finish.ResultJSON)), finish.Status == application.AgentMCPToolRoundStatusCompleted),
		ErrorCode:   nullableString(finish.ErrorCode), RoundUuid: finish.RoundUUID, OwnerTokenSha256: finish.OwnerTokenSHA256,
	})
	if err != nil {
		return false, fmt.Errorf("finish Agent MCP Tool round: %w", err)
	}
	return rows == 1, nil
}
