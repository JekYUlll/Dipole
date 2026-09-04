package agentmysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	mysqlData "github.com/JekYUlll/Dipole/internal/platform/mysql"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
)

// AgentOAuthCallbackHandoffRecordingRepository keeps transaction consumption
// and encrypted handoff persistence in one SQLC transaction.
type AgentOAuthCallbackHandoffRecordingRepository struct {
	store mysqlData.TransactionStore
}

var _ application.AgentOAuthCallbackHandoffRecorderV1 = (*AgentOAuthCallbackHandoffRecordingRepository)(nil)

func NewAgentOAuthCallbackHandoffRecordingRepository(store mysqlData.TransactionStore) (*AgentOAuthCallbackHandoffRecordingRepository, error) {
	if store == nil {
		return nil, errors.New("Agent OAuth callback handoff transaction store is required")
	}
	return &AgentOAuthCallbackHandoffRecordingRepository{store: store}, nil
}

func (r *AgentOAuthCallbackHandoffRecordingRepository) RecordAgentOAuthCallbackHandoff(ctx context.Context, input application.AgentOAuthCallbackHandoffRecordRequestV1, now time.Time) (record *application.AgentOAuthCallbackHandoffV1, recorded bool, err error) {
	if input.Validate() != nil || now.IsZero() {
		return nil, false, application.ErrAgentOAuthCallbackHandoffInvalid
	}
	now = now.UTC().Truncate(time.Millisecond)
	err = r.store.WithinTx(ctx, nil, func(queries *generated.Queries) error {
		transaction, transactionErr := loadOAuthTransaction(ctx, queries, input.TransactionUUID)
		if transactionErr != nil {
			return transactionErr
		}
		if transaction.OwnerUserUUID != input.OwnerUserUUID || transaction.StateSHA256 != input.StateSHA256 ||
			now.Before(transaction.CreatedAt) || !transaction.ExpiresAt.After(now) {
			return application.ErrAgentOAuthCallbackHandoffInvalid
		}
		candidate := &application.AgentOAuthCallbackHandoffV1{
			HandoffUUID: input.HandoffUUID, TransactionUUID: transaction.TransactionUUID, OwnerUserUUID: transaction.OwnerUserUUID,
			Issuer: transaction.Issuer, RedirectURI: transaction.RedirectURI, AuthorizationCodeSHA256: input.AuthorizationCodeSHA256,
			SealedAuthorizationCode: input.SealedAuthorizationCode, RuntimeKeyID: input.RuntimeKeyID,
			Status: application.AgentOAuthCallbackHandoffRecordedV1, ExpiresAt: transaction.ExpiresAt.UTC().Truncate(time.Millisecond),
		}
		if candidate.Validate() != nil {
			return application.ErrAgentOAuthCallbackHandoffInvalid
		}
		if !transaction.ConsumedAt.IsZero() {
			existing, lookupErr := loadOAuthCallbackHandoff(ctx, queries, input.HandoffUUID)
			if lookupErr != nil || !sameOAuthCallbackHandoff(existing, candidate) {
				return application.ErrAgentOAuthCallbackHandoffInvalid
			}
			record, recorded = existing, false
			return nil
		}

		inserted, insertErr := queries.InsertAgentOAuthCallbackHandoff(ctx, generated.InsertAgentOAuthCallbackHandoffParams{
			HandoffUuid: candidate.HandoffUUID, TransactionUuid: candidate.TransactionUUID, OwnerUserUuid: candidate.OwnerUserUUID,
			Issuer: candidate.Issuer, RedirectUri: candidate.RedirectURI, AuthorizationCodeSha256: candidate.AuthorizationCodeSHA256,
			SealedAuthorizationCode: candidate.SealedAuthorizationCode, RuntimeKeyID: candidate.RuntimeKeyID, ExpiresAt: candidate.ExpiresAt,
		})
		if insertErr != nil {
			return fmt.Errorf("insert Agent OAuth callback handoff: %w", insertErr)
		}
		if inserted != 1 {
			return application.ErrAgentOAuthCallbackHandoffInvalid
		}
		consumed, consumeErr := queries.ConsumeAgentOAuthAuthorizationTransaction(ctx, generated.ConsumeAgentOAuthAuthorizationTransactionParams{
			ConsumedAt: sql.NullTime{Time: now, Valid: true}, TransactionUuid: transaction.TransactionUUID,
			OwnerUserUuid: transaction.OwnerUserUUID, StateSha256: transaction.StateSHA256, ExpiresAt: now,
		})
		if consumeErr != nil {
			return fmt.Errorf("consume Agent OAuth authorization transaction: %w", consumeErr)
		}
		if consumed != 1 {
			return application.ErrAgentOAuthCallbackHandoffInvalid
		}
		record, recorded = candidate, true
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return record, recorded, nil
}

func loadOAuthTransaction(ctx context.Context, queries *generated.Queries, transactionUUID string) (*application.AgentOAuthAuthorizationTransactionV1, error) {
	row, err := queries.GetAgentOAuthAuthorizationTransaction(ctx, transactionUUID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, application.ErrAgentOAuthCallbackHandoffInvalid
	}
	if err != nil {
		return nil, fmt.Errorf("get Agent OAuth authorization transaction: %w", err)
	}
	record := &application.AgentOAuthAuthorizationTransactionV1{TransactionUUID: row.TransactionUuid, OwnerUserUUID: row.OwnerUserUuid, Issuer: row.Issuer, RedirectURI: row.RedirectUri, StateSHA256: row.StateSha256, SealedCodeVerifier: row.SealedCodeVerifier, ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt}
	if row.ConsumedAt.Valid {
		record.ConsumedAt = row.ConsumedAt.Time
	}
	if record.Validate() != nil {
		return nil, application.ErrAgentOAuthCallbackHandoffInvalid
	}
	return record, nil
}

func loadOAuthCallbackHandoff(ctx context.Context, queries *generated.Queries, handoffUUID string) (*application.AgentOAuthCallbackHandoffV1, error) {
	row, err := queries.GetAgentOAuthCallbackHandoff(ctx, handoffUUID)
	if err != nil {
		return nil, err
	}
	record := &application.AgentOAuthCallbackHandoffV1{HandoffUUID: row.HandoffUuid, TransactionUUID: row.TransactionUuid, OwnerUserUUID: row.OwnerUserUuid, Issuer: row.Issuer, RedirectURI: row.RedirectUri, AuthorizationCodeSHA256: row.AuthorizationCodeSha256, SealedAuthorizationCode: row.SealedAuthorizationCode, RuntimeKeyID: row.RuntimeKeyID, Status: application.AgentOAuthCallbackHandoffStatusV1(row.Status), Attempts: uint32(row.Attempts), ExpiresAt: row.ExpiresAt}
	if row.LeaseOwner.Valid {
		record.LeaseOwner = row.LeaseOwner.String
	}
	if row.LeaseExpiresAt.Valid {
		record.LeaseExpiresAt = row.LeaseExpiresAt.Time
	}
	if row.CompletedAt.Valid {
		record.CompletedAt = row.CompletedAt.Time
	}
	if record.Validate() != nil {
		return nil, application.ErrAgentOAuthCallbackHandoffInvalid
	}
	return record, nil
}

func sameOAuthCallbackHandoff(left, right *application.AgentOAuthCallbackHandoffV1) bool {
	return left != nil && right != nil && left.HandoffUUID == right.HandoffUUID && left.TransactionUUID == right.TransactionUUID &&
		left.OwnerUserUUID == right.OwnerUserUUID && left.Issuer == right.Issuer && left.RedirectURI == right.RedirectURI &&
		left.AuthorizationCodeSHA256 == right.AuthorizationCodeSHA256 && left.SealedAuthorizationCode == right.SealedAuthorizationCode &&
		left.RuntimeKeyID == right.RuntimeKeyID && left.Status == application.AgentOAuthCallbackHandoffRecordedV1 &&
		left.LeaseOwner == "" && left.LeaseExpiresAt.IsZero() && left.CompletedAt.IsZero() && left.ExpiresAt.Equal(right.ExpiresAt)
}
