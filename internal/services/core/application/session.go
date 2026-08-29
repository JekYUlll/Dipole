package coreapplication

import (
	"time"

	platformPresence "github.com/JekYUlll/Dipole/internal/platform/presence"
	coresession "github.com/JekYUlll/Dipole/internal/services/core/domain/session"
)

type SessionPresence interface {
	ListUserConnections(userUUID string) ([]platformPresence.ConnectionState, error)
}

type SessionTokenRevoker interface {
	Revoke(token string) error
	RevokeTokenID(tokenID string, expiresAt time.Time) error
}

type SessionKicker interface {
	KickConnections(userUUID string, connectionIDs []string) error
	KickAllConnections(userUUID string) error
}

type SessionDependencies struct {
	Presence SessionPresence
	Tokens   SessionTokenRevoker
	Kicker   SessionKicker
}

// LocalSessionApplication keeps device-session use cases behind the Core boundary.
type LocalSessionApplication struct {
	*coresession.SessionService
}

func NewSessionApplication(dependencies SessionDependencies) *LocalSessionApplication {
	return &LocalSessionApplication{
		SessionService: coresession.NewSessionService(dependencies.Presence, dependencies.Tokens, dependencies.Kicker),
	}
}
