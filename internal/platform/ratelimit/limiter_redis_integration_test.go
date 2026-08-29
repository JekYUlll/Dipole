package ratelimit

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/redis/go-redis/v9"
)

func TestAgentMCPRateLimitRealRedisContract(t *testing.T) {
	address := os.Getenv("DIPOLE_TEST_REDIS_ADDR")
	if address == "" {
		t.Skip("DIPOLE_TEST_REDIS_ADDR is required for Redis rate-limit integration tests")
	}
	client := redis.NewClient(&redis.Options{Addr: address})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping Redis: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	principal := fmt.Sprintf("contract-%d", time.Now().UnixNano())
	key := agentMCPRateKey(principal)
	t.Cleanup(func() { _ = client.Del(context.Background(), key).Err() })
	configuration := config.RateLimit{AgentMCPLimit: 1, AgentMCPWindowSeconds: 1}
	first, second := &Limiter{config: configuration, redis: client}, &Limiter{config: configuration, redis: client}
	if allowed, _ := first.AllowAgentMCP(principal); !allowed {
		t.Fatal("first Gateway instance rejected the principal")
	}
	if allowed, retryAfter := second.AllowAgentMCP(principal); allowed || retryAfter <= 0 || retryAfter > time.Second {
		t.Fatalf("second Gateway instance did not share the limit: allowed=%v retryAfter=%s", allowed, retryAfter)
	}
	time.Sleep(1100 * time.Millisecond)
	if allowed, retryAfter := second.AllowAgentMCP(principal); !allowed || retryAfter != 0 {
		t.Fatalf("expired window did not recover: allowed=%v retryAfter=%s", allowed, retryAfter)
	}
}
