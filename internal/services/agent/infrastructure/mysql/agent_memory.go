package agentmysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	mysqlData "github.com/JekYUlll/Dipole/internal/platform/mysql"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
)

type AgentMemoryRepository struct {
	queries generated.Querier
	store   transactionStore
}

var _ application.AgentMemoryStoreV1 = (*AgentMemoryRepository)(nil)
var _ application.AgentMemoryOwnerStoreV1 = (*AgentMemoryRepository)(nil)

func NewAgentMemoryRepository(queries generated.Querier) (*AgentMemoryRepository, error) {
	if queries == nil {
		return nil, errors.New("Agent Memory queries are required")
	}
	return &AgentMemoryRepository{queries: queries}, nil
}

func NewAgentMemoryRepositoryWithTransactions(store transactionStore) (*AgentMemoryRepository, error) {
	if store == nil || store.Queries() == nil {
		return nil, errors.New("Agent Memory transaction store is required")
	}
	return &AgentMemoryRepository{queries: store.Queries(), store: store}, nil
}

func (r *AgentMemoryRepository) CreateMemory(ctx context.Context, memory application.AgentMemoryV1) error {
	memory = application.CanonicalAgentMemoryLineageV1(memory)
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
		MemoryRootUuid: memory.MemoryRootUUID, MemoryVersion: memory.MemoryVersion, SupersedesMemoryUuid: nullableString(memory.SupersedesMemoryUUID),
		CorrectedByUuid: memory.CorrectedByUUID, CorrectionReason: memory.CorrectionReason,
	}); err != nil {
		return fmt.Errorf("create Agent Memory: %w", err)
	}
	return nil
}

func (r *AgentMemoryRepository) CorrectOwnedMemory(ctx context.Context, write application.AgentMemoryOwnerCorrectionWriteV1) (*application.AgentMemoryOwnerCorrectionResultV1, error) {
	if r.store == nil || strings.TrimSpace(write.TenantID) == "" || strings.TrimSpace(write.PrincipalUUID) == "" ||
		strings.TrimSpace(write.SourceMemoryUUID) == "" || write.ExpectedVersion == 0 || write.CorrectedAt.IsZero() || write.Corrected.Validate() != nil {
		return nil, application.ErrAgentMemoryInvalid
	}
	var result *application.AgentMemoryOwnerCorrectionResultV1
	err := r.store.WithinTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted}, func(queries *generated.Queries) error {
		row, err := queries.GetOwnedAgentMemoryForUpdate(ctx, generated.GetOwnedAgentMemoryForUpdateParams{
			TenantID: write.TenantID, PrincipalUuid: write.PrincipalUUID, MemoryUuid: write.SourceMemoryUUID,
		})
		if errors.Is(err, sql.ErrNoRows) {
			return application.ErrAgentMemoryDenied
		}
		if err != nil {
			return fmt.Errorf("lock owned Agent Memory: %w", err)
		}
		previous := mapAgentMemory(row)
		if previous.MemoryVersion != write.ExpectedVersion {
			return application.ErrAgentMemoryConflict
		}
		if previous.Status == application.AgentMemoryStatusActive {
			if previous.ExpiresAt != nil && !previous.ExpiresAt.After(write.CorrectedAt) {
				return application.ErrAgentMemoryConflict
			}
			rows, updateErr := queries.SupersedeOwnedAgentMemory(ctx, generated.SupersedeOwnedAgentMemoryParams{
				RevokedAt: sql.NullTime{Time: write.CorrectedAt, Valid: true}, RevokedByUuid: write.PrincipalUUID,
				RevokeReason: "superseded by " + write.Corrected.MemoryUUID, TenantID: write.TenantID,
				PrincipalUuid: write.PrincipalUUID, MemoryUuid: write.SourceMemoryUUID, MemoryVersion: write.ExpectedVersion,
			})
			if updateErr != nil {
				return fmt.Errorf("supersede owned Agent Memory: %w", updateErr)
			}
			if rows != 1 {
				return application.ErrAgentMemoryConflict
			}
			if insertErr := insertAgentMemoryV1(ctx, queries, write.Corrected); insertErr != nil {
				if mysqlData.IsDuplicateKey(insertErr) {
					return application.ErrAgentMemoryConflict
				}
				return fmt.Errorf("insert corrected Agent Memory: %w", insertErr)
			}
		} else if previous.Status != application.AgentMemoryStatusRevoked {
			return application.ErrAgentMemoryConflict
		}
		successorRow, err := queries.GetAgentMemoryBySupersedes(ctx, generated.GetAgentMemoryBySupersedesParams{
			TenantID: write.TenantID, PrincipalUuid: write.PrincipalUUID, SupersedesMemoryUuid: nullableString(write.SourceMemoryUUID),
		})
		if errors.Is(err, sql.ErrNoRows) {
			return application.ErrAgentMemoryConflict
		}
		if err != nil {
			return fmt.Errorf("get corrected Agent Memory: %w", err)
		}
		previousRow, err := queries.GetOwnedAgentMemory(ctx, generated.GetOwnedAgentMemoryParams{
			TenantID: write.TenantID, PrincipalUuid: write.PrincipalUUID, MemoryUuid: write.SourceMemoryUUID,
		})
		if err != nil {
			return fmt.Errorf("get superseded Agent Memory: %w", err)
		}
		storedPrevious, storedCorrected := mapAgentMemory(previousRow), mapAgentMemory(successorRow)
		if !exactStoredAgentMemoryCorrectionV1(storedPrevious, storedCorrected, write) {
			return application.ErrAgentMemoryConflict
		}
		result = &application.AgentMemoryOwnerCorrectionResultV1{Previous: storedPrevious, Corrected: storedCorrected}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *AgentMemoryRepository) EraseOwnedMemoryRoot(ctx context.Context, tenantID, principalUUID, memoryUUID, erasedByUUID string, reason application.AgentMemoryErasureReasonV1, erasedAt time.Time) (*application.AgentMemoryOwnerErasureReceiptV1, error) {
	if r.store == nil || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(principalUUID) == "" || strings.TrimSpace(memoryUUID) == "" ||
		strings.TrimSpace(erasedByUUID) == "" || reason != application.AgentMemoryErasureReasonOwnerRequest || erasedAt.IsZero() {
		return nil, application.ErrAgentMemoryInvalid
	}
	var receipt *application.AgentMemoryOwnerErasureReceiptV1
	err := r.store.WithinTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted}, func(queries *generated.Queries) error {
		selected, err := queries.GetOwnedAgentMemory(ctx, generated.GetOwnedAgentMemoryParams{TenantID: tenantID, PrincipalUuid: principalUUID, MemoryUuid: memoryUUID})
		if errors.Is(err, sql.ErrNoRows) {
			return application.ErrAgentMemoryDenied
		}
		if err != nil {
			return fmt.Errorf("resolve owned Agent Memory for erasure: %w", err)
		}
		rows, err := queries.ListOwnedAgentMemoryRootForUpdate(ctx, generated.ListOwnedAgentMemoryRootForUpdateParams{TenantID: tenantID, PrincipalUuid: principalUUID, MemoryRootUuid: selected.MemoryRootUuid})
		if err != nil {
			return fmt.Errorf("lock Agent Memory root for erasure: %w", err)
		}
		if err := validateAgentMemoryRootForErasure(rows, selected.MemoryRootUuid); err != nil {
			return err
		}
		if rows[0].ContentErasedAt.Valid {
			receipt = erasureReceipt(rows)
			if receipt == nil || receipt.ErasedByUUID != erasedByUUID || receipt.Reason != reason {
				return application.ErrAgentMemoryConflict
			}
			return nil
		}
		affected, err := queries.EraseOwnedAgentMemoryRoot(ctx, generated.EraseOwnedAgentMemoryRootParams{
			ContentErasedAt: sql.NullTime{Time: erasedAt, Valid: true}, ContentErasedByUuid: erasedByUUID,
			ContentErasureReasonCode: string(reason), TenantID: tenantID, PrincipalUuid: principalUUID, MemoryRootUuid: selected.MemoryRootUuid,
		})
		if err != nil {
			return fmt.Errorf("erase owned Agent Memory root: %w", err)
		}
		if affected != int64(len(rows)) {
			return application.ErrAgentMemoryConflict
		}
		stored, err := queries.ListOwnedAgentMemoryRootForUpdate(ctx, generated.ListOwnedAgentMemoryRootForUpdateParams{TenantID: tenantID, PrincipalUuid: principalUUID, MemoryRootUuid: selected.MemoryRootUuid})
		if err != nil {
			return fmt.Errorf("read erased Agent Memory root: %w", err)
		}
		receipt = erasureReceipt(stored)
		if receipt == nil || !receipt.ErasedAt.Equal(erasedAt) || receipt.ErasedByUUID != erasedByUUID || receipt.Reason != reason {
			return application.ErrAgentMemoryConflict
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return receipt, nil
}

func validateAgentMemoryRootForErasure(rows []generated.AgentMemory, root string) error {
	if len(rows) == 0 {
		return application.ErrAgentMemoryConflict
	}
	active := 0
	for i, row := range rows {
		if row.MemoryRootUuid != root || row.MemoryVersion != uint32(i+1) {
			return application.ErrAgentMemoryConflict
		}
		if row.Status == string(application.AgentMemoryStatusActive) {
			active++
		}
		if row.ContentErasedAt.Valid != rows[0].ContentErasedAt.Valid {
			return application.ErrAgentMemoryConflict
		}
	}
	if !rows[0].ContentErasedAt.Valid && active != 1 {
		return application.ErrAgentMemoryConflict
	}
	return nil
}

func erasureReceipt(rows []generated.AgentMemory) *application.AgentMemoryOwnerErasureReceiptV1 {
	if len(rows) == 0 || !rows[0].ContentErasedAt.Valid {
		return nil
	}
	first := rows[0]
	for _, row := range rows {
		if !row.ContentErasedAt.Valid || !row.ContentErasedAt.Time.Equal(first.ContentErasedAt.Time) || row.ContentErasedByUuid != first.ContentErasedByUuid ||
			row.ContentErasureReasonCode != first.ContentErasureReasonCode || row.Content != application.AgentMemoryErasedContentV1 || row.CompactContent.Valid || row.SourceUri.Valid ||
			row.ResourceType != application.AgentMemorySourceErasedV1 || row.ResourceID != application.AgentMemoryErasedReferenceV1 {
			return nil
		}
		if row.MemoryVersion == 1 && (row.SourceType != application.AgentMemorySourceErasedV1 || row.SourceID != application.AgentMemoryErasedReferenceV1 || row.SourceSequence.Valid) {
			return nil
		}
	}
	return &application.AgentMemoryOwnerErasureReceiptV1{MemoryRootUUID: first.MemoryRootUuid, Versions: uint32(len(rows)), ErasedAt: first.ContentErasedAt.Time, ErasedByUUID: first.ContentErasedByUuid, Reason: application.AgentMemoryErasureReasonV1(first.ContentErasureReasonCode)}
}

func insertAgentMemoryV1(ctx context.Context, queries generated.Querier, memory application.AgentMemoryV1) error {
	return queries.InsertAgentMemory(ctx, generated.InsertAgentMemoryParams{
		MemoryUuid: memory.MemoryUUID, TenantID: memory.TenantID, PrincipalUuid: memory.PrincipalUUID, AgentUuid: memory.AgentUUID,
		MemoryType: string(memory.MemoryType), Status: string(memory.Status), ResourceType: memory.ResourceType, ResourceID: memory.ResourceID,
		Content: memory.Content, CompactContent: nullableString(memory.CompactContent), Priority: memory.Priority,
		SourceType: memory.Provenance.SourceType, SourceID: memory.Provenance.SourceID, SourceUri: nullableString(memory.Provenance.URI),
		SourceSequence: nullableString(memory.Provenance.Sequence), ValidFrom: memory.ValidFrom, ExpiresAt: nullableTime(memory.ExpiresAt),
		RevokedAt: nullableTime(memory.RevokedAt), MemoryRootUuid: memory.MemoryRootUUID, MemoryVersion: memory.MemoryVersion,
		SupersedesMemoryUuid: nullableString(memory.SupersedesMemoryUUID), CorrectedByUuid: memory.CorrectedByUUID, CorrectionReason: memory.CorrectionReason,
	})
}

func exactStoredAgentMemoryCorrectionV1(previous, corrected application.AgentMemoryV1, write application.AgentMemoryOwnerCorrectionWriteV1) bool {
	expected := write.Corrected
	return previous.Validate() == nil && corrected.Validate() == nil && previous.MemoryUUID == write.SourceMemoryUUID &&
		previous.MemoryVersion == write.ExpectedVersion && previous.Status == application.AgentMemoryStatusRevoked &&
		previous.RevokedByUUID == write.PrincipalUUID && previous.RevokeReason == "superseded by "+expected.MemoryUUID &&
		corrected.MemoryUUID == expected.MemoryUUID && corrected.TenantID == expected.TenantID && corrected.PrincipalUUID == expected.PrincipalUUID &&
		corrected.AgentUUID == expected.AgentUUID && corrected.MemoryType == expected.MemoryType && corrected.Status == application.AgentMemoryStatusActive &&
		corrected.ResourceType == expected.ResourceType && corrected.ResourceID == expected.ResourceID && corrected.Content == expected.Content &&
		corrected.CompactContent == expected.CompactContent && corrected.Priority == expected.Priority && corrected.MemoryRootUUID == expected.MemoryRootUUID &&
		corrected.MemoryVersion == expected.MemoryVersion && corrected.SupersedesMemoryUUID == expected.SupersedesMemoryUUID &&
		corrected.CorrectedByUUID == expected.CorrectedByUUID && corrected.CorrectionReason == expected.CorrectionReason &&
		corrected.Provenance == expected.Provenance
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
		MemoryRootUUID: row.MemoryRootUuid, MemoryVersion: row.MemoryVersion, SupersedesMemoryUUID: row.SupersedesMemoryUuid.String,
		CorrectedByUUID: row.CorrectedByUuid, CorrectionReason: row.CorrectionReason,
		ContentErasedAt: timePointer(row.ContentErasedAt), ContentErasedByUUID: row.ContentErasedByUuid,
		ContentErasureReason: application.AgentMemoryErasureReasonV1(row.ContentErasureReasonCode),
	}
}
