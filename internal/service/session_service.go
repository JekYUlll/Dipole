package service

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	platformPresence "github.com/JekYUlll/Dipole/internal/platform/presence"
)

var (
	ErrSessionConnectionRequired = errors.New("session connection is required")
	ErrSessionNotFound           = errors.New("session not found")
)

type sessionPresenceReader interface {
	ListUserConnections(userUUID string) ([]platformPresence.ConnectionState, error)
}

type sessionTokenRevoker interface {
	Revoke(token string) error
	RevokeTokenID(tokenID string, expiresAt time.Time) error
}

type sessionKicker interface {
	KickConnections(userUUID string, connectionIDs []string) error
	KickAllConnections(userUUID string) error
}

type DeviceSessionView struct {
	ConnectionID string    `json:"connection_id"`
	Device       string    `json:"device"`
	DeviceID     string    `json:"device_id,omitempty"`
	UserAgent    string    `json:"user_agent,omitempty"`
	RemoteAddr   string    `json:"remote_addr,omitempty"`
	NodeID       string    `json:"node_id"`
	ConnectedAt  time.Time `json:"connected_at"`
	LastSeenAt   time.Time `json:"last_seen_at"`
}

type SessionKickEventPayload struct {
	UserUUID      string    `json:"user_uuid"`
	ConnectionIDs []string  `json:"connection_ids,omitempty"`
	All           bool      `json:"all"`
	Reason        string    `json:"reason"`
	OccurredAt    time.Time `json:"occurred_at"`
}

type SessionService struct {
	presence sessionPresenceReader
	tokens   sessionTokenRevoker
	kicker   sessionKicker
}

func NewSessionService(presence sessionPresenceReader, tokens sessionTokenRevoker, kicker sessionKicker) *SessionService {
	return &SessionService{
		presence: presence,
		tokens:   tokens,
		kicker:   kicker,
	}
}

func (s *SessionService) ListUserDevices(userUUID string) ([]*DeviceSessionView, error) {
	userUUID = strings.TrimSpace(userUUID)
	if userUUID == "" {
		return []*DeviceSessionView{}, nil
	}
	if s.presence == nil {
		return []*DeviceSessionView{}, nil
	}

	states, err := s.presence.ListUserConnections(userUUID)
	if err != nil {
		return nil, fmt.Errorf("list user device sessions: %w", err)
	}

	sort.Slice(states, func(i, j int) bool {
		return states[i].ConnectedAt.After(states[j].ConnectedAt)
	})

	devices := make([]*DeviceSessionView, 0, len(states))
	for _, state := range states {
		devices = append(devices, &DeviceSessionView{
			ConnectionID: state.ConnectionID,
			Device:       state.Device,
			DeviceID:     state.DeviceID,
			UserAgent:    state.UserAgent,
			RemoteAddr:   state.RemoteAddr,
			NodeID:       state.NodeID,
			ConnectedAt:  state.ConnectedAt,
			LastSeenAt:   state.LastSeenAt,
		})
	}

	return devices, nil
}

func (s *SessionService) ForceLogoutConnection(userUUID, connectionID string) error {
	userUUID = strings.TrimSpace(userUUID)
	connectionID = strings.TrimSpace(connectionID)
	if connectionID == "" {
		return ErrSessionConnectionRequired
	}
	if s.presence == nil {
		return ErrSessionNotFound
	}

	states, err := s.presence.ListUserConnections(userUUID)
	if err != nil {
		return fmt.Errorf("list user connections in force logout connection: %w", err)
	}

	var target *platformPresence.ConnectionState
	for i := range states {
		if states[i].ConnectionID == connectionID {
			target = &states[i]
			break
		}
	}
	if target == nil {
		return ErrSessionNotFound
	}

	if s.tokens != nil && target.TokenID != "" && !target.TokenExpiresAt.IsZero() {
		if err := s.tokens.RevokeTokenID(target.TokenID, target.TokenExpiresAt); err != nil {
			return fmt.Errorf("revoke token for forced connection logout: %w", err)
		}
	}
	if s.kicker != nil {
		if err := s.kicker.KickConnections(userUUID, []string{connectionID}); err != nil {
			return fmt.Errorf("kick forced logout connection: %w", err)
		}
	}

	return nil
}

func (s *SessionService) ForceLogoutAll(userUUID, currentToken string) error {
	userUUID = strings.TrimSpace(userUUID)
	currentToken = strings.TrimSpace(currentToken)

	if s.presence != nil {
		states, err := s.presence.ListUserConnections(userUUID)
		if err != nil {
			return fmt.Errorf("list user connections in force logout all: %w", err)
		}
		for _, state := range states {
			if s.tokens != nil && state.TokenID != "" && !state.TokenExpiresAt.IsZero() {
				if err := s.tokens.RevokeTokenID(state.TokenID, state.TokenExpiresAt); err != nil {
					return fmt.Errorf("revoke token in force logout all: %w", err)
				}
			}
		}
	}

	if s.tokens != nil && currentToken != "" {
		if err := s.tokens.Revoke(currentToken); err != nil {
			return fmt.Errorf("revoke current token in force logout all: %w", err)
		}
	}
	if s.kicker != nil {
		if err := s.kicker.KickAllConnections(userUUID); err != nil {
			return fmt.Errorf("kick all connections in force logout all: %w", err)
		}
	}

	return nil
}
