package agentmysql

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
)

type AgentMCPReadinessEvidenceQueries interface {
	InsertAgentMCPReadinessEvidence(context.Context, generated.InsertAgentMCPReadinessEvidenceParams) (int64, error)
	GetAgentMCPReadinessEvidence(context.Context, generated.GetAgentMCPReadinessEvidenceParams) (generated.AgentMcpReadinessEvidence, error)
	GetFreshAgentMCPReadinessEvidence(context.Context, generated.GetFreshAgentMCPReadinessEvidenceParams) (generated.AgentMcpReadinessEvidence, error)
}

type AgentMCPReadinessEvidenceRepository struct {
	queries AgentMCPReadinessEvidenceQueries
}

var _ application.AgentMCPReadinessEvidenceStoreV1 = (*AgentMCPReadinessEvidenceRepository)(nil)

func NewAgentMCPReadinessEvidenceRepository(queries AgentMCPReadinessEvidenceQueries) (*AgentMCPReadinessEvidenceRepository, error) {
	if queries == nil {
		return nil, errors.New("Agent MCP readiness evidence queries are required")
	}
	return &AgentMCPReadinessEvidenceRepository{queries: queries}, nil
}

func (r *AgentMCPReadinessEvidenceRepository) AppendAgentMCPReadinessEvidence(ctx context.Context, record application.AgentMCPReadinessEvidenceRecordV1) (bool, error) {
	if err := record.Validate(); err != nil {
		return false, err
	}
	rows, err := r.queries.InsertAgentMCPReadinessEvidence(ctx, generated.InsertAgentMCPReadinessEvidenceParams{
		EvidenceUuid: record.EvidenceUUID, SchemaVersion: record.SchemaVersion, TenantID: record.TenantID,
		ProfileBindingSha256: record.ProfileBindingSHA256, RuntimeBindingSha256: record.RuntimeBindingSHA256,
		ContentJson: string(record.ContentJSON), ContentSha256: record.ContentSHA256, OperatorUuid: record.OperatorUUID,
		RequestID: nullableReadinessString(record.RequestID), TraceID: nullableReadinessString(record.TraceID),
		CollectedAt: record.CollectedAt, ExpiresAt: record.ExpiresAt,
	})
	if err != nil {
		return false, fmt.Errorf("append Agent MCP readiness evidence: %w", err)
	}
	if rows > 0 {
		return true, nil
	}
	existing, err := r.GetAgentMCPReadinessEvidence(ctx, record.TenantID, record.EvidenceUUID)
	if err != nil {
		return false, err
	}
	if existing == nil || existing.Validate() != nil || !sameAgentMCPReadinessEvidenceRecordV1(*existing, record) {
		return false, fmt.Errorf("%w: evidence UUID=%s", application.ErrAgentMCPReadinessEvidenceConflict, record.EvidenceUUID)
	}
	return false, nil
}

func (r *AgentMCPReadinessEvidenceRepository) GetAgentMCPReadinessEvidence(ctx context.Context, tenantID, evidenceUUID string) (*application.AgentMCPReadinessEvidenceRecordV1, error) {
	tenantID, evidenceUUID = strings.TrimSpace(tenantID), strings.TrimSpace(evidenceUUID)
	if tenantID == "" || evidenceUUID == "" {
		return nil, application.ErrAgentMCPReadinessEvidenceInvalid
	}
	row, err := r.queries.GetAgentMCPReadinessEvidence(ctx, generated.GetAgentMCPReadinessEvidenceParams{TenantID: tenantID, EvidenceUuid: evidenceUUID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get Agent MCP readiness evidence: %w", err)
	}
	return mapAgentMCPReadinessEvidenceRecordV1(row), nil
}

func (r *AgentMCPReadinessEvidenceRepository) GetFreshAgentMCPReadinessEvidence(ctx context.Context, lookup application.AgentMCPReadinessEvidenceLookupV1) (*application.AgentMCPReadinessEvidenceRecordV1, error) {
	lookup.TenantID = strings.TrimSpace(lookup.TenantID)
	lookup.ProfileBindingSHA256 = strings.TrimSpace(lookup.ProfileBindingSHA256)
	lookup.RuntimeBindingSHA256 = strings.TrimSpace(lookup.RuntimeBindingSHA256)
	if lookup.TenantID == "" || len(lookup.ProfileBindingSHA256) != 64 || len(lookup.RuntimeBindingSHA256) != 64 || lookup.At.IsZero() {
		return nil, application.ErrAgentMCPReadinessEvidenceInvalid
	}
	at := lookup.At.UTC().Truncate(time.Millisecond)
	row, err := r.queries.GetFreshAgentMCPReadinessEvidence(ctx, generated.GetFreshAgentMCPReadinessEvidenceParams{
		TenantID: lookup.TenantID, ProfileBindingSha256: lookup.ProfileBindingSHA256,
		RuntimeBindingSha256: lookup.RuntimeBindingSHA256, ExpiresAt: at, CollectedAt: at,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get fresh Agent MCP readiness evidence: %w", err)
	}
	return mapAgentMCPReadinessEvidenceRecordV1(row), nil
}

func mapAgentMCPReadinessEvidenceRecordV1(row generated.AgentMcpReadinessEvidence) *application.AgentMCPReadinessEvidenceRecordV1 {
	return &application.AgentMCPReadinessEvidenceRecordV1{
		EvidenceUUID: row.EvidenceUuid, SchemaVersion: row.SchemaVersion, TenantID: row.TenantID,
		ProfileBindingSHA256: row.ProfileBindingSha256, RuntimeBindingSHA256: row.RuntimeBindingSha256,
		ContentJSON: json.RawMessage(row.ContentJson), ContentSHA256: row.ContentSha256, OperatorUUID: row.OperatorUuid,
		RequestID: row.RequestID.String, TraceID: row.TraceID.String, Status: string(row.Status),
		CollectedAt: row.CollectedAt, ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt,
	}
}

func sameAgentMCPReadinessEvidenceRecordV1(left, right application.AgentMCPReadinessEvidenceRecordV1) bool {
	return left.EvidenceUUID == right.EvidenceUUID && left.SchemaVersion == right.SchemaVersion && left.TenantID == right.TenantID &&
		left.ProfileBindingSHA256 == right.ProfileBindingSHA256 && left.RuntimeBindingSHA256 == right.RuntimeBindingSHA256 &&
		bytes.Equal(left.ContentJSON, right.ContentJSON) && left.ContentSHA256 == right.ContentSHA256 &&
		left.OperatorUUID == right.OperatorUUID && left.RequestID == right.RequestID && left.TraceID == right.TraceID &&
		left.Status == right.Status && left.CollectedAt.Equal(right.CollectedAt) && left.ExpiresAt.Equal(right.ExpiresAt)
}

func nullableReadinessString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}
