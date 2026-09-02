package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRequiredStateHelpersUseRedisAndTransaction(t *testing.T) {
	server := miniredis.RunT(t)
	previousRedis := RDB
	RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = RDB.Close()
		RDB = previousRedis
	})

	ctx := context.Background()
	if err := SetString(ctx, "state:token", "revoked", time.Minute); err != nil {
		t.Fatalf("SetString(): %v", err)
	}
	present, err := Exists(ctx, "state:token")
	if err != nil || !present {
		t.Fatalf("Exists() = %t, %v; want true, nil", present, err)
	}
	value, err := GetBytes(ctx, "state:token")
	if err != nil || string(value) != "revoked" {
		t.Fatalf("GetBytes() = %q, %v; want revoked, nil", value, err)
	}

	if err := RunTransaction(ctx, func(pipe redis.Pipeliner) {
		pipe.Set(ctx, "state:transaction", "committed", time.Minute)
	}); err != nil {
		t.Fatalf("RunTransaction(): %v", err)
	}
	value, err = GetBytes(ctx, "state:transaction")
	if err != nil || string(value) != "committed" {
		t.Fatalf("transaction value = %q, %v; want committed, nil", value, err)
	}
}

func TestRequiredStateHelpersFailWhenRedisIsUnavailable(t *testing.T) {
	previousRedis := RDB
	RDB = nil
	t.Cleanup(func() { RDB = previousRedis })

	ctx := context.Background()
	if Available() {
		t.Fatal("Available() = true with nil Redis client")
	}
	if err := SetString(ctx, "state:token", "revoked", time.Minute); err == nil {
		t.Fatal("SetString() succeeded without Redis")
	}
	if _, err := GetBytes(ctx, "state:token"); err == nil {
		t.Fatal("GetBytes() succeeded without Redis")
	}
	if _, err := Exists(ctx, "state:token"); err == nil {
		t.Fatal("Exists() succeeded without Redis")
	}
	if err := RunTransaction(ctx, nil); err == nil {
		t.Fatal("RunTransaction() did not fail without Redis")
	}
}
