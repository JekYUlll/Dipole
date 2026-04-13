package service

import (
	"errors"
	"testing"
	"time"

	platformPresence "github.com/JekYUlll/Dipole/internal/platform/presence"
)

type stubSessionPresence struct {
	states []platformPresence.ConnectionState
	err    error
}

func (s *stubSessionPresence) ListUserConnections(userUUID string) ([]platformPresence.ConnectionState, error) {
	if s.err != nil {
		return nil, s.err
	}

	result := make([]platformPresence.ConnectionState, 0, len(s.states))
	for _, state := range s.states {
		if state.UserUUID == userUUID {
			result = append(result, state)
		}
	}
	return result, nil
}

type stubSessionTokens struct {
	revokedTokens   []string
	revokedTokenIDs []string
}

func (s *stubSessionTokens) Revoke(token string) error {
	s.revokedTokens = append(s.revokedTokens, token)
	return nil
}

func (s *stubSessionTokens) RevokeTokenID(tokenID string, expiresAt time.Time) error {
	s.revokedTokenIDs = append(s.revokedTokenIDs, tokenID)
	return nil
}

type stubSessionKicker struct {
	kickedUserUUID    string
	kickedIDs         []string
	kickedAllUserUUID string
}

func (s *stubSessionKicker) KickConnections(userUUID string, connectionIDs []string) error {
	s.kickedUserUUID = userUUID
	s.kickedIDs = append([]string(nil), connectionIDs...)
	return nil
}

func (s *stubSessionKicker) KickAllConnections(userUUID string) error {
	s.kickedAllUserUUID = userUUID
	return nil
}

func TestSessionServiceListUserDevices(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	service := NewSessionService(&stubSessionPresence{
		states: []platformPresence.ConnectionState{
			{ConnectionID: "C100", UserUUID: "U100", Device: "desktop", NodeID: "node-a", ConnectedAt: now.Add(-time.Minute), LastSeenAt: now},
			{ConnectionID: "C200", UserUUID: "U100", Device: "mobile", NodeID: "node-b", ConnectedAt: now, LastSeenAt: now},
		},
	}, nil, nil)

	devices, err := service.ListUserDevices("U100")
	if err != nil {
		t.Fatalf("list user devices: %v", err)
	}
	if len(devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(devices))
	}
	if devices[0].ConnectionID != "C200" {
		t.Fatalf("expected newest connection first, got %s", devices[0].ConnectionID)
	}
}

func TestSessionServiceForceLogoutConnection(t *testing.T) {
	t.Parallel()

	tokens := &stubSessionTokens{}
	kicker := &stubSessionKicker{}
	service := NewSessionService(&stubSessionPresence{
		states: []platformPresence.ConnectionState{
			{
				ConnectionID:   "C100",
				UserUUID:       "U100",
				TokenID:        "T100",
				TokenExpiresAt: time.Now().UTC().Add(time.Hour),
			},
		},
	}, tokens, kicker)

	if err := service.ForceLogoutConnection("U100", "C100"); err != nil {
		t.Fatalf("force logout connection: %v", err)
	}
	if len(tokens.revokedTokenIDs) != 1 || tokens.revokedTokenIDs[0] != "T100" {
		t.Fatalf("expected token id T100 revoked, got %+v", tokens.revokedTokenIDs)
	}
	if kicker.kickedUserUUID != "U100" || len(kicker.kickedIDs) != 1 || kicker.kickedIDs[0] != "C100" {
		t.Fatalf("unexpected kick call: user=%s ids=%v", kicker.kickedUserUUID, kicker.kickedIDs)
	}
}

func TestSessionServiceForceLogoutConnectionNotFound(t *testing.T) {
	t.Parallel()

	service := NewSessionService(&stubSessionPresence{}, &stubSessionTokens{}, &stubSessionKicker{})

	err := service.ForceLogoutConnection("U100", "C404")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestSessionServiceForceLogoutAll(t *testing.T) {
	t.Parallel()

	tokens := &stubSessionTokens{}
	kicker := &stubSessionKicker{}
	service := NewSessionService(&stubSessionPresence{
		states: []platformPresence.ConnectionState{
			{
				ConnectionID:   "C100",
				UserUUID:       "U100",
				TokenID:        "T100",
				TokenExpiresAt: time.Now().UTC().Add(time.Hour),
			},
			{
				ConnectionID:   "C200",
				UserUUID:       "U100",
				TokenID:        "T200",
				TokenExpiresAt: time.Now().UTC().Add(time.Hour),
			},
		},
	}, tokens, kicker)

	if err := service.ForceLogoutAll("U100", "CURRENT"); err != nil {
		t.Fatalf("force logout all: %v", err)
	}
	if len(tokens.revokedTokenIDs) != 2 {
		t.Fatalf("expected 2 connection token ids revoked, got %d", len(tokens.revokedTokenIDs))
	}
	if len(tokens.revokedTokens) != 1 || tokens.revokedTokens[0] != "CURRENT" {
		t.Fatalf("expected current token revoked, got %v", tokens.revokedTokens)
	}
	if kicker.kickedAllUserUUID != "U100" {
		t.Fatalf("expected all connections kicked for U100, got %s", kicker.kickedAllUserUUID)
	}
}
