// Package cache provides thin Redis helpers for JSON serialisation/deserialisation
// and hash-field operations. All functions are no-ops when store.RDB is nil,
// so the application degrades gracefully when Redis is unavailable.
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/JekYUlll/Dipole/internal/store"
)

// TTLs for cached objects. Short enough to keep data fresh, long enough to
// absorb read bursts without hammering MySQL on every request.
const (
	UserProfileTTL      = 10 * time.Minute
	GroupMetaTTL        = 10 * time.Minute
	GroupMembersTTL     = 10 * time.Minute
	ContactRelationTTL  = 10 * time.Minute
	HotGroupMessagesTTL = time.Second
	HotGroupEmptyTTL    = 500 * time.Millisecond

	requestTimeout = time.Second
)

func NewContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), requestTimeout)
}

// Available reports whether the shared Redis client has been initialized.
func Available() bool {
	return store.RDB != nil
}

// GetBytes reads a raw value for service-owned state that is not JSON.
func GetBytes(ctx context.Context, key string) ([]byte, error) {
	if store.RDB == nil {
		return nil, fmt.Errorf("redis is not initialized")
	}

	return store.RDB.Get(ctx, key).Bytes()
}

// RunTransaction executes a Redis transaction without exposing the shared
// client to a domain package.
func RunTransaction(ctx context.Context, fn func(redis.Pipeliner)) error {
	if store.RDB == nil {
		return fmt.Errorf("redis is not initialized")
	}
	if fn == nil {
		return fmt.Errorf("redis transaction callback is required")
	}

	pipe := store.RDB.TxPipeline()
	fn(pipe)
	_, err := pipe.Exec(ctx)
	return err
}

func UserProfileKey(uuid string) string {
	return "user:profile:" + strings.TrimSpace(uuid)
}

func GroupMetaKey(groupUUID string) string {
	return "group:meta:" + strings.TrimSpace(groupUUID)
}

func GroupMembersKey(groupUUID string) string {
	return "group:members:" + strings.TrimSpace(groupUUID)
}

func ContactRelationKey(userUUID, targetUUID string) string {
	return "contact:relation:" + strings.TrimSpace(userUUID) + ":" + strings.TrimSpace(targetUUID)
}

func HotGroupMessagesKey(groupUUID string, afterID uint, limit int) string {
	return "group:messages:" + strings.TrimSpace(groupUUID) + ":after:" + fmtUint(afterID) + ":limit:" + fmtInt(limit)
}

func HotGroupMessagesSeqKey(groupUUID string, afterSeq uint64, limit int) string {
	return "group:messages:" + strings.TrimSpace(groupUUID) + ":after_seq:" + strconv.FormatUint(afterSeq, 10) + ":limit:" + fmtInt(limit)
}

func GetJSON(ctx context.Context, key string, target any) (bool, error) {
	if store.RDB == nil {
		return false, nil
	}

	value, err := store.RDB.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return false, nil
		}
		return false, err
	}
	if err := json.Unmarshal(value, target); err != nil {
		return false, err
	}

	return true, nil
}

func SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	if store.RDB == nil {
		return nil
	}

	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return store.RDB.Set(ctx, key, payload, ttl).Err()
}

// SetString writes a short-lived scalar used by service state such as token
// revocations. Unlike cache writes, missing Redis is an error for this path.
func SetString(ctx context.Context, key, value string, ttl time.Duration) error {
	if store.RDB == nil {
		return fmt.Errorf("redis is not initialized")
	}

	return store.RDB.Set(ctx, key, value, ttl).Err()
}

// Exists reports whether a service-state key is present. Missing Redis is an
// error so callers can fail closed when state cannot be validated.
func Exists(ctx context.Context, key string) (bool, error) {
	if store.RDB == nil {
		return false, fmt.Errorf("redis is not initialized")
	}

	count, err := store.RDB.Exists(ctx, key).Result()
	return count > 0, err
}

func Delete(ctx context.Context, keys ...string) error {
	if store.RDB == nil || len(keys) == 0 {
		return nil
	}

	return store.RDB.Del(ctx, keys...).Err()
}

func HashSetJSON(ctx context.Context, key, field string, value any) error {
	if store.RDB == nil {
		return nil
	}

	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return store.RDB.HSet(ctx, key, field, payload).Err()
}

func HashGetJSON(ctx context.Context, key, field string, target any) (bool, error) {
	if store.RDB == nil {
		return false, nil
	}

	value, err := store.RDB.HGet(ctx, key, field).Bytes()
	if err != nil {
		if err == redis.Nil {
			return false, nil
		}
		return false, err
	}
	if err := json.Unmarshal(value, target); err != nil {
		return false, err
	}

	return true, nil
}

func HashGetAll(ctx context.Context, key string) (map[string]string, error) {
	if store.RDB == nil {
		return nil, nil
	}

	values, err := store.RDB.HGetAll(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	return values, nil
}

func Expire(ctx context.Context, key string, ttl time.Duration) error {
	if store.RDB == nil {
		return nil
	}

	return store.RDB.Expire(ctx, key, ttl).Err()
}

func fmtUint(value uint) string {
	return strconv.FormatUint(uint64(value), 10)
}

func fmtInt(value int) string {
	return strconv.Itoa(value)
}
