package agentmysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
)

type AgentOAuthCallbackHandoffQueries interface {
	InsertAgentOAuthCallbackHandoff(context.Context, generated.InsertAgentOAuthCallbackHandoffParams) (int64, error)
	GetAgentOAuthCallbackHandoff(context.Context, string) (generated.AgentOauthCallbackHandoff, error)
	ClaimAgentOAuthCallbackHandoff(context.Context, generated.ClaimAgentOAuthCallbackHandoffParams) (int64, error)
	CompleteAgentOAuthCallbackHandoff(context.Context, generated.CompleteAgentOAuthCallbackHandoffParams) (int64, error)
	ReleaseAgentOAuthCallbackHandoff(context.Context, generated.ReleaseAgentOAuthCallbackHandoffParams) (int64, error)
}

type AgentOAuthCallbackHandoffRepository struct {
	queries AgentOAuthCallbackHandoffQueries
}

var _ application.AgentOAuthCallbackHandoffStoreV1 = (*AgentOAuthCallbackHandoffRepository)(nil)

func NewAgentOAuthCallbackHandoffRepository(queries AgentOAuthCallbackHandoffQueries) (*AgentOAuthCallbackHandoffRepository, error) {
	if queries == nil {
		return nil, errors.New("Agent OAuth callback handoff queries are required")
	}
	return &AgentOAuthCallbackHandoffRepository{queries: queries}, nil
}

func (r *AgentOAuthCallbackHandoffRepository) CreateAgentOAuthCallbackHandoff(ctx context.Context, record application.AgentOAuthCallbackHandoffV1) (bool, error) {
	if record.Status != application.AgentOAuthCallbackHandoffRecordedV1 || record.Validate() != nil {
		return false, application.ErrAgentOAuthCallbackHandoffInvalid
	}
	rows, err := r.queries.InsertAgentOAuthCallbackHandoff(ctx, generated.InsertAgentOAuthCallbackHandoffParams{
		HandoffUuid: record.HandoffUUID, TransactionUuid: record.TransactionUUID, OwnerUserUuid: record.OwnerUserUUID,
		Issuer: record.Issuer, RedirectUri: record.RedirectURI, AuthorizationCodeSha256: record.AuthorizationCodeSHA256,
		SealedAuthorizationCode: record.SealedAuthorizationCode, RuntimeKeyID: record.RuntimeKeyID, ExpiresAt: record.ExpiresAt.UTC().Truncate(time.Millisecond),
	})
	if err != nil {
		return false, fmt.Errorf("insert Agent OAuth callback handoff: %w", err)
	}
	return rows == 1, nil
}

func (r *AgentOAuthCallbackHandoffRepository) GetAgentOAuthCallbackHandoff(ctx context.Context, handoffUUID string) (*application.AgentOAuthCallbackHandoffV1, error) {
	if !validHandoffIdentifier(handoffUUID) {
		return nil, application.ErrAgentOAuthCallbackHandoffInvalid
	}
	row, err := r.queries.GetAgentOAuthCallbackHandoff(ctx, handoffUUID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get Agent OAuth callback handoff: %w", err)
	}
	record := &application.AgentOAuthCallbackHandoffV1{
		HandoffUUID: row.HandoffUuid, TransactionUUID: row.TransactionUuid, OwnerUserUUID: row.OwnerUserUuid,
		Issuer: row.Issuer, RedirectURI: row.RedirectUri, AuthorizationCodeSHA256: row.AuthorizationCodeSha256,
		SealedAuthorizationCode: row.SealedAuthorizationCode, RuntimeKeyID: row.RuntimeKeyID,
		Status: application.AgentOAuthCallbackHandoffStatusV1(row.Status), Attempts: uint32(row.Attempts), ExpiresAt: row.ExpiresAt,
	}
	if row.LeaseOwner.Valid {
		record.LeaseOwner = row.LeaseOwner.String
	}
	if row.LeaseExpiresAt.Valid {
		record.LeaseExpiresAt = row.LeaseExpiresAt.Time
	}
	if row.CompletedAt.Valid {
		record.CompletedAt = row.CompletedAt.Time
	}
	if err := record.Validate(); err != nil {
		return nil, fmt.Errorf("stored Agent OAuth callback handoff: %w", err)
	}
	return record, nil
}

func (r *AgentOAuthCallbackHandoffRepository) ClaimAgentOAuthCallbackHandoff(ctx context.Context, handoffUUID, leaseOwner string, now, leaseExpiresAt time.Time) (bool, error) {
	if !validHandoffIdentifier(handoffUUID) || !validLease(leaseOwner, now, leaseExpiresAt) {
		return false, application.ErrAgentOAuthCallbackHandoffInvalid
	}
	now, leaseExpiresAt = canonicalHandoffTime(now), canonicalHandoffTime(leaseExpiresAt)
	rows, err := r.queries.ClaimAgentOAuthCallbackHandoff(ctx, generated.ClaimAgentOAuthCallbackHandoffParams{
		LeaseOwner: sql.NullString{String: leaseOwner, Valid: true}, LeaseExpiresAt: sql.NullTime{Time: leaseExpiresAt, Valid: true}, HandoffUuid: handoffUUID,
		ExpiresAt: now, ExpiresAt_2: leaseExpiresAt, LeaseExpiresAt_2: sql.NullTime{Time: now, Valid: true},
	})
	if err != nil {
		return false, fmt.Errorf("claim Agent OAuth callback handoff: %w", err)
	}
	return rows == 1, nil
}

func (r *AgentOAuthCallbackHandoffRepository) CompleteAgentOAuthCallbackHandoff(ctx context.Context, handoffUUID, leaseOwner string, now time.Time) (bool, error) {
	if !validHandoffIdentifier(handoffUUID) || !validCompletion(leaseOwner, now) {
		return false, application.ErrAgentOAuthCallbackHandoffInvalid
	}
	now = canonicalHandoffTime(now)
	rows, err := r.queries.CompleteAgentOAuthCallbackHandoff(ctx, generated.CompleteAgentOAuthCallbackHandoffParams{
		CompletedAt: sql.NullTime{Time: now, Valid: true}, HandoffUuid: handoffUUID, LeaseOwner: sql.NullString{String: leaseOwner, Valid: true}, LeaseExpiresAt: sql.NullTime{Time: now, Valid: true}, ExpiresAt: now,
	})
	if err != nil {
		return false, fmt.Errorf("complete Agent OAuth callback handoff: %w", err)
	}
	return rows == 1, nil
}

func (r *AgentOAuthCallbackHandoffRepository) ReleaseAgentOAuthCallbackHandoff(ctx context.Context, handoffUUID, leaseOwner string, now time.Time) (bool, error) {
	if !validHandoffIdentifier(handoffUUID) || !validHandoffLeaseOwner(leaseOwner) || now.IsZero() {
		return false, application.ErrAgentOAuthCallbackHandoffInvalid
	}
	rows, err := r.queries.ReleaseAgentOAuthCallbackHandoff(ctx, generated.ReleaseAgentOAuthCallbackHandoffParams{HandoffUuid: handoffUUID, LeaseOwner: sql.NullString{String: leaseOwner, Valid: true}, LeaseExpiresAt: sql.NullTime{Time: canonicalHandoffTime(now), Valid: true}})
	if err != nil {
		return false, fmt.Errorf("release Agent OAuth callback handoff: %w", err)
	}
	return rows == 1, nil
}

func canonicalHandoffTime(value time.Time) time.Time { return value.UTC().Truncate(time.Millisecond) }

func validHandoffIdentifier(value string) bool {
	return len(value) >= 16 && len(value) <= 64 && value == strings.TrimSpace(value) &&
		strings.Trim(value, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-") == ""
}

func validHandoffLeaseOwner(value string) bool {
	return value != "" && len(value) <= 128 && value == strings.TrimSpace(value)
}

func validLease(owner string, now, expiresAt time.Time) bool {
	return validHandoffLeaseOwner(owner) && !now.IsZero() && !expiresAt.IsZero() && expiresAt.After(now)
}

func validCompletion(owner string, now time.Time) bool {
	return validHandoffLeaseOwner(owner) && !now.IsZero()
}
