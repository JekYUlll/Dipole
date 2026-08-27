package store_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/platform/hotgroup"
	"github.com/JekYUlll/Dipole/internal/platform/presence"
	"github.com/JekYUlll/Dipole/internal/platform/ratelimit"
	"github.com/JekYUlll/Dipole/internal/store"
)

func TestRedisSentinelFailoverPreservesRealtimeSemantics(t *testing.T) {
	sentinelAddresses := splitAddresses(os.Getenv("DIPOLE_TEST_REDIS_SENTINELS"))
	if len(sentinelAddresses) == 0 {
		t.Skip("DIPOLE_TEST_REDIS_SENTINELS is required for Redis failover integration tests")
	}

	const masterName = "dipole-master"
	client, err := store.NewRedisClient(config.Redis{
		Mode:               "sentinel",
		SentinelMasterName: masterName,
		SentinelAddresses:  sentinelAddresses,
	})
	if err != nil {
		t.Fatalf("create Redis failover client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	originalRDB := store.RDB
	store.RDB = client
	t.Cleanup(func() { store.RDB = originalRDB })

	sentinel := redis.NewSentinelClient(&redis.Options{Addr: sentinelAddresses[0]})
	t.Cleanup(func() { _ = sentinel.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	originalPrimary := waitForPrimary(t, ctx, sentinel, masterName, "")
	if err := client.Set(ctx, "dipole:failover:confirmed", "before", 0).Err(); err != nil {
		t.Fatalf("write confirmed probe before failover: %v", err)
	}

	presenceStore := presence.NewRedisPresence()
	if presenceStore == nil {
		t.Fatal("expected Redis Presence to be enabled")
	}
	presenceStore.Register(presence.ConnectionState{
		ConnectionID: "C-FAILOVER",
		UserUUID:     "U-FAILOVER",
		Device:       "integration",
	})

	detector := hotgroup.NewDetector(config.HotGroup{
		Enabled:              true,
		MemberCountThreshold: 2,
		MessageThreshold:     2,
		WindowSeconds:        60,
		CoolingSeconds:       180,
	})
	for range 2 {
		if _, err := detector.ObserveMessage("G-FAILOVER", 3); err != nil {
			t.Fatalf("record hot-group state: %v", err)
		}
	}

	limiter := ratelimit.NewLimiter()
	rateLimitConfig := config.RateLimitConfig()
	if rateLimitConfig.LoginLimit <= 0 {
		t.Fatalf("expected positive login rate limit, got %d", rateLimitConfig.LoginLimit)
	}
	for attempt := 1; attempt <= rateLimitConfig.LoginLimit; attempt++ {
		if allowed, _ := limiter.AllowLogin("redis-failover-user"); !allowed {
			t.Fatalf("expected rate-limit attempt %d to be allowed", attempt)
		}
	}

	subscription := client.Subscribe(ctx, "dipole:failover:pubsub")
	t.Cleanup(func() { _ = subscription.Close() })
	if _, err := subscription.Receive(ctx); err != nil {
		t.Fatalf("establish Pub/Sub subscription: %v", err)
	}
	messages := subscription.Channel(redis.WithChannelSize(16))
	if err := client.Publish(ctx, "dipole:failover:pubsub", "before").Err(); err != nil {
		t.Fatalf("publish before failover: %v", err)
	}
	waitForPubSubMessage(t, ctx, messages, "before")

	waitForReplicas(t, ctx, client, 2)
	fmt.Printf("REDIS_FAILOVER_PRIMARY_READY=%s\n", originalPrimary)

	newPrimary := waitForPrimary(t, ctx, sentinel, masterName, originalPrimary)
	waitForClientWrite(t, ctx, client)

	if value, err := client.Get(ctx, "dipole:failover:confirmed").Result(); err != nil || value != "before" {
		t.Fatalf("read confirmed probe after failover: value=%q err=%v", value, err)
	}
	connections, err := presenceStore.ListUserConnections("U-FAILOVER")
	if err != nil || len(connections) != 1 || connections[0].ConnectionID != "C-FAILOVER" {
		t.Fatalf("verify Presence after failover: connections=%+v err=%v", connections, err)
	}
	status, err := detector.Status("G-FAILOVER", 3)
	if err != nil || !status.IsHot {
		t.Fatalf("verify hot-group state after failover: status=%+v err=%v", status, err)
	}
	if allowed, retryAfter := limiter.AllowLogin("redis-failover-user"); allowed || retryAfter <= 0 {
		t.Fatalf("verify rate-limit state after failover: allowed=%v retry_after=%s", allowed, retryAfter)
	}

	waitForPubSubRecovery(t, ctx, client, messages)
	fmt.Printf("REDIS_FAILOVER_OK=%s\n", newPrimary)
}

func splitAddresses(raw string) []string {
	parts := strings.Split(raw, ",")
	addresses := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			addresses = append(addresses, part)
		}
	}
	return addresses
}

func waitForPrimary(t *testing.T, ctx context.Context, sentinel *redis.SentinelClient, masterName, previous string) string {
	t.Helper()
	for ctx.Err() == nil {
		address, err := sentinel.GetMasterAddrByName(ctx, masterName).Result()
		if err == nil && len(address) == 2 {
			primary := address[0] + ":" + address[1]
			if primary != previous {
				return primary
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("wait for Redis primary change from %q: %v", previous, ctx.Err())
	return ""
}

func waitForReplicas(t *testing.T, ctx context.Context, client *redis.Client, expected int64) {
	t.Helper()
	for ctx.Err() == nil {
		count, err := client.Do(ctx, "WAIT", expected, 1000).Int64()
		if err == nil && count >= expected {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("wait for %d Redis replicas: %v", expected, ctx.Err())
}

func waitForClientWrite(t *testing.T, ctx context.Context, client *redis.Client) {
	t.Helper()
	for ctx.Err() == nil {
		if err := client.Set(ctx, "dipole:failover:after", "after", 0).Err(); err == nil {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("wait for go-redis failover client: %v", ctx.Err())
}

func waitForPubSubRecovery(t *testing.T, ctx context.Context, client *redis.Client, messages <-chan *redis.Message) {
	t.Helper()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for ctx.Err() == nil {
		if err := client.Publish(ctx, "dipole:failover:pubsub", "after").Err(); err != nil {
			continue
		}
		select {
		case message := <-messages:
			if message != nil && message.Payload == "after" {
				return
			}
		case <-ticker.C:
		case <-ctx.Done():
		}
	}
	t.Fatalf("wait for Pub/Sub recovery: %v", ctx.Err())
}

func waitForPubSubMessage(t *testing.T, ctx context.Context, messages <-chan *redis.Message, expected string) {
	t.Helper()
	select {
	case message := <-messages:
		if message == nil || message.Payload != expected {
			t.Fatalf("expected Pub/Sub payload %q, got %+v", expected, message)
		}
	case <-ctx.Done():
		t.Fatalf("wait for Pub/Sub payload %q: %v", expected, ctx.Err())
	}
}
