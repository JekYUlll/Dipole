package agentmysql

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
)

type agentOAuthAuthorizationTransactionQueriesStub struct {
	insert  func(context.Context, generated.InsertAgentOAuthAuthorizationTransactionParams) (int64, error)
	get     func(context.Context, string) (generated.AgentOauthAuthorizationTransaction, error)
	consume func(context.Context, generated.ConsumeAgentOAuthAuthorizationTransactionParams) (int64, error)
}

func (s agentOAuthAuthorizationTransactionQueriesStub) InsertAgentOAuthAuthorizationTransaction(ctx context.Context, p generated.InsertAgentOAuthAuthorizationTransactionParams) (int64, error) {
	return s.insert(ctx, p)
}
func (s agentOAuthAuthorizationTransactionQueriesStub) GetAgentOAuthAuthorizationTransaction(ctx context.Context, id string) (generated.AgentOauthAuthorizationTransaction, error) {
	return s.get(ctx, id)
}
func (s agentOAuthAuthorizationTransactionQueriesStub) ConsumeAgentOAuthAuthorizationTransaction(ctx context.Context, p generated.ConsumeAgentOAuthAuthorizationTransactionParams) (int64, error) {
	return s.consume(ctx, p)
}

func TestAgentOAuthAuthorizationTransactionRepositoryConsumesExactUnexpiredBinding(t *testing.T) {
	now := time.Date(2026, 8, 31, 0, 0, 0, 123000000, time.UTC)
	called := false
	store, err := NewAgentOAuthAuthorizationTransactionRepository(agentOAuthAuthorizationTransactionQueriesStub{
		insert: func(context.Context, generated.InsertAgentOAuthAuthorizationTransactionParams) (int64, error) {
			return 0, nil
		},
		get: func(context.Context, string) (generated.AgentOauthAuthorizationTransaction, error) {
			return generated.AgentOauthAuthorizationTransaction{}, sql.ErrNoRows
		},
		consume: func(_ context.Context, input generated.ConsumeAgentOAuthAuthorizationTransactionParams) (int64, error) {
			called = true
			if input.TransactionUuid != strings.Repeat("a", 22) || input.OwnerUserUuid != "U100" ||
				input.StateSha256 != strings.Repeat("b", 64) || !input.ConsumedAt.Valid || !input.ConsumedAt.Time.Equal(now) || !input.ExpiresAt.Equal(now) {
				t.Fatalf("consume binding drifted: %+v", input)
			}
			return 1, nil
		},
	})
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}
	consumed, err := store.ConsumeAgentOAuthAuthorizationTransaction(context.Background(), strings.Repeat("a", 22), "U100", strings.Repeat("b", 64), now)
	if err != nil || !consumed || !called {
		t.Fatalf("consume=%v err=%v called=%v", consumed, err, called)
	}
	if _, err := store.ConsumeAgentOAuthAuthorizationTransaction(context.Background(), "short", "U100", strings.Repeat("b", 64), now); !errors.Is(err, application.ErrAgentOAuthAuthorizationTransactionInvalid) {
		t.Fatalf("expected invalid binding, got %v", err)
	}
}
