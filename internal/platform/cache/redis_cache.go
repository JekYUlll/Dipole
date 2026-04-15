package cache

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/JekYUlll/Dipole/internal/store"
)

const (
	UserProfileTTL     = 10 * time.Minute
	GroupMetaTTL       = 10 * time.Minute
	GroupMembersTTL    = 10 * time.Minute
	ContactRelationTTL = 10 * time.Minute

	requestTimeout = time.Second
)

func NewContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), requestTimeout)
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
