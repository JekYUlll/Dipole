package ws

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type ConnectionIdentity struct {
	Device     string
	DeviceID   string
	UserAgent  string
	RemoteAddr string
}

type ConnectionSnapshot struct {
	ConnectionID   string    `json:"connection_id"`
	UserUUID       string    `json:"user_uuid"`
	TokenID        string    `json:"-"`
	TokenExpiresAt time.Time `json:"-"`
	Device         string    `json:"device"`
	DeviceID       string    `json:"device_id,omitempty"`
	UserAgent      string    `json:"user_agent,omitempty"`
	RemoteAddr     string    `json:"remote_addr,omitempty"`
	ConnectedAt    time.Time `json:"connected_at"`
	LastSeenAt     time.Time `json:"last_seen_at"`
}

type PresenceTracker interface {
	Register(snapshot ConnectionSnapshot)
	Touch(snapshot ConnectionSnapshot)
	Unregister(userUUID, connectionID string)
	OnlineUserCount() int
	TotalConnectionCount() int
	UserConnectionCount(userUUID string) int
}

func newConnectionID() string {
	buf := make([]byte, 10)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Errorf("generate websocket connection id: %w", err))
	}

	return "C" + strings.ToUpper(hex.EncodeToString(buf))
}
