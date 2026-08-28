package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
)

type AgentMemoryRepository struct{ queries generated.Querier }

var _ application.AgentMemoryStoreV1 = (*AgentMemoryRepository)(nil)
var _ application.AgentMemoryOwnerStoreV1 = (*AgentMemoryRepository)(nil)

func NewAgentMemoryRepository(queries generated.Querier) (*AgentMemoryRepository, error) {
	if queries == nil {
		return nil, errors.New("Agent Memory queries are required")
	}
	return &AgentMemoryRepository{queries: queries}, nil
}

func (r *AgentMemoryRepository) CreateMemory(ctx context.Context, memory application.AgentMemoryV1) error {
	if err := memory.Validate(); err != nil || memory.Status != application.AgentMemoryStatusActive || memory.RevokedAt != nil {
		return fmt.Errorf("validate Agent Memory: %w", application.ErrAgentMemoryInvalid)
	}
	if err := r.queries.InsertAgentMemory(ctx, generated.InsertAgentMemoryParams{
		MemoryUuid: memory.MemoryUUID, TenantID: memory.TenantID, PrincipalUuid: memory.PrincipalUUID, AgentUuid: memory.AgentUUID,
		MemoryType: string(memory.MemoryType), Status: string(memory.Status), ResourceType: memory.ResourceType, ResourceID: memory.ResourceID,
		Content: memory.Content, CompactContent: nullableString(memory.CompactContent), Priority: memory.Priority,
		SourceType: memory.Provenance.SourceType, SourceID: memory.Provenance.SourceID,
		SourceUri: nullableString(memory.Provenance.URI), SourceSequence: nullableString(memory.Provenance.Sequence),
		ValidFrom: memory.ValidFrom, ExpiresAt: nullableTime(memory.ExpiresAt), RevokedAt: nullableTime(memory.RevokedAt),
	}); err != nil {
		return fmt.Errorf("create Agent Memory: %w", err)
	}
	return nil
}

func (r *AgentMemoryRepository) ListContextMemories(ctx context.Context, query application.AgentMemoryQueryV1) ([]application.AgentMemoryV1, error) {
	if strings.TrimSpace(query.TenantID) == "" || strings.TrimSpace(query.PrincipalUUID) == "" || strings.TrimSpace(query.AgentUUID) == "" ||
		strings.TrimSpace(query.ResourceType) == "" || strings.TrimSpace(query.ResourceID) == "" || query.CreatedBefore.IsZero() || query.At.IsZero() || query.Limit < 1 || query.Limit > 100 {
		return nil, application.ErrAgentMemoryInvalid
	}
	rows, err := r.queries.ListAgentContextMemories(ctx, generated.ListAgentContextMemoriesParams{
		TenantID: query.TenantID, PrincipalUuid: query.PrincipalUUID, AgentUuid: query.AgentUUID,
		ResourceType: query.ResourceType, ResourceID: query.ResourceID, CreatedAt: query.CreatedBefore,
		ValidFrom: query.At, ExpiresAt: nullableTime(&query.At), Limit: int32(query.Limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list Agent Context Memories: %w", err)
	}
	items := make([]application.AgentMemoryV1, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapAgentMemory(row))
	}
	return items, nil
}

func (r *AgentMemoryRepository) RevokeMemory(ctx context.Context, memoryUUID string, revokedAt time.Time) error {
	if strings.TrimSpace(memoryUUID) == "" || revokedAt.IsZero() {
		return application.ErrAgentMemoryInvalid
	}
	rows, err := r.queries.RevokeAgentMemory(ctx, generated.RevokeAgentMemoryParams{RevokedAt: sql.NullTime{Time: revokedAt, Valid: true}, MemoryUuid: strings.TrimSpace(memoryUUID)})
	if err != nil {
		return fmt.Errorf("revoke Agent Memory: %w", err)
	}
	if rows == 0 {
		return application.ErrAgentMemoryInvalid
	}
	return nil
}

func (r *AgentMemoryRepository) ListOwnedMemories(ctx context.Context, request application.AgentMemoryOwnerListRequestV1) ([]application.AgentMemoryV1, error) {
	if strings.TrimSpace(request.TenantID) == "" || strings.TrimSpace(request.PrincipalUUID) == "" || request.AfterCreatedAt.IsZero() ||
		request.Limit < 1 || request.Limit > 101 || (request.AfterUUID != "" && len([]rune(request.AfterUUID)) > 64) {
		return nil, application.ErrAgentMemoryInvalid
	}
	rows, err := r.queries.ListOwnedAgentMemories(ctx, generated.ListOwnedAgentMemoriesParams{
		TenantID: request.TenantID, PrincipalUuid: request.PrincipalUUID, AfterCreatedAt: request.AfterCreatedAt,
		AfterMemoryUuid: request.AfterUUID, Limit: int32(request.Limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list owned Agent Memories: %w", err)
	}
	items := make([]application.AgentMemoryV1, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapAgentMemory(row))
	}
	return items, nil
}

func (r *AgentMemoryRepository) GetOwnedMemory(ctx context.Context, tenantID, principalUUID, memoryUUID string) (*application.AgentMemoryV1, error) {
	if strings.TrimSpace(tenantID) == "" || strings.TrimSpace(principalUUID) == "" || strings.TrimSpace(memoryUUID) == "" {
		return nil, application.ErrAgentMemoryInvalid
	}
	row, err := r.queries.GetOwnedAgentMemory(ctx, generated.GetOwnedAgentMemoryParams{TenantID: tenantID, PrincipalUuid: principalUUID, MemoryUuid: memoryUUID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get owned Agent Memory: %w", err)
	}
	item := mapAgentMemory(row)
	return &item, nil
}

func (r *AgentMemoryRepository) RevokeOwnedMemory(ctx context.Context, tenantID, principalUUID, memoryUUID, revokedByUUID, reason string, revokedAt time.Time) error {
	if strings.TrimSpace(tenantID) == "" || strings.TrimSpace(principalUUID) == "" || strings.TrimSpace(memoryUUID) == "" ||
		strings.TrimSpace(revokedByUUID) == "" || strings.TrimSpace(reason) == "" || revokedAt.IsZero() {
		return application.ErrAgentMemoryInvalid
	}
	rows, err := r.queries.RevokeOwnedAgentMemory(ctx, generated.RevokeOwnedAgentMemoryParams{
		RevokedAt: sql.NullTime{Time: revokedAt, Valid: true}, RevokedByUuid: revokedByUUID, RevokeReason: reason,
		TenantID: tenantID, PrincipalUuid: principalUUID, MemoryUuid: memoryUUID,
	})
	if err != nil {
		return fmt.Errorf("revoke owned Agent Memory: %w", err)
	}
	if rows == 0 {
		return application.ErrAgentMemoryConflict
	}
	return nil
}

func mapAgentMemory(row generated.AgentMemory) application.AgentMemoryV1 {
	return application.AgentMemoryV1{
		MemoryUUID: row.MemoryUuid, TenantID: row.TenantID, PrincipalUUID: row.PrincipalUuid, AgentUUID: row.AgentUuid,
		MemoryType: application.AgentMemoryTypeV1(row.MemoryType), Status: application.AgentMemoryStatusV1(row.Status),
		ResourceType: row.ResourceType, ResourceID: row.ResourceID, Content: row.Content, CompactContent: row.CompactContent.String,
		Priority: row.Priority, Provenance: application.AgentMemoryProvenanceV1{
			SourceType: row.SourceType, SourceID: row.SourceID, URI: row.SourceUri.String, Sequence: row.SourceSequence.String,
		}, ValidFrom: row.ValidFrom, ExpiresAt: timePointer(row.ExpiresAt), RevokedAt: timePointer(row.RevokedAt),
		RevokedByUUID: row.RevokedByUuid, RevokeReason: row.RevokeReason, CreatedAt: row.CreatedAt,
	}
}
