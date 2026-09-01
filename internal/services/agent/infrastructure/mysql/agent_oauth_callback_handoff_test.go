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

type agentOAuthCallbackHandoffQueriesStub struct {
	insert   func(context.Context, generated.InsertAgentOAuthCallbackHandoffParams) (int64, error)
	get      func(context.Context, string) (generated.AgentOauthCallbackHandoff, error)
	claim    func(context.Context, generated.ClaimAgentOAuthCallbackHandoffParams) (int64, error)
	complete func(context.Context, generated.CompleteAgentOAuthCallbackHandoffParams) (int64, error)
	release  func(context.Context, generated.ReleaseAgentOAuthCallbackHandoffParams) (int64, error)
}

func (s agentOAuthCallbackHandoffQueriesStub) InsertAgentOAuthCallbackHandoff(c context.Context, p generated.InsertAgentOAuthCallbackHandoffParams) (int64, error) {
	return s.insert(c, p)
}
func (s agentOAuthCallbackHandoffQueriesStub) GetAgentOAuthCallbackHandoff(c context.Context, id string) (generated.AgentOauthCallbackHandoff, error) {
	return s.get(c, id)
}
func (s agentOAuthCallbackHandoffQueriesStub) ClaimAgentOAuthCallbackHandoff(c context.Context, p generated.ClaimAgentOAuthCallbackHandoffParams) (int64, error) {
	return s.claim(c, p)
}
func (s agentOAuthCallbackHandoffQueriesStub) CompleteAgentOAuthCallbackHandoff(c context.Context, p generated.CompleteAgentOAuthCallbackHandoffParams) (int64, error) {
	return s.complete(c, p)
}
func (s agentOAuthCallbackHandoffQueriesStub) ReleaseAgentOAuthCallbackHandoff(c context.Context, p generated.ReleaseAgentOAuthCallbackHandoffParams) (int64, error) {
	return s.release(c, p)
}

func TestAgentOAuthCallbackHandoffRepositoryUsesLeaseBoundTransitions(t *testing.T) {
	now := time.Date(2026, 8, 31, 0, 0, 0, 123000000, time.UTC)
	handoffID, transactionID := strings.Repeat("a", 22), strings.Repeat("b", 22)
	called := map[string]bool{}
	store, err := NewAgentOAuthCallbackHandoffRepository(agentOAuthCallbackHandoffQueriesStub{
		insert: func(_ context.Context, p generated.InsertAgentOAuthCallbackHandoffParams) (int64, error) {
			called["insert"] = true
			if p.HandoffUuid != handoffID || p.TransactionUuid != transactionID || p.SealedAuthorizationCode != "v1.abc.def.ghi" || p.RuntimeKeyID != "oauth-runtime-1" {
				t.Fatalf("insert drifted: %+v", p)
			}
			return 1, nil
		},
		get: func(context.Context, string) (generated.AgentOauthCallbackHandoff, error) {
			return generated.AgentOauthCallbackHandoff{}, sql.ErrNoRows
		},
		claim: func(_ context.Context, p generated.ClaimAgentOAuthCallbackHandoffParams) (int64, error) {
			called["claim"] = true
			if p.HandoffUuid != handoffID || !p.LeaseOwner.Valid || p.LeaseOwner.String != "agent-runtime-1" || !p.LeaseExpiresAt.Time.Equal(now.Add(time.Minute)) || !p.ExpiresAt.Equal(now) || !p.ExpiresAt_2.Equal(now.Add(time.Minute)) || !p.LeaseExpiresAt_2.Time.Equal(now) {
				t.Fatalf("claim drifted: %+v", p)
			}
			return 1, nil
		},
		complete: func(_ context.Context, p generated.CompleteAgentOAuthCallbackHandoffParams) (int64, error) {
			called["complete"] = true
			if p.HandoffUuid != handoffID || !p.LeaseOwner.Valid || p.LeaseOwner.String != "agent-runtime-1" || !p.CompletedAt.Time.Equal(now) || !p.LeaseExpiresAt.Time.Equal(now) || !p.ExpiresAt.Equal(now) {
				t.Fatalf("complete drifted: %+v", p)
			}
			return 1, nil
		},
		release: func(_ context.Context, p generated.ReleaseAgentOAuthCallbackHandoffParams) (int64, error) {
			called["release"] = true
			if p.HandoffUuid != handoffID || !p.LeaseOwner.Valid || p.LeaseOwner.String != "agent-runtime-1" || !p.LeaseExpiresAt.Time.Equal(now) {
				t.Fatalf("release drifted: %+v", p)
			}
			return 1, nil
		},
	})
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}
	recorded := application.AgentOAuthCallbackHandoffV1{HandoffUUID: handoffID, TransactionUUID: transactionID, OwnerUserUUID: "U100", Issuer: "https://auth.example.com", RedirectURI: "https://dipole.example.com/oauth/callback", AuthorizationCodeSHA256: strings.Repeat("c", 64), SealedAuthorizationCode: "v1.abc.def.ghi", RuntimeKeyID: "oauth-runtime-1", Status: application.AgentOAuthCallbackHandoffRecordedV1, ExpiresAt: now.Add(10 * time.Minute)}
	if ok, err := store.CreateAgentOAuthCallbackHandoff(context.Background(), recorded); err != nil || !ok {
		t.Fatalf("create=%v err=%v", ok, err)
	}
	if ok, err := store.ClaimAgentOAuthCallbackHandoff(context.Background(), handoffID, "agent-runtime-1", now, now.Add(time.Minute)); err != nil || !ok {
		t.Fatalf("claim=%v err=%v", ok, err)
	}
	if ok, err := store.CompleteAgentOAuthCallbackHandoff(context.Background(), handoffID, "agent-runtime-1", now); err != nil || !ok {
		t.Fatalf("complete=%v err=%v", ok, err)
	}
	if ok, err := store.ReleaseAgentOAuthCallbackHandoff(context.Background(), handoffID, "agent-runtime-1", now); err != nil || !ok {
		t.Fatalf("release=%v err=%v", ok, err)
	}
	if !called["insert"] || !called["claim"] || !called["complete"] || !called["release"] {
		t.Fatalf("missing transition: %#v", called)
	}
	if _, err := store.ClaimAgentOAuthCallbackHandoff(context.Background(), "short", "agent-runtime-1", now, now.Add(time.Minute)); !errors.Is(err, application.ErrAgentOAuthCallbackHandoffInvalid) {
		t.Fatalf("expected invalid claim, got %v", err)
	}
}
