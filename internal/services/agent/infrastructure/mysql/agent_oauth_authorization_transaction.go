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

type AgentOAuthAuthorizationTransactionQueries interface {
	InsertAgentOAuthAuthorizationTransaction(context.Context, generated.InsertAgentOAuthAuthorizationTransactionParams) (int64, error)
	GetAgentOAuthAuthorizationTransaction(context.Context, string) (generated.AgentOauthAuthorizationTransaction, error)
	ConsumeAgentOAuthAuthorizationTransaction(context.Context, generated.ConsumeAgentOAuthAuthorizationTransactionParams) (int64, error)
}

type AgentOAuthAuthorizationTransactionRepository struct {
	queries AgentOAuthAuthorizationTransactionQueries
}

var _ application.AgentOAuthAuthorizationTransactionStoreV1 = (*AgentOAuthAuthorizationTransactionRepository)(nil)

func NewAgentOAuthAuthorizationTransactionRepository(queries AgentOAuthAuthorizationTransactionQueries) (*AgentOAuthAuthorizationTransactionRepository, error) {
	if queries == nil {
		return nil, errors.New("Agent OAuth authorization transaction queries are required")
	}
	return &AgentOAuthAuthorizationTransactionRepository{queries: queries}, nil
}

func (r *AgentOAuthAuthorizationTransactionRepository) CreateAgentOAuthAuthorizationTransaction(ctx context.Context, record application.AgentOAuthAuthorizationTransactionV1) (bool, error) {
	if err := record.Validate(); err != nil {
		return false, err
	}
	rows, err := r.queries.InsertAgentOAuthAuthorizationTransaction(ctx, generated.InsertAgentOAuthAuthorizationTransactionParams{
		TransactionUuid: record.TransactionUUID, OwnerUserUuid: record.OwnerUserUUID, Issuer: record.Issuer,
		RedirectUri: record.RedirectURI, StateSha256: record.StateSHA256, SealedCodeVerifier: record.SealedCodeVerifier, ExpiresAt: record.ExpiresAt,
	})
	if err != nil {
		return false, fmt.Errorf("insert Agent OAuth authorization transaction: %w", err)
	}
	return rows == 1, nil
}

func (r *AgentOAuthAuthorizationTransactionRepository) GetAgentOAuthAuthorizationTransaction(ctx context.Context, transactionUUID string) (*application.AgentOAuthAuthorizationTransactionV1, error) {
	row, err := r.queries.GetAgentOAuthAuthorizationTransaction(ctx, transactionUUID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get Agent OAuth authorization transaction: %w", err)
	}
	record := &application.AgentOAuthAuthorizationTransactionV1{TransactionUUID: row.TransactionUuid, OwnerUserUUID: row.OwnerUserUuid,
		Issuer: row.Issuer, RedirectURI: row.RedirectUri, StateSHA256: row.StateSha256, SealedCodeVerifier: row.SealedCodeVerifier,
		ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt}
	if row.ConsumedAt.Valid {
		record.ConsumedAt = row.ConsumedAt.Time
	}
	if err := record.Validate(); err != nil {
		return nil, fmt.Errorf("stored Agent OAuth authorization transaction: %w", err)
	}
	return record, nil
}

func (r *AgentOAuthAuthorizationTransactionRepository) ConsumeAgentOAuthAuthorizationTransaction(ctx context.Context, transactionUUID, ownerUserUUID, stateSHA256 string, now time.Time) (bool, error) {
	if len(transactionUUID) < 16 || transactionUUID != strings.TrimSpace(transactionUUID) || len(transactionUUID) > 64 ||
		strings.Trim(transactionUUID, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-") != "" ||
		ownerUserUUID == "" || ownerUserUUID != strings.TrimSpace(ownerUserUUID) || len(ownerUserUUID) > 64 ||
		len(stateSHA256) != 64 || strings.Trim(stateSHA256, "0123456789abcdef") != "" || now.IsZero() {
		return false, application.ErrAgentOAuthAuthorizationTransactionInvalid
	}
	now = now.UTC().Truncate(time.Millisecond)
	rows, err := r.queries.ConsumeAgentOAuthAuthorizationTransaction(ctx, generated.ConsumeAgentOAuthAuthorizationTransactionParams{
		ConsumedAt: sql.NullTime{Time: now, Valid: true}, TransactionUuid: transactionUUID,
		OwnerUserUuid: ownerUserUUID, StateSha256: stateSHA256, ExpiresAt: now,
	})
	if err != nil {
		return false, fmt.Errorf("consume Agent OAuth authorization transaction: %w", err)
	}
	return rows == 1, nil
}
