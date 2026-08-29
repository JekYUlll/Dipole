package coreauth

import (
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/store"
)

func TestAgentMCPAccessTokenIsAudienceAndScopeBound(t *testing.T) {
	t.Chdir("../../../../..")
	t.Setenv("DIPOLE_CONFIG_FILE", "configs/config.dist.yaml")
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer mr.Close()
	previousRedis := store.RDB
	store.RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = store.RDB.Close(); store.RDB = previousRedis })

	tokens := NewTokenService()
	accessToken, err := tokens.IssueAgentMCPAccessToken("U100", AgentMCPResource, []string{AgentMCPReadScope}, true)
	if err != nil {
		t.Fatalf("issue Agent MCP access token: %v", err)
	}
	session, err := tokens.ResolveAgentMCPAccessToken(accessToken, AgentMCPResource, AgentMCPReadScope)
	if err != nil || session.UserUUID != "U100" {
		t.Fatalf("resolve Agent MCP access token: session=%+v err=%v", session, err)
	}
	if _, err := tokens.ResolveSession(accessToken); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("MCP token was accepted as a session token: %v", err)
	}
	if _, err := tokens.ResolveAgentMCPAccessToken(accessToken, "https://other.example/mcp", AgentMCPReadScope); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("wrong audience was accepted: %v", err)
	}
	if _, err := tokens.ResolveAgentMCPAccessToken(accessToken, AgentMCPResource, "dipole.agent.mcp.write"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("wrong scope was accepted: %v", err)
	}

	sessionToken, err := tokens.Issue(&model.User{UUID: "U100"})
	if err != nil {
		t.Fatalf("issue session token: %v", err)
	}
	if _, err := tokens.ResolveAgentMCPAccessToken(sessionToken, AgentMCPResource, AgentMCPReadScope); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("session token was accepted by MCP: %v", err)
	}
}

func TestAgentMCPAccessTokenRequiresExplicitExactConsent(t *testing.T) {
	t.Chdir("../../../../..")
	t.Setenv("DIPOLE_CONFIG_FILE", "configs/config.dist.yaml")
	tokens := NewTokenService()
	for name, testCase := range map[string]struct {
		resource string
		scopes   []string
		consent  bool
	}{
		"missing consent": {AgentMCPResource, []string{AgentMCPReadScope}, false},
		"wrong resource":  {"https://other.example/mcp", []string{AgentMCPReadScope}, true},
		"wrong scope":     {AgentMCPResource, []string{"dipole.agent.mcp.write"}, true},
		"extra scope":     {AgentMCPResource, []string{AgentMCPReadScope, "dipole.agent.mcp.write"}, true},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := tokens.IssueAgentMCPAccessToken("U100", testCase.resource, testCase.scopes, testCase.consent); !errors.Is(err, ErrInvalidAgentMCPGrant) {
				t.Fatalf("expected invalid grant, got %v", err)
			}
		})
	}
}
