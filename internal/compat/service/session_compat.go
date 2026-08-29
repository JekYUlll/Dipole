package service

import (
	"time"

	platformPresence "github.com/JekYUlll/Dipole/internal/platform/presence"
	coresession "github.com/JekYUlll/Dipole/internal/services/core/domain/session"
)

var (
	ErrSessionConnectionRequired = coresession.ErrSessionConnectionRequired
	ErrSessionNotFound           = coresession.ErrSessionNotFound
)

type DeviceSessionView = coresession.DeviceSessionView
type SessionKickEventPayload = coresession.SessionKickEventPayload
type SessionService = coresession.SessionService

func NewSessionService(presence interface {
	ListUserConnections(userUUID string) ([]platformPresence.ConnectionState, error)
}, tokens interface {
	Revoke(token string) error
	RevokeTokenID(tokenID string, expiresAt time.Time) error
}, kicker interface {
	KickConnections(userUUID string, connectionIDs []string) error
	KickAllConnections(userUUID string) error
}) *SessionService {
	return coresession.NewSessionService(presence, tokens, kicker)
}
