package delivery

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRedisAuthorityFenceWriterRealCAS(t *testing.T) {
	address := os.Getenv("DIPOLE_TEST_REALTIME_FENCE_REDIS_ENDPOINT")
	if address == "" {
		t.Skip("DIPOLE_TEST_REALTIME_FENCE_REDIS_ENDPOINT is not set")
	}
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: address})
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping isolated Redis: %v", err)
	}
	suffix := time.Now().UTC().Format("20060102T150405.000000000")
	key := "dipole:test:realtime:fence:" + suffix
	receiptPrefix := key + ":receipt:"
	t.Cleanup(func() {
		_ = client.Del(ctx, key, receiptPrefix+"bootstrap", receiptPrefix+"freeze", receiptPrefix+"activate").Err()
	})
	now := time.Now().UTC()
	writer, err := NewRedisAuthorityFenceWriter(client, key, receiptPrefix, time.Hour, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	bootstrap, err := writer.Apply(ctx, FenceTransitionRequest{
		TransitionID: "bootstrap", Action: FenceTransitionBootstrap, OperatorID: "integration",
		Reason: "real Redis Lua CAS", TargetAuthority: AuthorityGo, LeaseUntil: now.Add(time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := writer.Apply(ctx, FenceTransitionRequest{
		TransitionID: "freeze", Action: FenceTransitionFreeze, OperatorID: "integration",
		Reason: "real Redis freeze", ExpectedSHA256: bootstrap.NextSHA256, LeaseUntil: now.Add(time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	activated, err := writer.Apply(ctx, FenceTransitionRequest{
		TransitionID: "activate", Action: FenceTransitionActivate, OperatorID: "integration",
		Reason: "real Redis activate", ExpectedSHA256: frozen.NextSHA256,
		TargetAuthority: AuthorityCPP, LeaseUntil: now.Add(time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	if activated.Authority != AuthorityCPP || activated.Phase != FencePhaseActive || activated.Epoch != 2 {
		t.Fatalf("activated receipt = %+v", activated)
	}
	if ttl := client.PTTL(ctx, key).Val(); ttl <= 0 || ttl > time.Minute {
		t.Fatalf("real Redis lease TTL = %v", ttl)
	}
	if count, err := client.Exists(ctx, receiptPrefix+"bootstrap", receiptPrefix+"freeze", receiptPrefix+"activate").Result(); err != nil || count != 3 {
		t.Fatalf("real Redis receipt count = %d, error = %v", count, err)
	}
}
