package ws

import platformPresence "github.com/JekYUlll/Dipole/internal/platform/presence"

type RedisPresenceTracker struct {
	tracker *platformPresence.RedisPresence
}

func NewRedisPresenceTracker(tracker *platformPresence.RedisPresence) *RedisPresenceTracker {
	if tracker == nil {
		return nil
	}
	return &RedisPresenceTracker{tracker: tracker}
}

func (a *RedisPresenceTracker) Register(snapshot ConnectionSnapshot) {
	if a != nil && a.tracker != nil {
		a.tracker.Register(toPresenceState(snapshot))
	}
}

func (a *RedisPresenceTracker) Touch(snapshot ConnectionSnapshot) {
	if a != nil && a.tracker != nil {
		a.tracker.Touch(toPresenceState(snapshot))
	}
}

func (a *RedisPresenceTracker) Unregister(userUUID, connectionID string) {
	if a != nil && a.tracker != nil {
		a.tracker.Unregister(userUUID, connectionID)
	}
}

func (a *RedisPresenceTracker) OnlineUserCount() int {
	if a == nil || a.tracker == nil {
		return 0
	}
	return a.tracker.OnlineUserCount()
}

func (a *RedisPresenceTracker) TotalConnectionCount() int {
	if a == nil || a.tracker == nil {
		return 0
	}
	return a.tracker.TotalConnectionCount()
}

func (a *RedisPresenceTracker) UserConnectionCount(userUUID string) int {
	if a == nil || a.tracker == nil {
		return 0
	}
	return a.tracker.UserConnectionCount(userUUID)
}

func toPresenceState(snapshot ConnectionSnapshot) platformPresence.ConnectionState {
	return platformPresence.ConnectionState{
		ConnectionID:   snapshot.ConnectionID,
		UserUUID:       snapshot.UserUUID,
		TokenID:        snapshot.TokenID,
		TokenExpiresAt: snapshot.TokenExpiresAt,
		Device:         snapshot.Device,
		DeviceID:       snapshot.DeviceID,
		UserAgent:      snapshot.UserAgent,
		RemoteAddr:     snapshot.RemoteAddr,
		ConnectedAt:    snapshot.ConnectedAt,
		LastSeenAt:     snapshot.LastSeenAt,
	}
}
