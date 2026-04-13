package presence

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/store"
)

func TestRedisPresenceRegisterTouchAndUnregister(t *testing.T) {
	cleanup := setupPresenceTest(t)
	defer cleanup()

	tracker := &RedisPresence{
		config: config.Presence{
			Enabled:    true,
			NodeID:     "node-a",
			TTLSeconds: 120,
		},
		nodeID: "node-a",
	}

	state := ConnectionState{
		ConnectionID: "C100",
		UserUUID:     "U100",
		Device:       "desktop",
		ConnectedAt:  time.Now().UTC(),
	}

	tracker.Register(state)

	if got := tracker.OnlineUserCount(); got != 1 {
		t.Fatalf("expected online user count 1, got %d", got)
	}
	if got := tracker.TotalConnectionCount(); got != 1 {
		t.Fatalf("expected online connection count 1, got %d", got)
	}
	if got := tracker.UserConnectionCount("U100"); got != 1 {
		t.Fatalf("expected user connection count 1, got %d", got)
	}

	connections, err := tracker.ListUserConnections("U100")
	if err != nil {
		t.Fatalf("list user connections: %v", err)
	}
	if len(connections) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(connections))
	}
	if connections[0].NodeID != "node-a" {
		t.Fatalf("expected node id node-a, got %s", connections[0].NodeID)
	}

	time.Sleep(5 * time.Millisecond)
	tracker.Touch(state)
	connections, err = tracker.ListUserConnections("U100")
	if err != nil {
		t.Fatalf("list user connections after touch: %v", err)
	}
	if len(connections) != 1 {
		t.Fatalf("expected 1 connection after touch, got %d", len(connections))
	}
	if !connections[0].LastSeenAt.After(connections[0].ConnectedAt) && !connections[0].LastSeenAt.Equal(connections[0].ConnectedAt) {
		t.Fatalf("expected last_seen_at to be updated")
	}

	tracker.Unregister("U100", "C100")
	if got := tracker.OnlineUserCount(); got != 0 {
		t.Fatalf("expected online user count 0 after unregister, got %d", got)
	}
	if got := tracker.TotalConnectionCount(); got != 0 {
		t.Fatalf("expected online connection count 0 after unregister, got %d", got)
	}
	if got := tracker.UserConnectionCount("U100"); got != 0 {
		t.Fatalf("expected user connection count 0 after unregister, got %d", got)
	}
}

func TestRedisPresenceCountsMultipleConnections(t *testing.T) {
	cleanup := setupPresenceTest(t)
	defer cleanup()

	tracker := &RedisPresence{
		config: config.Presence{
			Enabled:    true,
			NodeID:     "node-a",
			TTLSeconds: 120,
		},
		nodeID: "node-a",
	}

	tracker.Register(ConnectionState{ConnectionID: "C100", UserUUID: "U100", ConnectedAt: time.Now().UTC()})
	tracker.Register(ConnectionState{ConnectionID: "C101", UserUUID: "U100", ConnectedAt: time.Now().UTC()})
	tracker.Register(ConnectionState{ConnectionID: "C200", UserUUID: "U200", ConnectedAt: time.Now().UTC()})

	if got := tracker.OnlineUserCount(); got != 2 {
		t.Fatalf("expected online user count 2, got %d", got)
	}
	if got := tracker.TotalConnectionCount(); got != 3 {
		t.Fatalf("expected online connection count 3, got %d", got)
	}
	if got := tracker.UserConnectionCount("U100"); got != 2 {
		t.Fatalf("expected user U100 connection count 2, got %d", got)
	}
}

func setupPresenceTest(t *testing.T) func() {
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
