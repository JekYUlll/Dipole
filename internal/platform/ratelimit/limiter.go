// Package ratelimit implements a Redis-backed fixed-window counter rate limiter.
// Each operation type (login, register, message send, file upload, Agent MCP) has its own
// key namespace and independently configured limit/window from config.yaml.
// Legacy user operations fail open on Redis errors; Agent MCP fails closed.
package ratelimit

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/logger"
	"github.com/JekYUlll/Dipole/internal/store"
)

const requestTimeout = time.Second

type Limiter struct {
	config config.RateLimit
	log    *zap.Logger
}

func NewLimiter() *Limiter {
	return &Limiter{
		config: config.RateLimitConfig(),
		log:    logger.Named("rate_limit"),
	}
}

func (l *Limiter) AllowRegister(identifier string) (bool, time.Duration) {
	if l == nil || !l.config.Enabled {
		return true, 0
	}
	return l.allow(
		registerRateKey(identifier),
		l.config.RegisterLimit,
		secondsToDuration(l.config.RegisterWindowSeconds),
		true,
	)
}

func (l *Limiter) AllowLogin(identifier string) (bool, time.Duration) {
	if l == nil || !l.config.Enabled {
		return true, 0
	}
	return l.allow(
		loginRateKey(identifier),
		l.config.LoginLimit,
		secondsToDuration(l.config.LoginWindowSeconds),
		true,
	)
}

func (l *Limiter) AllowMessageSend(userUUID string) (bool, time.Duration) {
	if l == nil || !l.config.Enabled {
		return true, 0
	}
	return l.allow(
		messageRateKey(userUUID),
		l.config.MessageLimit,
		secondsToDuration(l.config.MessageWindowSeconds),
		true,
	)
}

func (l *Limiter) AllowFileUpload(userUUID string) (bool, time.Duration) {
	if l == nil || !l.config.Enabled {
		return true, 0
	}
	return l.allow(
		fileUploadRateKey(userUUID),
		l.config.FileUploadLimit,
		secondsToDuration(l.config.FileUploadWindowSeconds),
		true,
	)
}

func (l *Limiter) AllowAgentMCP(principalUUID string) (bool, time.Duration) {
	if l == nil {
		return false, time.Minute
	}
	window := secondsToDuration(l.config.AgentMCPWindowSeconds)
	if l.config.AgentMCPLimit <= 0 || window <= 0 {
		return false, fallbackRetryAfter(window)
	}
	return l.allow(agentMCPRateKey(principalUUID), l.config.AgentMCPLimit, window, false)
}

// allow is the shared implementation for all rate-limit checks.
// It uses INCR + EXPIRE to implement a fixed-window counter in Redis.
// On the first increment the TTL is set, establishing the window boundary.
// Returns (true, 0) when allowed, or (false, retryAfter) when the limit is exceeded.
func (l *Limiter) allow(key string, limit int, window time.Duration, failOpen bool) (bool, time.Duration) {
	if store.RDB == nil || limit <= 0 || window <= 0 || key == "" {
		return rateLimitDependencyResult(failOpen, window)
	}

	ctx, cancel := storeContext()
	defer cancel()

	count, err := store.RDB.Incr(ctx, key).Result()
	if err != nil {
		l.log.Warn("increment redis rate limit counter failed", zap.String("key", key), zap.Error(err))
		return rateLimitDependencyResult(failOpen, window)
	}

	if count == 1 {
		if err := store.RDB.Expire(ctx, key, window).Err(); err != nil {
			l.log.Warn("set redis rate limit ttl failed", zap.String("key", key), zap.Error(err))
			return rateLimitDependencyResult(failOpen, window)
		}
	}

	if count <= int64(limit) {
		return true, 0
	}

	retryAfter, err := store.RDB.TTL(ctx, key).Result()
	if err != nil {
		l.log.Warn("read redis rate limit ttl failed", zap.String("key", key), zap.Error(err))
		return false, window
	}
	if retryAfter <= 0 {
		retryAfter = window
	}

	return false, retryAfter
}

func rateLimitDependencyResult(failOpen bool, window time.Duration) (bool, time.Duration) {
	if failOpen {
		return true, 0
	}
	return false, fallbackRetryAfter(window)
}

func fallbackRetryAfter(window time.Duration) time.Duration {
	if window > 0 {
		return window
	}
	return time.Minute
}

func registerRateKey(identifier string) string {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return ""
	}

	return "rate:register:" + identifier
}

func loginRateKey(identifier string) string {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return ""
	}

	return "rate:login:" + identifier
}

func messageRateKey(userUUID string) string {
	userUUID = strings.TrimSpace(userUUID)
	if userUUID == "" {
		return ""
	}

	return "rate:msg:" + userUUID
}

func fileUploadRateKey(userUUID string) string {
	userUUID = strings.TrimSpace(userUUID)
	if userUUID == "" {
		return ""
	}

	return "rate:file:" + userUUID
}

func agentMCPRateKey(principalUUID string) string {
	principalUUID = strings.TrimSpace(principalUUID)
	if principalUUID == "" {
		return ""
	}
	return "rate:agent_mcp:" + principalUUID
}

func secondsToDuration(seconds int) time.Duration {
	if seconds <= 0 {
		return 0
	}

	return time.Duration(seconds) * time.Second
}

func storeContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), requestTimeout)
}
