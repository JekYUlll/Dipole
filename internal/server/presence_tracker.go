package server

import (
	platformPresence "github.com/JekYUlll/Dipole/internal/platform/presence"
	wsTransport "github.com/JekYUlll/Dipole/internal/transport/ws"
)

type wsPresenceTrackerAdapter struct {
	tracker *platformPresence.RedisPresence
}

func newWSPresenceTrackerAdapter(tracker *platformPresence.RedisPresence) *wsPresenceTrackerAdapter {
	if tracker == nil {
		return nil
	}

	return &wsPresenceTrackerAdapter{tracker: tracker}
}

func (a *wsPresenceTrackerAdapter) Register(snapshot wsTransport.ConnectionSnapshot) {
	if a == nil || a.tracker == nil {
		return
	}

	a.tracker.Register(toPresenceState(snapshot))
}

func (a *wsPresenceTrackerAdapter) Touch(snapshot wsTransport.ConnectionSnapshot) {
	if a == nil || a.tracker == nil {
		return
	}

	a.tracker.Touch(toPresenceState(snapshot))
}

func (a *wsPresenceTrackerAdapter) Unregister(userUUID, connectionID string) {
	if a == nil || a.tracker == nil {
		return
	}

	a.tracker.Unregister(userUUID, connectionID)
}

func (a *wsPresenceTrackerAdapter) OnlineUserCount() int {
	if a == nil || a.tracker == nil {
		return 0
	}

	return a.tracker.OnlineUserCount()
}

func (a *wsPresenceTrackerAdapter) TotalConnectionCount() int {
	if a == nil || a.tracker == nil {
		return 0
	}

	return a.tracker.TotalConnectionCount()
}

func (a *wsPresenceTrackerAdapter) UserConnectionCount(userUUID string) int {
	if a == nil || a.tracker == nil {
		return 0
	}

	return a.tracker.UserConnectionCount(userUUID)
}

func toPresenceState(snapshot wsTransport.ConnectionSnapshot) platformPresence.ConnectionState {
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
