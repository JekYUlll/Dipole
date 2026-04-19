// Package hotgroup detects "hot" groups — groups with high message frequency —
// so the message delivery path can switch from per-message WS push to a
// lightweight notify-and-pull model, reducing fan-out pressure on busy groups.
//
// Detection uses two Redis keys per group:
//   group:hot:counter:<uuid>  — INCR counter with a sliding window TTL (default 60s)
//   group:hot:active:<uuid>   — presence flag set when thresholds are crossed (default 180s cooling)
//
// A group is considered hot when both member count and recent message count
// exceed the configured thresholds, OR while the active flag is still alive
// (cooling period prevents rapid hot/cold flapping).
package hotgroup

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/logger"
	"github.com/JekYUlll/Dipole/internal/store"
)

const requestTimeout = time.Second

type Status struct {
	Enabled              bool
	IsHot                bool
	MemberCount          int
	RecentMessageCount   int
	MemberCountThreshold int
	MessageThreshold     int
	WindowSeconds        int
	CoolingSeconds       int
}

type RedisDetector struct {
	config config.HotGroup
	log    *zap.Logger
}

func NewRedisDetector() *RedisDetector {
	return NewDetector(config.HotGroupConfig())
}

func NewDetector(cfg config.HotGroup) *RedisDetector {
	if !cfg.Enabled {
		return nil
	}

	return &RedisDetector{
		config: cfg,
		log:    logger.Named("hot-group"),
	}
}

// ObserveMessage increments the sliding-window counter for a group and returns
// the current hot status. Called on every group message send so the detector
// can promote a group to hot as soon as thresholds are crossed.
func (d *RedisDetector) ObserveMessage(groupUUID string, memberCount int) (Status, error) {
	groupUUID = strings.TrimSpace(groupUUID)
	status := d.baseStatus(memberCount)
	if !d.shouldRun(groupUUID) {
		return status, nil
	}

	ctx, cancel := withTimeout()
	defer cancel()

	count, err := store.RDB.Incr(ctx, counterKey(groupUUID)).Result()
	if err != nil {
		return status, fmt.Errorf("increase hot group counter: %w", err)
	}
	if count == 1 {
		if err := store.RDB.Expire(ctx, counterKey(groupUUID), d.window()).Err(); err != nil {
			return status, fmt.Errorf("expire hot group counter: %w", err)
		}
	}

	status.RecentMessageCount = int(count)
	if status.MemberCount >= d.config.MemberCountThreshold && status.RecentMessageCount >= d.config.MessageThreshold {
		status.IsHot = true
		if err := store.RDB.Set(ctx, activeKey(groupUUID), 1, d.cooling()).Err(); err != nil {
			return status, fmt.Errorf("set hot group active flag: %w", err)
		}
		return status, nil
	}

	exists, err := store.RDB.Exists(ctx, activeKey(groupUUID)).Result()
	if err != nil {
		return status, fmt.Errorf("check hot group active flag: %w", err)
	}
	status.IsHot = exists > 0
	return status, nil
}

// Status reads the current hot status without modifying the counter.
// Used by the Kafka delivery handler to decide push strategy per message.
func (d *RedisDetector) Status(groupUUID string, memberCount int) (Status, error) {
	groupUUID = strings.TrimSpace(groupUUID)
	status := d.baseStatus(memberCount)
	if !d.shouldRun(groupUUID) {
		return status, nil
	}

	ctx, cancel := withTimeout()
	defer cancel()

	count, err := store.RDB.Get(ctx, counterKey(groupUUID)).Int()
	if err != nil && !errors.Is(err, redis.Nil) {
		return status, fmt.Errorf("get hot group counter: %w", err)
	}
	if err == nil {
		status.RecentMessageCount = count
	}

	if status.MemberCount >= d.config.MemberCountThreshold && status.RecentMessageCount >= d.config.MessageThreshold {
		status.IsHot = true
		return status, nil
	}

	exists, err := store.RDB.Exists(ctx, activeKey(groupUUID)).Result()
	if err != nil {
		return status, fmt.Errorf("check hot group active flag: %w", err)
	}
	status.IsHot = exists > 0 && status.MemberCount >= d.config.MemberCountThreshold
	return status, nil
}

func (d *RedisDetector) shouldRun(groupUUID string) bool {
	return d != nil && d.config.Enabled && groupUUID != "" && store.RDB != nil
}

func (d *RedisDetector) baseStatus(memberCount int) Status {
	if d == nil {
		return Status{
			Enabled:     false,
			MemberCount: memberCount,
		}
	}

	return Status{
		Enabled:              d != nil && d.config.Enabled,
		MemberCount:          memberCount,
		MemberCountThreshold: d.config.MemberCountThreshold,
		MessageThreshold:     d.config.MessageThreshold,
		WindowSeconds:        d.config.WindowSeconds,
		CoolingSeconds:       d.config.CoolingSeconds,
	}
}

func (d *RedisDetector) window() time.Duration {
	seconds := d.config.WindowSeconds
	if seconds <= 0 {
		seconds = 60
	}
	return time.Duration(seconds) * time.Second
}

func (d *RedisDetector) cooling() time.Duration {
	seconds := d.config.CoolingSeconds
	if seconds <= 0 {
		seconds = 180
	}
	return time.Duration(seconds) * time.Second
}

func withTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), requestTimeout)
}

func counterKey(groupUUID string) string {
	return "group:hot:counter:" + strings.TrimSpace(groupUUID)
}

func activeKey(groupUUID string) string {
	return "group:hot:active:" + strings.TrimSpace(groupUUID)
}
