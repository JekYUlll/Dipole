package agentmysql

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
)

type agentOAuthTokenLifecycleQueriesStub struct {
	insert func(context.Context, generated.InsertAgentOAuthTokenLifecycleFromClaimParams) (int64, error)
	get    func(context.Context, string) (generated.AgentOauthTokenLifecycle, error)
}

func (s agentOAuthTokenLifecycleQueriesStub) InsertAgentOAuthTokenLifecycleFromClaim(ctx context.Context, params generated.InsertAgentOAuthTokenLifecycleFromClaimParams) (int64, error) {
	return s.insert(ctx, params)
}

func (s agentOAuthTokenLifecycleQueriesStub) GetAgentOAuthTokenLifecycle(ctx context.Context, handoffID string) (generated.AgentOauthTokenLifecycle, error) {
	return s.get(ctx, handoffID)
}

func TestAgentOAuthTokenLifecycleRepositoryBindsInitialWriteToLiveLease(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 4, 12, 0, 0, 123000000, time.UTC)
	input := application.AgentOAuthTokenLifecycleWriteRequestV1{
		HandoffUUID: "handoff_123456789", LeaseOwner: "runtime-worker-1", State: application.AgentOAuthTokenLifecycleActiveV1,
		SealedTokenBundle: "v1.nonce.ciphertext.tag.wrapped", TokenBundleSHA256: strings.Repeat("a", 64), AccessTokenExpiresAt: now.Add(time.Hour), Scope: "calendar.read",
	}
	store, err := NewAgentOAuthTokenLifecycleRepository(agentOAuthTokenLifecycleQueriesStub{
		insert: func(_ context.Context, params generated.InsertAgentOAuthTokenLifecycleFromClaimParams) (int64, error) {
			if params.HandoffUuid != input.HandoffUUID || !params.LeaseOwner.Valid || params.LeaseOwner.String != input.LeaseOwner ||
				!params.LeaseExpiresAt.Valid || !params.LeaseExpiresAt.Time.Equal(now) || !params.ExpiresAt.Equal(now) ||
				params.State != string(input.State) || params.SealedTokenBundle.String != input.SealedTokenBundle || params.TokenBundleSha256.String != input.TokenBundleSHA256 ||
				!params.AccessTokenExpiresAt.Time.Equal(input.AccessTokenExpiresAt) || params.Scope.String != input.Scope || params.RevocationReason.Valid {
				t.Fatalf("persist parameters drifted: %+v", params)
			}
			return 1, nil
		},
		get: func(context.Context, string) (generated.AgentOauthTokenLifecycle, error) {
			return generated.AgentOauthTokenLifecycle{}, sql.ErrNoRows
		},
	})
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}
	if persisted, err := store.PersistAgentOAuthTokenLifecycle(context.Background(), input, now); err != nil || !persisted {
		t.Fatalf("persisted=%v err=%v", persisted, err)
	}
}

func TestAgentOAuthTokenLifecycleRepositoryAcceptsOnlyExactRetry(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 4, 12, 0, 0, 123000000, time.UTC)
	input := application.AgentOAuthTokenLifecycleWriteRequestV1{
		HandoffUUID: "handoff_123456789", LeaseOwner: "runtime-worker-1", State: application.AgentOAuthTokenLifecycleActiveV1,
		SealedTokenBundle: "v1.nonce.ciphertext.tag.wrapped", TokenBundleSHA256: strings.Repeat("a", 64), AccessTokenExpiresAt: now.Add(time.Hour), Scope: "calendar.read",
	}
	store, err := NewAgentOAuthTokenLifecycleRepository(agentOAuthTokenLifecycleQueriesStub{
		insert: func(context.Context, generated.InsertAgentOAuthTokenLifecycleFromClaimParams) (int64, error) {
			return 0, nil
		},
		get: func(_ context.Context, handoffID string) (generated.AgentOauthTokenLifecycle, error) {
			return generated.AgentOauthTokenLifecycle{
				HandoffUuid: handoffID, RuntimeKeyID: "runtime-key-1", State: string(input.State),
				SealedTokenBundle: sql.NullString{String: input.SealedTokenBundle, Valid: true}, TokenBundleSha256: sql.NullString{String: input.TokenBundleSHA256, Valid: true},
				AccessTokenExpiresAt: sql.NullTime{Time: input.AccessTokenExpiresAt, Valid: true}, Scope: sql.NullString{String: input.Scope, Valid: true},
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}
	if persisted, err := store.PersistAgentOAuthTokenLifecycle(context.Background(), input, now); err != nil || !persisted {
		t.Fatalf("exact retry persisted=%v err=%v", persisted, err)
	}
}
