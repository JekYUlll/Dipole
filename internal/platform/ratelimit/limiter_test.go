package ratelimit

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/platform/cache"
)

func TestLimiterAllowLoginBlocksAfterLimit(t *testing.T) {
	cleanup := setupLimiterTest(t)
	defer cleanup()

	limiter := &Limiter{
		config: config.RateLimit{
			Enabled:            true,
			LoginLimit:         2,
			LoginWindowSeconds: 60,
		},
	}

	for i := 0; i < 2; i++ {
		allowed, retryAfter := limiter.AllowLogin("13800138000")
		if !allowed {
			t.Fatalf("expected login to be allowed on attempt %d, retryAfter=%s", i+1, retryAfter)
		}
	}

	allowed, retryAfter := limiter.AllowLogin("13800138000")
	if allowed {
		t.Fatalf("expected login to be limited on third attempt")
	}
	if retryAfter <= 0 {
		t.Fatalf("expected positive retryAfter, got %s", retryAfter)
	}
}

func TestLimiterAllowMessageSendUsesUserScopedCounter(t *testing.T) {
	cleanup := setupLimiterTest(t)
	defer cleanup()

	limiter := &Limiter{
		config: config.RateLimit{
			Enabled:              true,
			MessageLimit:         1,
			MessageWindowSeconds: 60,
		},
	}

	allowed, _ := limiter.AllowMessageSend("U100")
	if !allowed {
		t.Fatalf("expected first message to be allowed")
	}

	allowed, _ = limiter.AllowMessageSend("U100")
	if allowed {
		t.Fatalf("expected second message to be blocked")
	}

	allowed, _ = limiter.AllowMessageSend("U200")
	if !allowed {
		t.Fatalf("expected other user counter to stay independent")
	}
}

func TestLimiterAllowAgentMCPUsesPrincipalScopedFailClosedCounter(t *testing.T) {
	cleanup := setupLimiterTest(t)
	defer cleanup()

	limiter := &Limiter{config: config.RateLimit{AgentMCPLimit: 2, AgentMCPWindowSeconds: 60}}
	for i := 0; i < 2; i++ {
		if allowed, retryAfter := limiter.AllowAgentMCP("U100"); !allowed || retryAfter != 0 {
			t.Fatalf("MCP attempt %d: allowed=%v retryAfter=%s", i+1, allowed, retryAfter)
		}
	}
	if allowed, retryAfter := limiter.AllowAgentMCP("U100"); allowed || retryAfter <= 0 {
		t.Fatalf("third MCP attempt: allowed=%v retryAfter=%s", allowed, retryAfter)
	}
	if allowed, _ := limiter.AllowAgentMCP("U200"); !allowed {
		t.Fatal("another principal must have an independent MCP budget")
	}
	if allowed, retryAfter := (&Limiter{}).AllowAgentMCP("U300"); allowed || retryAfter != time.Minute {
		t.Fatalf("invalid MCP limit must fail closed: allowed=%v retryAfter=%s", allowed, retryAfter)
	}
}

func TestLimiterAllowAgentMCPFailsClosedWithoutRedisAndPreservesMessageFailOpen(t *testing.T) {
	oldRDB := cache.RDB
	cache.RDB = nil
	t.Cleanup(func() { cache.RDB = oldRDB })
	limiter := &Limiter{config: config.RateLimit{
		Enabled: true, MessageLimit: 1, MessageWindowSeconds: 60,
		AgentMCPLimit: 2, AgentMCPWindowSeconds: 60,
	}}
	if allowed, retryAfter := limiter.AllowAgentMCP("U100"); allowed || retryAfter != time.Minute {
		t.Fatalf("MCP dependency failure: allowed=%v retryAfter=%s", allowed, retryAfter)
	}
	if allowed, retryAfter := limiter.AllowMessageSend("U100"); !allowed || retryAfter != 0 {
		t.Fatalf("message compatibility changed: allowed=%v retryAfter=%s", allowed, retryAfter)
	}
}

func setupLimiterTest(t *testing.T) func() {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("run miniredis: %v", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	oldRDB := cache.RDB
	cache.RDB = rdb

	return func() {
		cache.RDB = oldRDB
		_ = rdb.Close()
		mr.Close()
	}
}

func TestSecondsToDuration(t *testing.T) {
	if got := secondsToDuration(2); got != 2*time.Second {
		t.Fatalf("expected 2s, got %s", got)
	}
	if got := secondsToDuration(0); got != 0 {
		t.Fatalf("expected 0 duration, got %s", got)
	}
}
