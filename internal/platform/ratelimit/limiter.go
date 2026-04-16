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
	return l.allow(
		registerRateKey(identifier),
		l.config.RegisterLimit,
		secondsToDuration(l.config.RegisterWindowSeconds),
	)
}

func (l *Limiter) AllowLogin(identifier string) (bool, time.Duration) {
	return l.allow(
		loginRateKey(identifier),
		l.config.LoginLimit,
		secondsToDuration(l.config.LoginWindowSeconds),
	)
}

func (l *Limiter) AllowMessageSend(userUUID string) (bool, time.Duration) {
	return l.allow(
		messageRateKey(userUUID),
		l.config.MessageLimit,
		secondsToDuration(l.config.MessageWindowSeconds),
	)
}

func (l *Limiter) AllowFileUpload(userUUID string) (bool, time.Duration) {
	return l.allow(
		fileUploadRateKey(userUUID),
		l.config.FileUploadLimit,
		secondsToDuration(l.config.FileUploadWindowSeconds),
	)
}

func (l *Limiter) allow(key string, limit int, window time.Duration) (bool, time.Duration) {
	if l == nil || !l.config.Enabled || store.RDB == nil || limit <= 0 || window <= 0 {
		return true, 0
	}
	if key == "" {
		return true, 0
	}

	ctx, cancel := storeContext()
	defer cancel()

	count, err := store.RDB.Incr(ctx, key).Result()
	if err != nil {
		l.log.Warn("increment redis rate limit counter failed", zap.String("key", key), zap.Error(err))
		return true, 0
	}

	if count == 1 {
		if err := store.RDB.Expire(ctx, key, window).Err(); err != nil {
			l.log.Warn("set redis rate limit ttl failed", zap.String("key", key), zap.Error(err))
			return true, 0
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

func secondsToDuration(seconds int) time.Duration {
	if seconds <= 0 {
		return 0
	}

	return time.Duration(seconds) * time.Second
}

func storeContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), requestTimeout)
}
