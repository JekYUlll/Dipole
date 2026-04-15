package ratelimit

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/store"
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

func setupLimiterTest(t *testing.T) func() {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("run miniredis: %v", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	oldRDB := store.RDB
	store.RDB = rdb

	return func() {
		store.RDB = oldRDB
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
