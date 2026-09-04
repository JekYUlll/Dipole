package agentmysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
)

type AgentOAuthTokenLifecycleQueries interface {
	InsertAgentOAuthTokenLifecycleFromClaim(context.Context, generated.InsertAgentOAuthTokenLifecycleFromClaimParams) (int64, error)
	GetAgentOAuthTokenLifecycle(context.Context, string) (generated.AgentOauthTokenLifecycle, error)
}

// AgentOAuthTokenLifecycleRepository keeps the initial exchange result bound
// to the still-live callback lease. A later refresh worker gets its own
// explicit authorization slice instead of reusing a completed callback lease.
type AgentOAuthTokenLifecycleRepository struct {
	queries AgentOAuthTokenLifecycleQueries
}

var _ application.AgentOAuthTokenLifecycleStoreV1 = (*AgentOAuthTokenLifecycleRepository)(nil)

func NewAgentOAuthTokenLifecycleRepository(queries AgentOAuthTokenLifecycleQueries) (*AgentOAuthTokenLifecycleRepository, error) {
	if queries == nil {
		return nil, errors.New("Agent OAuth token lifecycle queries are required")
	}
	return &AgentOAuthTokenLifecycleRepository{queries: queries}, nil
}

func (r *AgentOAuthTokenLifecycleRepository) PersistAgentOAuthTokenLifecycle(ctx context.Context, input application.AgentOAuthTokenLifecycleWriteRequestV1, now time.Time) (bool, error) {
	if input.Validate() != nil || now.IsZero() {
		return false, application.ErrAgentOAuthTokenLifecycleInvalid
	}
	now = canonicalHandoffTime(now)
	rows, err := r.queries.InsertAgentOAuthTokenLifecycleFromClaim(ctx, generated.InsertAgentOAuthTokenLifecycleFromClaimParams{
		State:                string(input.State),
		SealedTokenBundle:    nullableLifecycleString(input.SealedTokenBundle),
		TokenBundleSha256:    nullableLifecycleString(input.TokenBundleSHA256),
		AccessTokenExpiresAt: nullableLifecycleTime(input.AccessTokenExpiresAt),
		Scope:                nullableLifecycleString(input.Scope),
		RevocationReason:     nullableLifecycleString(input.RevocationReason),
		HandoffUuid:          input.HandoffUUID,
		LeaseOwner:           sql.NullString{String: input.LeaseOwner, Valid: true},
		LeaseExpiresAt:       sql.NullTime{Time: now, Valid: true},
		ExpiresAt:            now,
	})
	if err != nil {
		return false, fmt.Errorf("persist Agent OAuth token lifecycle: %w", err)
	}
	if rows == 1 {
		return true, nil
	}

	// INSERT IGNORE returns zero both for an unavailable lease and for a
	// duplicate. Read only enough sealed metadata to accept an exact retry.
	existing, err := r.queries.GetAgentOAuthTokenLifecycle(ctx, input.HandoffUUID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("get Agent OAuth token lifecycle: %w", err)
	}
	if lifecycleMatchesWrite(existing, input) {
		return true, nil
	}
	return false, nil
}

func nullableLifecycleString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func nullableLifecycleTime(value time.Time) sql.NullTime {
	return sql.NullTime{Time: canonicalHandoffTime(value), Valid: !value.IsZero()}
}

func lifecycleMatchesWrite(row generated.AgentOauthTokenLifecycle, input application.AgentOAuthTokenLifecycleWriteRequestV1) bool {
	return row.RuntimeKeyID != "" && row.State == string(input.State) &&
		row.SealedTokenBundle.String == input.SealedTokenBundle && row.SealedTokenBundle.Valid == (input.SealedTokenBundle != "") &&
		row.TokenBundleSha256.String == input.TokenBundleSHA256 && row.TokenBundleSha256.Valid == (input.TokenBundleSHA256 != "") &&
		row.AccessTokenExpiresAt.Valid == !input.AccessTokenExpiresAt.IsZero() &&
		(!row.AccessTokenExpiresAt.Valid || row.AccessTokenExpiresAt.Time.Equal(canonicalHandoffTime(input.AccessTokenExpiresAt))) &&
		row.Scope.String == input.Scope && row.Scope.Valid == (input.Scope != "") &&
		row.RevocationReason.String == input.RevocationReason && row.RevocationReason.Valid == (input.RevocationReason != "")
}
