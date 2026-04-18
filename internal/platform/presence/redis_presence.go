// Package presence tracks which WebSocket connections are live and on which node.
// Each connection is stored as a JSON-serialised ConnectionState in a Redis hash
// keyed by user UUID. Two sorted sets (scored by expiry Unix timestamp) allow
// fast online-user and online-connection counts without a full scan.
//
// Data layout:
//   presence:user:<uuid>:connections  — HASH  field=connectionID  value=ConnectionState JSON
//   presence:online_users             — ZSET  member=userUUID     score=expiry unix
//   presence:online_connections       — ZSET  member=connectionID score=expiry unix
package presence

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/logger"
	"github.com/JekYUlll/Dipole/internal/store"
)

const requestTimeout = time.Second

type ConnectionState struct {
	ConnectionID   string    `json:"connection_id"`
	UserUUID       string    `json:"user_uuid"`
	TokenID        string    `json:"token_id,omitempty"`
	TokenExpiresAt time.Time `json:"token_expires_at,omitempty"`
	NodeID         string    `json:"node_id"`
	Device         string    `json:"device"`
	DeviceID       string    `json:"device_id,omitempty"`
	UserAgent      string    `json:"user_agent,omitempty"`
	RemoteAddr     string    `json:"remote_addr,omitempty"`
	ConnectedAt    time.Time `json:"connected_at"`
	LastSeenAt     time.Time `json:"last_seen_at"`
}

type RedisPresence struct {
	config config.Presence
	nodeID string
	log    *zap.Logger
}

func NewRedisPresence() *RedisPresence {
	cfg := config.PresenceConfig()
	if !cfg.Enabled {
		return nil
	}

	return &RedisPresence{
		config: cfg,
		nodeID: resolveNodeID(cfg),
		log:    logger.Named("presence"),
	}
}

// Register writes a new connection into Redis. Called when a WS client connects.
func (p *RedisPresence) Register(state ConnectionState) {
	if !p.shouldRun(state.UserUUID, state.ConnectionID) {
		return
	}

	now := time.Now().UTC()
	state.NodeID = p.nodeID
	state.LastSeenAt = now
	if state.ConnectedAt.IsZero() {
		state.ConnectedAt = now
	}

	if err := p.writeState(state); err != nil {
		p.log.Warn("register redis presence failed",
			zap.String("user_uuid", state.UserUUID),
			zap.String("connection_id", state.ConnectionID),
			zap.Error(err),
		)
	}
}

// Touch refreshes the TTL of an existing connection. Called on WS heartbeat/ping.
func (p *RedisPresence) Touch(state ConnectionState) {
	if !p.shouldRun(state.UserUUID, state.ConnectionID) {
		return
	}

	now := time.Now().UTC()
	state.NodeID = p.nodeID
	state.LastSeenAt = now
	if state.ConnectedAt.IsZero() {
		state.ConnectedAt = now
	}

	if err := p.writeState(state); err != nil {
		p.log.Warn("touch redis presence failed",
			zap.String("user_uuid", state.UserUUID),
			zap.String("connection_id", state.ConnectionID),
			zap.Error(err),
		)
	}
}

// Unregister removes a connection from Redis. If it was the user's last connection,
// the user entry is also removed from the online-users sorted set.
func (p *RedisPresence) Unregister(userUUID, connectionID string) {
	userUUID = strings.TrimSpace(userUUID)
	connectionID = strings.TrimSpace(connectionID)
	if !p.shouldRun(userUUID, connectionID) {
		return
	}

	ctx, cancel := withTimeout()
	defer cancel()

	pipe := store.RDB.TxPipeline()
	pipe.HDel(ctx, userConnectionsKey(userUUID), connectionID)
	pipe.ZRem(ctx, onlineConnectionsKey(), connectionID)
	remainingCmd := pipe.HLen(ctx, userConnectionsKey(userUUID))
	_, err := pipe.Exec(ctx)
	if err != nil {
		p.log.Warn("unregister redis presence failed",
			zap.String("user_uuid", userUUID),
			zap.String("connection_id", connectionID),
			zap.Error(err),
		)
		return
	}

	remaining := remainingCmd.Val()
	if remaining <= 0 {
		ctx2, cancel2 := withTimeout()
		defer cancel2()
		pipe2 := store.RDB.TxPipeline()
		pipe2.Del(ctx2, userConnectionsKey(userUUID))
		pipe2.ZRem(ctx2, onlineUsersKey(), userUUID)
		if _, err := pipe2.Exec(ctx2); err != nil {
			p.log.Warn("clear redis presence user state failed",
				zap.String("user_uuid", userUUID),
				zap.Error(err),
			)
		}
		return
	}

	if err := p.refreshUserExpiry(userUUID); err != nil {
		p.log.Warn("refresh redis presence user expiry failed",
			zap.String("user_uuid", userUUID),
			zap.Error(err),
		)
	}
}

func (p *RedisPresence) OnlineUserCount() int {
	if !p.enabled() {
		return 0
	}

	ctx, cancel := withTimeout()
	defer cancel()

	p.cleanupExpired(ctx)
	count, err := store.RDB.ZCard(ctx, onlineUsersKey()).Result()
	if err != nil {
		p.log.Warn("count redis online users failed", zap.Error(err))
		return 0
	}

	return int(count)
}

func (p *RedisPresence) TotalConnectionCount() int {
	if !p.enabled() {
		return 0
	}

	ctx, cancel := withTimeout()
	defer cancel()

	p.cleanupExpired(ctx)
	count, err := store.RDB.ZCard(ctx, onlineConnectionsKey()).Result()
	if err != nil {
		p.log.Warn("count redis online connections failed", zap.Error(err))
		return 0
	}

	return int(count)
}

func (p *RedisPresence) UserConnectionCount(userUUID string) int {
	userUUID = strings.TrimSpace(userUUID)
	if !p.shouldRun(userUUID, "presence-count") {
		return 0
	}

	ctx, cancel := withTimeout()
	defer cancel()

	count, err := store.RDB.HLen(ctx, userConnectionsKey(userUUID)).Result()
	if err != nil {
		if err != redis.Nil {
			p.log.Warn("count redis user connections failed",
				zap.String("user_uuid", userUUID),
				zap.Error(err),
			)
		}
		return 0
	}

	return int(count)
}

func (p *RedisPresence) ListUserConnections(userUUID string) ([]ConnectionState, error) {
	userUUID = strings.TrimSpace(userUUID)
	if !p.enabled() || userUUID == "" || store.RDB == nil {
		return []ConnectionState{}, nil
	}

	ctx, cancel := withTimeout()
	defer cancel()

	values, err := store.RDB.HGetAll(ctx, userConnectionsKey(userUUID)).Result()
	if err != nil {
		if err == redis.Nil {
			return []ConnectionState{}, nil
		}
		return nil, fmt.Errorf("get redis presence connections: %w", err)
	}

	states := make([]ConnectionState, 0, len(values))
	for _, raw := range values {
		var state ConnectionState
		if err := json.Unmarshal([]byte(raw), &state); err != nil {
			continue
		}
		states = append(states, state)
	}

	return states, nil
}

func (p *RedisPresence) NodeID() string {
	if p == nil {
		return ""
	}

	return p.nodeID
}

func (p *RedisPresence) writeState(state ConnectionState) error {
	ctx, cancel := withTimeout()
	defer cancel()

	payload, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal redis presence state: %w", err)
	}

	expiresAt := time.Now().UTC().Add(p.ttl())
	pipe := store.RDB.TxPipeline()
	pipe.HSet(ctx, userConnectionsKey(state.UserUUID), state.ConnectionID, payload)
	pipe.Expire(ctx, userConnectionsKey(state.UserUUID), p.ttl())
	pipe.ZAdd(ctx, onlineUsersKey(), redis.Z{
		Score:  float64(expiresAt.Unix()),
		Member: state.UserUUID,
	})
	pipe.ZAdd(ctx, onlineConnectionsKey(), redis.Z{
		Score:  float64(expiresAt.Unix()),
		Member: state.ConnectionID,
	})
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("write redis presence pipeline: %w", err)
	}

	return nil
}

func (p *RedisPresence) refreshUserExpiry(userUUID string) error {
	ctx, cancel := withTimeout()
	defer cancel()

	return store.RDB.ZAdd(ctx, onlineUsersKey(), redis.Z{
		Score:  float64(time.Now().UTC().Add(p.ttl()).Unix()),
		Member: userUUID,
	}).Err()
}

func (p *RedisPresence) cleanupExpired(ctx context.Context) {
	if ctx == nil || !p.enabled() || store.RDB == nil {
		return
	}

	now := time.Now().UTC().Unix()
	pipe := store.RDB.TxPipeline()
	pipe.ZRemRangeByScore(ctx, onlineUsersKey(), "-inf", fmt.Sprintf("%d", now))
	pipe.ZRemRangeByScore(ctx, onlineConnectionsKey(), "-inf", fmt.Sprintf("%d", now))
	if _, err := pipe.Exec(ctx); err != nil {
		p.log.Warn("cleanup redis presence expired members failed", zap.Error(err))
	}
}

func (p *RedisPresence) ttl() time.Duration {
	if p.config.TTLSeconds <= 0 {
		return 120 * time.Second
	}

	return time.Duration(p.config.TTLSeconds) * time.Second
}

func (p *RedisPresence) enabled() bool {
	return p != nil && p.config.Enabled
}

func (p *RedisPresence) shouldRun(userUUID, connectionID string) bool {
	return p.enabled() && store.RDB != nil && strings.TrimSpace(userUUID) != "" && strings.TrimSpace(connectionID) != ""
}

func withTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), requestTimeout)
}

func resolveNodeID(cfg config.Presence) string {
	if nodeID := strings.TrimSpace(cfg.NodeID); nodeID != "" {
		return nodeID
	}

	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		return "dipole-node"
	}

	return hostname
}

func onlineUsersKey() string {
	return "presence:online_users"
}

func onlineConnectionsKey() string {
	return "presence:online_connections"
}

func userConnectionsKey(userUUID string) string {
	return "presence:user:" + strings.TrimSpace(userUUID) + ":connections"
}
